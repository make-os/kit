package repo

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// SignCommitCmd adds transaction information to a new or recent commit and signs it.
func SignCommitCmd(
	cfg *config.AppConfig,
	message string,
	targetRepo core.BareRepo,
	txFee,
	nextNonce string,
	amendRecent bool,
	mergeID,
	head,
	pushKeyID,
	pushKeyPass,
	targetRemote string,
	resetTokens bool,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, pushKeyID, pushKeyPass, targetRepo)
	if err != nil {
		return err
	}

	// Validate merge ID is set.
	// Must be numeric and 8 bytes long
	if mergeID != "" {
		if !govalidator.IsNumeric(mergeID) {
			return fmt.Errorf("merge id must be numeric")
		} else if len(mergeID) > 8 {
			return fmt.Errorf("merge proposal id exceeded 8 bytes limit")
		}
	}

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfPushKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Get the reference pointed to by HEAD
	if head == "" {
		head, err = targetRepo.Head()
		if err != nil {
			return fmt.Errorf("failed to get HEAD") // :D
		}
	} else {
		if !isBranch(head) {
			head = plumbing.NewBranchReferenceName(head).String()
		}
	}

	// Gather any transaction options
	options := []string{fmt.Sprintf("reference=%s", head)}
	if mergeID != "" {
		options = append(options, fmt.Sprintf("mergeID=%s", mergeID))
	}

	// Make the transaction parameter object
	txDetail, err := types.MakeAndValidateTxDetail(txFee, nextNonce, pushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = setPushTokenToRemotes(targetRepo, targetRemote, txDetail, key, resetTokens); err != nil {
		return err
	}

	// Create a new quiet commit if recent commit amendment is not desired
	if !amendRecent {
		if err := targetRepo.CreateAndOrSignQuietCommit(message, pushKeyID); err != nil {
			return err
		}
		return nil
	}

	// Otherwise, amend the recent commit.
	// Get recent commit hash of the current branch.
	hash, err := targetRepo.GetRecentCommit()
	if err != nil {
		if err == ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	commit, _ := targetRepo.CommitObject(plumbing.NewHash(hash))
	if message == "" {
		message = commit.Message
	}

	// Update the recent commit message
	if err = targetRepo.UpdateRecentCommitMsg(message, pushKeyID); err != nil {
		return err
	}

	return nil
}

// SignTagCmd creates an annotated tag, appends transaction information to its
// message and signs it.
func SignTagCmd(
	cfg *config.AppConfig,
	gitArgs []string,
	msg string,
	targetRepo core.BareRepo,
	txFee,
	nextNonce,
	pushKeyID,
	pushKeyPass,
	targetRemote string,
	resetTokens bool,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	gitFlags := pflag.NewFlagSet("git-tag", pflag.ExitOnError)
	gitFlags.ParseErrorsWhitelist.UnknownFlags = true
	gitFlags.StringP("message", "m", "", "user message")
	gitFlags.StringP("local-user", "u", "", "user signing key")
	gitFlags.Parse(gitArgs)

	// If --local-user (-u) flag is provided in the git args, use the value as the push key ID
	if gitFlags.Lookup("local-user") != nil {
		pushKeyID, _ = gitFlags.GetString("local-user")
	}

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, pushKeyID, pushKeyPass, targetRepo)
	if err != nil {
		return err
	}

	// If --message (-m) flag is provided, use the value as the message
	if gitFlags.Lookup("message") != nil {
		msg, _ = gitFlags.GetString("message")
	}

	// Remove -m, --message, -u, -local-user flag from the git args
	gitArgs = util.RemoveFlag(gitArgs, []string{"m", "message", "u", "local-user"})

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfPushKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Gather any transaction params options
	options := []string{
		fmt.Sprintf("reference=%s", plumbing.NewTagReferenceName(gitFlags.Arg(0)).String()),
	}

	// Construct the txDetail and append to the current message
	txDetail, err := types.MakeAndValidateTxDetail(txFee, nextNonce, pushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = setPushTokenToRemotes(targetRepo, targetRemote, txDetail, key, resetTokens); err != nil {
		return err
	}

	// Create the tag
	if err = targetRepo.CreateTagWithMsg(gitArgs, msg, pushKeyID); err != nil {
		return err
	}

	return nil
}

// SignNoteCmd creates adds transaction information to a note and signs it.
func SignNoteCmd(
	cfg *config.AppConfig,
	targetRepo core.BareRepo,
	txFee,
	nextNonce,
	note,
	pushKeyID,
	pushKeyPass,
	targetRemote string,
	resetTokens bool,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, pushKeyID, pushKeyPass, targetRepo)
	if err != nil {
		return err
	}

	// Expand note name to full reference name if name is short
	if !strings.HasPrefix("refs/notes", note) {
		note = "refs/notes/" + note
	}

	// Get the HEAD hash of the note and add it as a option
	noteRef, err := targetRepo.Reference(plumbing.ReferenceName(note), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}
	options := []string{
		fmt.Sprintf("reference=%s", noteRef.Name()),
		fmt.Sprintf("head=%s", noteRef.Hash().String()),
	}

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfPushKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Construct the txDetail
	txDetail, err := types.MakeAndValidateTxDetail(txFee, nextNonce, pushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = setPushTokenToRemotes(targetRepo, targetRemote, txDetail, key, resetTokens); err != nil {
		return err
	}

	return nil
}

// CreateAndSendMergeRequestCmd creates merge request proposal
// and sends it to the network
// cfg: App config object
// keyAddrOrIdx: Address or index of key
// passphrase: Passphrase of the key
func CreateAndSendMergeRequestCmd(
	cfg *config.AppConfig,
	keyAddrOrIdx,
	passphrase,
	repoName,
	proposalID,
	baseBranch,
	baseHash,
	targetBranch,
	targetHash,
	fee,
	nextNonce string,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signer key
	am := keystore.New(path.Join(cfg.DataDir(), config.KeystoreDirName))
	unlocked, err := am.UIUnlockKey(keyAddrOrIdx, passphrase)
	if err != nil {
		return errors.Wrap(err, "unable to unlock")
	}
	fmt.Println("")

	// Determine the next nonce, if unset from flag
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfAccount(unlocked.GetAddress(), rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Create the merge request transaction
	data := map[string]interface{}{
		"name":         repoName,
		"id":           proposalID,
		"type":         core.TxTypeRepoProposalMergeRequest,
		"fee":          fee,
		"senderPubKey": unlocked.GetKey().PubKey().Base58(),
		"nonce":        nextNonce,
		"base":         baseBranch,
		"baseHash":     baseHash,
		"target":       targetBranch,
		"targetHash":   targetHash,
		"timestamp":    time.Now().Unix(),
	}

	// Attempt to load the transaction from map to its native object.
	// If we successfully create its native object, we are sure that the server will
	// be able to do it without error. Also use the object `Sign` method to create
	// a signature and set it on the map
	o := core.NewBareRepoProposalMergeRequest()
	if err := o.FromMap(data); err != nil {
		return errors.Wrap(err, "invalid transaction data")
	}
	sig, err := o.Sign(unlocked.GetKey().PrivKey().Base58())
	if err != nil {
		return errors.Wrap(err, "failed to sign transaction")
	}
	data["sig"] = util.ToHex(sig)

	txHash, err := api.SendTxPayload(data, rpcClient, remoteClients)
	if err != nil {
		return err
	}

	fmt.Println(color.GreenString("Success! Merge proposal sent."))
	fmt.Println("Hash:", txHash)

	return nil
}

// getAndUnlockPushKey takes a push key ID and unlocks it using the default passphrase
// or one obtained from the git config of the repository or from an environment variable.
func getAndUnlockPushKey(
	cfg *config.AppConfig,
	pushKeyID,
	defaultPassphrase string,
	targetRepo core.BareRepo) (core.StoredKey, error) {

	// Get the push key from the key store
	ks := keystore.New(cfg.KeystoreDir())
	ks.SetOutput(ioutil.Discard)

	// Ensure the push key exist
	key, err := ks.GetByIndexOrAddress(pushKeyID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find push key (%s)", pushKeyID)
	}

	// Get the request token from the config
	repoCfg, err := targetRepo.Config()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get repo config")
	}

	// APPNAME_REPONAME_PASS may contain the passphrase
	_, repoName := filepath.Split(targetRepo.Path())
	passEnvVarName := fmt.Sprintf("%s_%s_PASS", strings.ToUpper(config.AppName), strings.ToUpper(repoName))

	// If push key is protected, require passphrase
	var passphrase = defaultPassphrase
	if !key.IsUnprotected() && passphrase == "" {

		// Get the password from the "user.passphrase" option in git config
		passphrase = repoCfg.Raw.Section("user").Option("passphrase")

		// If we still don't have a passphrase, get it from the env variable:
		// APPNAME_REPONAME_PASS
		if passphrase == "" {
			passphrase = os.Getenv(passEnvVarName)
		}

		// Well, if no passphrase still, so exit with error
		if passphrase == "" {
			return nil, fmt.Errorf("passphrase of signing key is required")
		}
	}

	key, err = ks.UIUnlockKey(pushKeyID, passphrase)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unlock push key (%s)", pushKeyID)
	}

	// Set the passphrase on the env var so signing/verify commands
	// can learn about the passphrase
	os.Setenv(passEnvVarName, passphrase)

	return key, nil
}

