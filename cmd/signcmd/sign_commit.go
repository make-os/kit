package signcmd

import (
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/config"
	plumbing2 "github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/server"
	"github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/remote/validation"
	types2 "github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/api"
	errors2 "github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type SignCommitArgs struct {
	// Fee is the network transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// Value is for sending special fee
	Value string

	// Message is a custom commit message
	Message string

	// AmendCommit indicates whether to amend the last commit or create an empty commit
	AmendCommit bool

	// MergeID indicates an optional merge proposal ID to attach to the transaction
	MergeID string

	// Head specifies a reference to use in the transaction info instead of the signed branch reference
	Head string

	// Branch specifies a branch to checkout and sign instead of the current branch (HEAD)
	Branch string

	// ForceCheckout forcefully checks out the target branch (clears unsaved work)
	ForceCheckout bool

	// ForceSign forcefully signs a commit when signing is supposed to be skipped
	ForceSign bool

	// PushKeyID is the signers push key ID
	SigningKey string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// SignRefOnly indicates that only the target reference should be signed.
	SignRefOnly bool

	// CreatePushTokenOnly indicates that only the remote token should be created and signed.
	CreatePushTokenOnly bool

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// NoPrompt prevents key unlocker prompt
	NoPrompt bool

	// RpcClient is the RPC client
	RPCClient types2.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// SetRemotePushToken is a function for setting push tokens on a git remote config
	SetRemotePushToken server.RemotePushTokenSetter

	// PemDecoder is a function for decoding PEM data
	PemDecoder func(data []byte) (p *pem.Block, rest []byte)

	Stdout io.Writer
	Stderr io.Writer
}

var ErrMissingPushKeyID = fmt.Errorf("push key ID is required")

type SignCommitFunc func(cfg *config.AppConfig, repo types.LocalRepo, args *SignCommitArgs) error

