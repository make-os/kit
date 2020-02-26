package cmd

import (
	"crypto/rsa"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/k0kubun/pp"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/account"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	repo2 "gitlab.com/makeos/mosdef/repo"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// DetermineNextNonceOfAccount is used to determine the next nonce of the account
//
// It will use the rpc and remote clients as source to request for
// the current account nonce.

// SignCommitCmd adds transaction information to the recent commit and signs it.
// If rpcClient is set, the transaction nonce of the signing account is fetched
// from the rpc server.
func SignCommitCmd(
	repo core.BareRepo,
	txFee,
	nextNonce,
	signingKey string,
	amendRecent,
	deleteRefAction bool,
	mergeID string,
	rpcClient *client.RPCClient,
	remoteClients []*restclient.RESTClient) error {

	if !govalidator.IsNumeric(mergeID) {
		return fmt.Errorf("merge id must be numeric")
	} else if len(mergeID) > 8 {
		return fmt.Errorf("merge id limit of 8 bytes exceeded")
	}

	// Get the signing key id from the git config if not passed as an argument
	if signingKey == "" {
		signingKey = repo.GetConfig("user.signingKey")
	}
	if signingKey == "" {
		return errors.New("signing key was not set or provided")
	}

	// Get the public key of the signing key
	pkEntity, err := crypto.GetGPGPublicKey(signingKey, repo.GetConfig("gpg.program"), "")
	if err != nil {
		return errors.Wrap(err, "failed to get gpg public key")
	}

	// Get the public key network ID
	gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	// Get the next nonce, if not set
	if nextNonce == "" {
		nextNonce, err = api.DetermineNextNonceOfGPGKeyOwner(gpgID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	directives := []string{}
	if deleteRefAction {
		directives = append(directives, "deleteRef")
	}
	if mergeID != "" {
		directives = append(directives, fmt.Sprintf("mergeID=%s", mergeID))
	}

	txParams, err := util.MakeAndValidateTxParams(txFee, nextNonce, gpgID, nil, directives...)
	if err != nil {
		return err
	}

	// Create a new commit if recent commit amendment is not required
	if !amendRecent {
		if err := repo.MakeSignableCommit(string(txParams), signingKey); err != nil {
			return err
		}
		return nil
	}

	// Otherwise, amend the recent commit.
	// Get recent commit hash of the current branch.
	hash, err := repo.GetRecentCommit()
	if err != nil {
		if err == repo2.ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	// Remove any existing txparams and append the new one
	commit, _ := repo.CommitObject(plumbing.NewHash(hash))
	msg := util.RemoveTxParams(commit.Message)
	msg += "\n\n" + txParams

	// Update the recent commit message
	if err = repo.UpdateRecentCommitMsg(msg, signingKey); err != nil {
		return err
	}

	return nil
}

// SignTagCmd creates an annotated tag, appends transaction information to its
// message and signs it.
// If rpcClient is set, the transaction nonce of the signing account is fetched
// from the rpc server.
func SignTagCmd(
	args []string,
	repo core.BareRepo,
	txFee,
	nextNonce,
	signingKey string,
	deleteRefAction bool,
	rpcClient *client.RPCClient,
	remoteClients []*restclient.RESTClient) error {

	parsed := util.ParseSimpleArgs(args)

	// If -u flag is provided in the git args, use it a signing key
	if parsed["u"] != "" {
		signingKey = parsed["u"]
	}
	// Get the signing key id from the git config if not passed via app -u flag
	if signingKey == "" {
		signingKey = repo.GetConfig("user.signingKey")
	}
	// Return error if we still don't have a signing key
	if signingKey == "" {
		return errors.New("signing key was not set or provided")
	}

	// Get the user-supplied message from the arguments provided
	msg := ""
	if m, ok := parsed["m"]; ok {
		msg = m
	} else if message, ok := parsed["message"]; ok {
		msg = message
	}

	// Remove -m or --message flag from args
	args = util.RemoveFlagVal(args, []string{"m", "message", "u"})

	// Get the public key of the signing key
	pkEntity, err := crypto.GetGPGPublicKey(signingKey, repo.GetConfig("gpg.program"), "")
	if err != nil {
		return errors.Wrap(err, "failed to get gpg public key")
	}

	// Get the public key network ID
	gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	// Get the next nonce, if not set
	if nextNonce == "" {
		nextNonce, err = api.DetermineNextNonceOfGPGKeyOwner(gpgID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	directives := []string{}
	if deleteRefAction {
		directives = append(directives, "deleteRef")
	}

	// Construct the txparams and append to the current message
	txParams, err := util.MakeAndValidateTxParams(txFee, nextNonce, gpgID, nil, directives...)
	if err != nil {
		return err
	}

	// Create the tag
	msg += "\n\n" + txParams
	if err = repo.CreateTagWithMsg(args, msg, signingKey); err != nil {
		return err
	}

	return nil
}

// SignNoteCmd creates adds transaction information to a note and signs it.
// If rpcClient is set, the transaction nonce of the signing account is fetched
// from the rpc server.
func SignNoteCmd(
	repo core.BareRepo,
	txFee,
	nextNonce,
	signingKey,
	note string,
	deleteRefAction bool,
	rpcClient *client.RPCClient,
	remoteClients []*restclient.RESTClient) error {

	// Get the signing key id from the git config if not provided via -s flag
	if signingKey == "" {
		signingKey = repo.GetConfig("user.signingKey")
	}
	// Return error if we still don't have a signing key
	if signingKey == "" {
		return errors.New("signing key was not set or provided")
	}

	// Enforce the inclusion of `refs/notes` to the note argument
	if !strings.HasPrefix("refs/notes", note) {
		note = "refs/notes/" + note
	}

	// Find a list of all notes entries in the note
	noteEntries, err := repo.ListTreeObjects(note, false)
	if err != nil {
		msg := fmt.Sprintf("unable to fetch note entries for tree object (%s)", note)
		return errors.Wrap(err, msg)
	}

	// From the entries, find existing tx blob and stop after the first one
	var lastTxBlob *object.Blob
	for hash := range noteEntries {
		obj, err := repo.BlobObject(plumbing.NewHash(hash))
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to read object (%s)", hash))
		}
		r, err := obj.Reader()
		if err != nil {
			return err
		}
		prefix := make([]byte, 3)
		r.Read(prefix)
		if string(prefix) == util.TxParamsPrefix {
			lastTxBlob = obj
			break
		}
	}

	// Remove the last tx blob from the note, if present
	if lastTxBlob != nil {
		err = repo.RemoveEntryFromNote(note, noteEntries[lastTxBlob.Hash.String()])
		if err != nil {
			return errors.Wrap(err, "failed to delete existing transaction blob")
		}
	}

	// Get the commit hash the note is currently referencing.
	// We need to add this hash to the signature.
	noteRef, err := repo.Reference(plumbing.ReferenceName(note), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}
	noteHash := noteRef.Hash().String()

	// Get the public key of the signing key
	pkEntity, err := crypto.GetGPGPrivateKey(signingKey, repo.GetConfig("gpg.program"), "")
	if err != nil {
		return errors.Wrap(err, "failed to get gpg public key")
	}

	// Get the public key network ID
	gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	// Get the next nonce, if not set
	if nextNonce == "" {
		nextNonce, err = api.DetermineNextNonceOfGPGKeyOwner(gpgID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Sign a message composed of the tx information
	// fee + nonce + public key id + note hash
	sigMsg := []byte(txFee +
		nextNonce +
		gpgID +
		noteHash +
		fmt.Sprintf("%v", deleteRefAction))
	sig, err := crypto.GPGSign(pkEntity, sigMsg)
	if err != nil {
		return errors.Wrap(err, "failed to sign transaction parameters")
	}

	directives := []string{}
	if deleteRefAction {
		directives = append(directives, "deleteRef")
	}

	// Construct the txparams
	txParams, err := util.MakeAndValidateTxParams(txFee, nextNonce, gpgID, sig, directives...)
	if err != nil {
		return err
	}

	// Create a blob with 0 byte content which be the subject of our note.
	blobHash, err := repo.CreateBlob("")
	if err != nil {
		return err
	}

	// Next we add the tx blob to the note
	if err = repo.AddEntryToNote(note, blobHash, txParams); err != nil {
		return errors.Wrap(err, "failed to add tx blob")
	}

	return nil
}

// CreateAndSendMergeRequestCmd creates merge request proposal
// and sends it to the network
// cfg: App config object
// addrOrIdx: Address or index of account
// passphrase: Passphrase of account
func CreateAndSendMergeRequestCmd(
	cfg *config.AppConfig,
	addrOrIdx,
	passphrase,
	baseBranch,
	baseHash,
	targetBranch,
	targetHash,
	fee,
	nonce string,
	rpcClient *client.RPCClient,
	remoteClients []*restclient.RESTClient) error {

	// Get the signer account
	am := account.New(path.Join(cfg.DataDir(), config.AccountDirName))
	unlocked, err := am.UIUnlockAccount(addrOrIdx, passphrase)
	if err != nil {
		return errors.Wrap(err, "unable to unlock")
	}

	// Create the merge request transaction
	tx := core.NewBareRepoProposalMergeRequest()
	tx.Fee = util.String(fee)
	tx.SenderPubKey = util.StrToPublicKey(unlocked.GetKey().PubKey().Base58())
	tx.BaseBranch = baseBranch
	tx.BaseBranchHash = baseHash
	tx.TargetBranch = targetBranch
	tx.TargetBranchHash = targetHash

	// Determine the next nonce, if unset from flag
	if nonce == "" {
		nonce, err = api.DetermineNextNonceOfAccount(unlocked.Address, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	nonceUInt, err := strconv.ParseUint(nonce, 10, 64)
	if err != nil {
		return err
	}
	tx.SetNonce(nonceUInt)

	pp.Println(tx)

	return nil
}
