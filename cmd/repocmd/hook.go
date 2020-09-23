package repocmd

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/cmd/signcmd"
	"github.com/make-os/lobe/config"
	plumbing2 "github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/server"
	"github.com/make-os/lobe/remote/types"
	types2 "github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/util/api"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type HookArgs struct {
	*signcmd.SignCommitArgs

	// Args is the command arguments
	Args []string

	// RpcClient is the RPC client
	RPCClient types2.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// SetRemotePushToken is a function for setting push tokens on a git remote config
	SetRemotePushToken server.RemotePushTokenSetter

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
		references = append(references, plumbing.ReferenceName(refname))
	}

	// Sign each reference
	for _, ref := range references {
		if ref.IsBranch() {
			if err := args.CommitSigner(cfg, repo, &signcmd.SignCommitArgs{
				Branch:             ref.String(),
				ForceCheckout:      false,
				AmendCommit:        plumbing2.IsMergeRequestReference(ref.String()) || plumbing2.IsIssueReference(ref.String()),
				Remote:             args.Args[0],
				NoPrompt:           true,
				ResetTokens:        false,
				RPCClient:          args.RPCClient,
				KeyUnlocker:        args.KeyUnlocker,
				GetNextNonce:       args.GetNextNonce,
				SetRemotePushToken: args.SetRemotePushToken,
			}); err != nil {
				return err
			}
		}

		if ref.IsTag() {
			name := strings.Replace(ref.String(), "refs/tags/", "", 1)
			if err := args.TagSigner(cfg, []string{name}, repo, &signcmd.SignTagArgs{
				Remote:             args.Args[0],
				NoPrompt:           true,
				Force:              true,
				ResetTokens:        false,
				RPCClient:          args.RPCClient,
				KeyUnlocker:        args.KeyUnlocker,
				GetNextNonce:       args.GetNextNonce,
				SetRemotePushToken: args.SetRemotePushToken,
			}); err != nil {
				return err
			}
		}

		if ref.IsNote() {
			if err := args.NoteSigner(cfg, repo, &signcmd.SignNoteArgs{
				Name:               strings.Replace(ref.String(), "refs/notes/", "", 1),
				Remote:             args.Args[0],
				NoPrompt:           true,
				ResetTokens:        false,
				RPCClient:          args.RPCClient,
				KeyUnlocker:        args.KeyUnlocker,
				GetNextNonce:       args.GetNextNonce,
				SetRemotePushToken: args.SetRemotePushToken,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
