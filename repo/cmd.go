package repo

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// getPushPrivateKey gets the push private key corresponding to the given push key ID from the keystore.
// Using the push key id, it will attempt to unlock the key if found. If the key is unprotected,
// it will attempt to unlock it using the default passphrase.
func getPushPrivateKey(keystoreDir, pushKeyID, pushKeyPass string) (core.StoredKey, error) {

	// Get the private key from the keystore
	ks := keystore.New(keystoreDir)
	key, err := ks.GetByAddress(pushKeyID)
	if err != nil {
		return nil, err
	}

	// Unlock the address
	if key.IsUnprotected() {
		if err = key.Unlock(keystore.DefaultPassphrase); err != nil {
			return nil, errors.Wrap(err, "failed to unlock unprotected push key")
		}
		return key, nil
	}

	if pushKeyPass == "" {
		return nil, fmt.Errorf("push key passphrase is requred")
	}

	if err = key.Unlock(pushKeyPass); err != nil {
		return nil, errors.Wrap(err, "failed to unlock push key")
	}

	return key, nil
}

// SignCommitCmd adds transaction information to a new or recent commit and signs it.
func SignCommitCmd(
	cfg *config.AppConfig,
	targetRepo core.BareRepo,
	txFee,
	nextNonce string,
	amendRecent,
	deleteRefAction bool,
	mergeID string,
	pushKeyID,
	pushKeyPass string,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get the push key private key
	pushKeyPrivKey, err := getPushPrivateKey(cfg.KeystoreDir(), pushKeyID, pushKeyPass)
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
		nextNonce, err = api.DetermineNextNonceOfGPGKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Gather any transaction params directives
	directives := []string{}
	if deleteRefAction {
		directives = append(directives, "deleteRef")
	}
	if mergeID != "" {
		directives = append(directives, fmt.Sprintf("mergeID=%s", mergeID))
	}

	// Make the transaction parameter object
	txParams, err := util.MakeAndValidateTxParams(txFee, nextNonce, pushKeyID, nil, directives...)
	if err != nil {
		return err
	}

	var commit *object.Commit
	var hash, msg string

	// Create a new quiet commit if recent commit amendment is not desired
	if !amendRecent {
		if err := targetRepo.CreateAndOrSignQuietCommit(txParams.String(), pushKeyID); err != nil {
			return err
		}
		goto add_req_token
	}

	// Otherwise, amend the recent commit.
	// Get recent commit hash of the current branch.
	hash, err = targetRepo.GetRecentCommit()
	if err != nil {
		if err == ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	// Remove any existing txparams and append the new one
	commit, _ = targetRepo.CommitObject(plumbing.NewHash(hash))
	msg = util.RemoveTxParams(commit.Message)
	msg += "\n\n" + txParams.String()

	// Update the recent commit message
	if err = targetRepo.UpdateRecentCommitMsg(msg, pushKeyID); err != nil {
		return err
	}

add_req_token:

	// Create & set request token to remote URLs
	if err = createAndSetRequestTokenToRemoteURLs(pushKeyPrivKey, targetRepo, txParams); err != nil {
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
	nextNonce string,
	deleteRefAction bool,
	pushKeyID,
	pushKeyPass string,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// If -u or --local-user flag is provided in the git args, use the value as the push key ID
	parsedGitArgs := util.ParseSimpleArgs(gitArgs)
	if parsedGitArgs["u"] != "" {
		pushKeyID = parsedGitArgs["u"]
	} else if parsedGitArgs["local-user"] != "" {
		pushKeyID = parsedGitArgs["local-user"]
	}

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get the push key private key
	pushKeyPrivKey, err := getPushPrivateKey(cfg.KeystoreDir(), pushKeyID, pushKeyPass)
	if err != nil {
		return err
	}

	// Get message from the git arguments if message
	// was not provided via our own --message/-m flag
	if msg == "" {
		if m, ok := parsedGitArgs["m"]; ok {
			msg = m
		} else if message, ok := parsedGitArgs["message"]; ok {
			msg = message
		}
	}

	// Remove -m, --message, -u, -local-user flag from the git args since
	// we wont be pass the message via flags but stdin
	gitArgs = util.RemoveFlagVal(gitArgs, []string{"m", "message", "u", "local-user"})

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfGPGKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Gather any transaction params directives
	directives := []string{}
	if deleteRefAction {
		directives = append(directives, "deleteRef")
	}

	// Construct the txparams and append to the current message
	txParams, err := util.MakeAndValidateTxParams(txFee, nextNonce, pushKeyID, nil, directives...)
	if err != nil {
		return err
	}

	// Create the tag
	msg += "\n\n" + txParams.String()
	if err = targetRepo.CreateTagWithMsg(gitArgs, msg, pushKeyID); err != nil {
		return err
	}

	// Create & set request token to remote URLs
	if err = createAndSetRequestTokenToRemoteURLs(pushKeyPrivKey, targetRepo, txParams); err != nil {
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
	note string,
	deleteRefAction bool,
	pushKeyID,
	pushKeyPass string,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get the push key private key
	pushKeyPrivKey, err := getPushPrivateKey(cfg.KeystoreDir(), pushKeyID, pushKeyPass)
	if err != nil {
		return err
	}

	// Expand note name to full reference name if name is short
	if !strings.HasPrefix("refs/notes", note) {
		note = "refs/notes/" + note
	}

	// Get a list of all notes entries in the note
	noteEntries, err := targetRepo.ListTreeObjects(note, false)
	if err != nil {
		msg := fmt.Sprintf("unable to fetch note entries for tree object (%s)", note)
		return errors.Wrap(err, msg)
	}

	// From the entries, find existing tx blob and stop after the first one
	var lastTxBlob *object.Blob
	for hash := range noteEntries {
		obj, err := targetRepo.BlobObject(plumbing.NewHash(hash))
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to read object (%s)", hash))
		}
		r, err := obj.Reader()
		if err != nil {
			return err
		}
		prefix := make([]byte, 3)
		_, _ = r.Read(prefix)
		if string(prefix) == util.TxParamsPrefix {
			lastTxBlob = obj
			break
		}
	}

	// Remove the last tx blob from the note, if present
	if lastTxBlob != nil {
		err = targetRepo.RemoveEntryFromNote(note, noteEntries[lastTxBlob.Hash.String()])
		if err != nil {
			return errors.Wrap(err, "failed to delete existing transaction blob")
		}
	}

	// Get the commit hash the note is currently referencing.
	// We need to add this hash to the signature.
	noteRef, err := targetRepo.Reference(plumbing.ReferenceName(note), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}
	noteHash := noteRef.Hash().String()

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfGPGKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Sign a message composed of the tx information
	sigMsg := MakeNoteSigMsg(txFee, nextNonce, pushKeyID, noteHash, deleteRefAction)
	sig, err := pushKeyPrivKey.GetKey().PrivKey().Sign(sigMsg)
	if err != nil {
		return errors.Wrap(err, "failed to sign transaction parameters")
	}

	// Gather any transaction params directives
	directives := []string{}
	if deleteRefAction {
		directives = append(directives, "deleteRef")
	}

	// Construct the txparams
	txParams, err := util.MakeAndValidateTxParams(txFee, nextNonce, pushKeyID, sig, directives...)
	if err != nil {
		return err
	}

	// Create a blob with 0 byte content which be the subject of our note.
	blobHash, err := targetRepo.CreateBlob("")
	if err != nil {
		return err
	}

	// Next we add the tx blob to the note
	if err = targetRepo.AddEntryToNote(note, blobHash, txParams.String()); err != nil {
		return errors.Wrap(err, "failed to add tx blob")
	}

	// Create & set request token to remote URLs
	if err = createAndSetRequestTokenToRemoteURLs(pushKeyPrivKey, targetRepo, txParams); err != nil {
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

// GitSignCmd mocks git signing interface allowing this program to
// be used by git for signing commits, tags etc
func GitSignCmd(args []string, data io.Reader) {

	// Get the data to be signed and the key id to use
	// dataBz, _ := ioutil.ReadAll(data)
	// keyID := os.Args[3]

	w := bytes.NewBuffer(nil)
	pem.Encode(w, &pem.Block{Bytes: []byte("some sig"), Type: "PGP SIGNATURE", Headers: map[string]string{
		"time":   fmt.Sprintf("%d", time.Now().Unix()),
		"signer": "maker1abc",
	}})

	fmt.Fprintf(os.Stderr, "[GNUPG:] BEGIN_SIGNING\n")
	fmt.Fprintf(os.Stderr, "[GNUPG:] SIG_CREATED C 1 8 00 1584390372 690B4F273B5A8C04AFD41E1DE14EE57A45993CDF\n")
	fmt.Fprintf(os.Stdout, "%s\n", w.Bytes())

	os.Exit(0)
}

// GitVerifyCmd mocks git signing interface allowing this program to
// be used by git for verifying signatures.
func GitVerifyCmd(args []string) {

	sig, err := ioutil.ReadFile(args[4])
	if err != nil {
		log.Fatalf("failed to read sig file: %s", err)
	}

	p, _ := pem.Decode(sig)
	if p == nil {
		os.Stderr.Write([]byte("malformed signature"))
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "[GNUPG:] NEWSIG\n")

	var signedAt int64
	if ts, ok := p.Headers["time"]; ok {
		signedAt, err = strconv.ParseInt(ts, 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stdout, "[GNUPG:] BADSIG 0\n")
			fmt.Fprintf(os.Stderr, "headers.time is invalid")
			os.Exit(1)
		}
	}

	signer, ok := p.Headers["signer"]
	if !ok {
		fmt.Fprintf(os.Stdout, "[GNUPG:] BADSIG 0\n")
		fmt.Fprintf(os.Stderr, "headers.signer is required")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "[GNUPG:] GOODSIG 0\n")
	fmt.Fprintf(os.Stdout, "[GNUPG:] TRUST_FULLY 0 shell\n")
	fmt.Fprintf(os.Stderr, "%s\n", color.GreenString("sig: signature is ok"))
	fmt.Fprintf(os.Stderr, "%s\n", color.GreenString("sig: signed on %s",
		time.Unix(signedAt, 0).String()))
	fmt.Fprintf(os.Stderr, "%s\n", color.GreenString("sig: signed by %s", signer))

	os.Exit(0)
}