// GitSignCmd mocks git signing interface allowing this program to
// be used by git for signing a commit or tag.
func GitSignCmd(cfg *config.AppConfig, data io.Reader) {

	log := cfg.G().Log

	// Get the data to be signed and the key id to use
	pushKeyID := os.Args[3]

	// Get the target repo
	repoDir, _ := os.Getwd()
	targetRepo, err := GetRepo(repoDir)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "failed to get repo").Error())
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, pushKeyID, "", targetRepo)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Get the push request token
	token := os.Getenv(fmt.Sprintf("%s_LAST_PUSH_TOKEN", config.AppName))
	if token == "" {
		log.Fatal(errors.Wrap(err, "push request token not set").Error())
	}

	// Decode the push request token
	txDetail, err := DecodePushToken(token)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to decode token").Error())
	}

	// Construct the message
	// git sig message + msgpack(tx parameters)
	msg, _ := ioutil.ReadAll(data)
	msg = append(msg, txDetail.BytesNoSig()...)

	// Sign the message
	sig, err := key.GetKey().PrivKey().Sign(msg)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to sign").Error())
	}

	// Write output
	w := bytes.NewBuffer(nil)
	pem.Encode(w, &pem.Block{Bytes: sig, Type: "PGP SIGNATURE", Headers: txDetail.ToMapForPEMHeader()})
	fmt.Fprintf(os.Stderr, "[GNUPG:] BEGIN_SIGNING\n")
	fmt.Fprintf(os.Stderr, "[GNUPG:] SIG_CREATED C\n")
	fmt.Fprintf(os.Stdout, "%s", w.Bytes())

	os.Exit(0)
}

