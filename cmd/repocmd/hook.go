package repocmd

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/cmd/signcmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/remote/types"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util/api"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type HookArgs struct {
	*signcmd.SignCommitArgs

	// Args is the command arguments
	Args []string

	// PostCommit when true indicates that the hook was called in a post-commit hook
	PostCommit bool

	// RpcClient is the RPC client
	RPCClient types2.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// SetRemotePushToken is a function for creating, signing and apply a push token  to a give remote
	SetRemotePushToken server.MakeAndApplyPushTokenToRemoteFunc

	CommitSigner signcmd.SignCommitFunc
	TagSigner    signcmd.SignTagFunc
	NoteSigner   signcmd.SignNoteFunc

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// HookCmd handles pre-push calls by git
func HookCmd(cfg *config.AppConfig, repo types.LocalRepo, args *HookArgs) error {

	updates, err := ioutil.ReadAll(args.Stdin)
	if err != nil {
		return err
	}

	// Read the references to be updated
	var references []plumbing.ReferenceName
	for _, line := range strings.Split(strings.TrimSpace(string(updates)), "\n") {
		refname := strings.Split(strings.TrimSpace(line), " ")[0]
		if refname != "" {
			references = append(references, plumbing.ReferenceName(refname))
		}
	}

	// If called in a post-commit hook, use the current HEAD as the reference
	if args.PostCommit {
		head, err := repo.Head()
		if err != nil {
			return errors.Wrapf(err, "failed to get HEAD")
		}
		references = append(references, plumbing.ReferenceName(head))
	}

	// Sign each reference
	for _, ref := range references {
		var remote string
		if len(args.Args) > 0 {
			remote = args.Args[0]
		}

		if ref.IsBranch() {
			if err := args.CommitSigner(cfg, repo, &signcmd.SignCommitArgs{
				Head:                         ref.String(),
				Remote:                       remote,
				NoPrompt:                     true,
				ResetTokens:                  false,
				RPCClient:                    args.RPCClient,
				KeyUnlocker:                  args.KeyUnlocker,
				GetNextNonce:                 args.GetNextNonce,
				CreateApplyPushTokenToRemote: args.SetRemotePushToken,
			}); err != nil {
				return err
			}
		}

		if ref.IsTag() {
			name := strings.Replace(ref.String(), "refs/tags/", "", 1)
			if err := args.TagSigner(cfg, []string{name}, repo, &signcmd.SignTagArgs{
				Remote:                       remote,
				NoPrompt:                     true,
				ResetTokens:                  false,
				RPCClient:                    args.RPCClient,
				KeyUnlocker:                  args.KeyUnlocker,
				GetNextNonce:                 args.GetNextNonce,
				CreateApplyPushTokenToRemote: args.SetRemotePushToken,
			}); err != nil {
				return err
			}
		}

		if ref.IsNote() {
			if err := args.NoteSigner(cfg, repo, &signcmd.SignNoteArgs{
				Name:                         strings.Replace(ref.String(), "refs/notes/", "", 1),
				Remote:                       remote,
				NoPrompt:                     true,
				ResetTokens:                  false,
				RPCClient:                    args.RPCClient,
				KeyUnlocker:                  args.KeyUnlocker,
				GetNextNonce:                 args.GetNextNonce,
				CreateApplyPushTokenToRemote: args.SetRemotePushToken,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