// SignCommitCmd adds transaction information to a new or recent commit and signs it.
// cfg: App config object
// repo: The target repository at the working directory
// args: Arguments
func SignCommitCmd(cfg *config.AppConfig, repo types.LocalRepo, args *SignCommitArgs) error {

	populateSignCommitArgsFromRepoConfig(repo, args)

	// Set merge ID from env if unset
	if args.MergeID == "" {
		args.MergeID = strings.ToUpper(os.Getenv(fmt.Sprintf("%s_MR_ID", cfg.GetAppName())))
	}

	// Signing key is required
	if args.SigningKey == "" {
		return ErrMissingPushKeyID
	}

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: args.SigningKey,
		Passphrase: args.PushKeyPass,
		NoPrompt:   args.NoPrompt,
		TargetRepo: repo,
		Stdout:     args.Stdout,
		Prompt:     "Enter passphrase to unlock the signing key\n",
	})
	if err != nil {
		return errors2.Wrap(err, "failed to unlock the signing key")
	}

	// Get push key from key (args.SigningKey may not be push key address)
	pushKeyID := key.GetPushKeyAddress()

	// Updated the push key passphrase to the actual passphrase used to unlock the key.
	// This is required when the passphrase was gotten via an interactive prompt.
	args.PushKeyPass = objx.New(key.GetMeta().Map()).Get("passphrase").Str(args.PushKeyPass)

	// if MergeID is set, validate it.
	if args.MergeID != "" {
		err = validation.CheckMergeProposalID(args.MergeID, -1)
		if err != nil {
			return fmt.Errorf(err.(*util.BadFieldError).Msg)
		}
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient)
		if err != nil {
			return errors2.Wrapf(err, "failed to get next nonce")
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// Get the current active branch.
	// When branch is explicitly provided, use it as the active branch
	var curHead string
	repoHead, err := repo.Head()
	curHead = repoHead
	if err != nil {
		return fmt.Errorf("failed to get HEAD")
	} else if args.Branch != "" {
		repoHead = args.Branch
		if !plumbing2.IsReference(repoHead) {
			repoHead = plumbing.NewBranchReferenceName(repoHead).String()
		}
	}

	// If an explicit branch was provided via flag, check it out.
	// Then set a deferred function to revert back the the original branch.
	if repoHead != curHead {
		err := repo.Checkout(plumbing.ReferenceName(repoHead).Short(), false, args.ForceCheckout)
		if err != nil {
			return fmt.Errorf("failed to checkout branch (%s): %s", repoHead, err)
		}
		defer repo.Checkout(plumbing.ReferenceName(curHead).Short(), false, false)
	}

	// Use active branch as the tx reference only if args.HEAD was not explicitly provided
	var reference = repoHead
	if args.Head != "" {
		reference = args.Head
		if !plumbing2.IsReference(reference) {
			reference = plumbing.NewBranchReferenceName(args.Head).String()
		}
	}

	// If the APPNAME_REPONAME_PASS var is unset, set it to the user-defined push key pass.
	// This is required to allow git-sign learn the passphrase to unlock the push key.
	passVar := common.MakeRepoScopedEnvVar(cfg.GetAppName(), repo.GetName(), "PASS")
	if len(os.Getenv(passVar)) == 0 {
		os.Setenv(passVar, args.PushKeyPass)
	}

	// Check if current commit has previously been signed. If yes:
	// Skip resigning if push key of current attempt didn't change and only if args.ForceSign is false.
	headObj, _ := repo.HeadObject()
	if headObj != nil && headObj.(*object.Commit).PGPSignature != "" && !args.ForceSign {
		txd, _ := types.DecodeSignatureHeader([]byte(headObj.(*object.Commit).PGPSignature))
		if txd != nil && txd.PushKeyID == pushKeyID {
			goto createToken
		}
	}

	// Skip signing if CreatePushTokenOnly is true
	if args.CreatePushTokenOnly {
		goto createToken
	}

	// If commit amendment is not required, create and sign a new commit instead
	if !args.AmendCommit {
		if err := repo.CreateEmptyCommit(args.Message, args.SigningKey); err != nil {
			return err
		}
		goto createToken
	}

	// At this point, recent commit amendment is required.
	// Ensure there is a commit to amend
	if headObj == nil {
		return errors.New("no commit found; empty branch")
	}

	// Use recent commit message as default if none was provided
	if args.Message == "" {
		args.Message = headObj.(*object.Commit).Message
	}

	// Update the recent commit message.
	if err = repo.AmendRecentCommitWithMsg(args.Message, args.SigningKey); err != nil {
		return err
	}

	// Create & set push request token on the remote in config.
	// Also get the post-sign hash of the current branch.
createToken:

	// Skip token creation if only the reference needs to be signed
	if args.SignRefOnly {
		return nil
	}

	hash, _ := repo.GetRecentCommitHash()
	if _, err = args.SetRemotePushToken(repo, &server.GenSetPushTokenArgs{
		TargetRemote: args.Remote,
		PushKey:      key,
		Stderr:       args.Stderr,
		ResetTokens:  args.ResetTokens,
		TxDetail: &types.TxDetail{
			Fee:             util.String(args.Fee),
			Value:           util.String(args.Value),
			Nonce:           args.Nonce,
			PushKeyID:       pushKeyID,
			MergeProposalID: args.MergeID,
			Reference:       reference,
			Head:            hash,
		},
	}); err != nil {
		return err
	}

	return nil
}

// populateSignCommitArgsFromRepoConfig populates empty arguments field from repo config.
func populateSignCommitArgsFromRepoConfig(repo types.LocalRepo, args *SignCommitArgs) {
	if args.SigningKey == "" {
		args.SigningKey = repo.GetGitConfigOption("user.signingKey")
	}
	if args.PushKeyPass == "" {
		args.PushKeyPass = repo.GetGitConfigOption("user.passphrase")
	}
	if util.IsZeroString(args.Fee) {
		args.Fee = repo.GetGitConfigOption("user.fee")
	}
	if args.Nonce == 0 {
		args.Nonce = cast.ToUint64(repo.GetGitConfigOption("user.nonce"))
	}
	if util.IsZeroString(args.Value) {
		args.Value = repo.GetGitConfigOption("user.value")
	}
	if args.AmendCommit == false {
		args.AmendCommit = cast.ToBool(repo.GetGitConfigOption("commit.amend"))
	}
	if args.MergeID == "" {
		args.MergeID = repo.GetGitConfigOption("sign.mergeID")
	}
}