// GitVerifyCmd mocks git signing interface allowing this program to
// be used by git for verifying signatures.
func GitVerifyCmd(cfg *config.AppConfig, args []string) {

	fp := fmt.Fprintf

	// Read the signature
	sig, err := ioutil.ReadFile(args[4])
	if err != nil {
		fp(os.Stderr, "failed to read sig file: %s", err)
		os.Exit(1)
	}

	// Attempt to decode from PEM
	decSig, _ := pem.Decode(sig)
	if decSig == nil {
		fp(os.Stderr, "malformed signatured. Expected pem encoded signature.", err)
		os.Exit(1)
	}

	// Get tx parameters from the header
	txDetail, err := types.TxDetailFromPEMHeader(decSig.Headers)
	if err != nil {
		fp(os.Stdout, "[GNUPG:] BADSIG 0\n")
		fp(os.Stderr, "invalid header: %s\n", err)
		os.Exit(1)
	}

	// Ensure push key is set
	if txDetail.PushKeyID == "" {
		fp(os.Stdout, "[GNUPG:] BADSIG 0\n")
		fp(os.Stderr, "invalid header: 'pkID' is required\n")
		os.Exit(1)
	}

	// Get the target repo
	repoDir, _ := os.Getwd()
	targetRepo, err := GetRepo(repoDir)
	if err != nil {
		fp(os.Stderr, errors.Wrapf(err, "failed to get repo").Error()+"\n")
		os.Exit(1)
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, txDetail.PushKeyID, "", targetRepo)
	if err != nil {
		fp(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}

	// Construct the signature message
	msg, _ := ioutil.ReadAll(os.Stdin)
	msg = append(msg, txDetail.BytesNoSig()...)

	// Verify the signature
	if ok, err := key.GetKey().PubKey().Verify(msg, decSig.Bytes); err != nil || !ok {
		fp(os.Stderr, "signature is not valid\n")
		os.Exit(1)
	}

	// Write output
	cg := color.GreenString
	fp(os.Stdout, "[GNUPG:] NEWSIG\n")
	fp(os.Stdout, "[GNUPG:] GOODSIG 0\n")
	fp(os.Stdout, "[GNUPG:] TRUST_FULLY 0 shell\n")
	fp(os.Stderr, "%s\n", cg("sig: signature is ok"))
	fp(os.Stderr, "%s\n", cg("sig: signed by %s (nonce: %d)", txDetail.PushKeyID, txDetail.Nonce))
	fp(os.Stderr, "%s\n", cg("sig: fee: %s", txDetail.Fee))
	if txDetail.MergeProposalID != "" {
		fp(os.Stderr, "%s\n", cg("sig: fulfilling merge proposal: %s", txDetail.MergeProposalID))
	}

	os.Exit(0)
}
