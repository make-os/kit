package repocmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	fmtcfg "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/make-os/kit/cmd/common"
	types3 "github.com/make-os/kit/cmd/signcmd/types"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/remote/types"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util/api"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type HookArgs struct {
	*types3.SignCommitArgs

	// Args is the command arguments
	Args []string

	// PostCommit when true indicates that the hook was called in a post-commit hook
	PostCommit bool

	// RpcClient is the RPC client
	RPCClient types2.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// SetRemotePushToken is a function for creating, signing and applying a push token to a give remote
	SetRemotePushToken server.MakeAndApplyPushTokenToRemoteFunc

	CommitSigner types3.SignCommitFunc
	TagSigner    types3.SignTagFunc
	NoteSigner   types3.SignNoteFunc

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// HookCmd handles git hook operations
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
			if err := args.CommitSigner(cfg, repo, &types3.SignCommitArgs{
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
			if err := args.TagSigner(cfg, []string{name}, repo, &types3.SignTagArgs{
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
			if err := args.NoteSigner(cfg, repo, &types3.SignNoteArgs{
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

// AskPassCmd handles git core.askPass calls.
//
// We use this command to fetch push tokens that may have been created
// and signed by the pre-push hook during git push but cannot be used
// by git since git does not allow the connection to be altered in the
// pre-push hook phase. By returning the push token as a username via
// askPass utility, we can inject the token into the push request.
//
// The command tries to find tokens that have been created and signed for
// remotes where one or more of its urls have matching hostname as the url
// git is requesting password for.
func AskPassCmd(repo types.LocalRepo, args []string, stdout io.Writer) error {

	input := strings.Fields(args[1])

	// Respond to username request.
	if input[0] == "Username" {

		// Get and parse the URL git want's credentials for
		targetRemoteUrl := strings.Trim(strings.TrimRight(input[2], ":"), "'")
		parsedTargetRemoteUrl, err := url.Parse(targetRemoteUrl)
		if err != nil {
			return fmt.Errorf("bad remote url")
		}

		gitCfg, err := repo.Config()
		if err != nil {
			return errors.Wrap(err, "unable to read git config")
		}

		var rawCfg = gitCfg.Raw
		if rawCfg == nil {
			rawCfg = &fmtcfg.Config{}
		}

		// Find remotes with urls that have matching hostname.
		// Skip remotes with kitignore option
		var selected = make(map[string]struct{})
		for _, remote := range rawCfg.Section("remote").Subsections {
			if cast.ToBool(remote.Option("kitignore")) {
				continue
			}
			for _, options := range remote.Options {
				if options.Key != "push" && options.Key != "url" {
					continue
				}
				urlParse, err := url.Parse(options.Value)
				if err != nil {
					continue
				}
				if urlParse.Host == parsedTargetRemoteUrl.Host {
					selected[remote.Name] = struct{}{}
				}
			}
		}

		// Return error if no remotes were found
		if len(selected) == 0 {
			return fmt.Errorf("no push token(s) found")
		}

		// Get repo configuration
		repoCfg, err := repo.GetRepoConfig()
		if err != nil {
			return errors.Wrap(err, "unable to read repocfg file")
		}

		// Get known tokens for all selected remotes
		var tokens []string
		for r := range selected {
			if val, ok := repoCfg.Tokens[r]; ok {
				tokens = append(tokens, val...)
			}
		}

		// Return error if no tokens were found
		if len(tokens) == 0 {
			return fmt.Errorf("no push token(s) found")
		}

		_, _ = fmt.Fprint(stdout, strings.Join(tokens, ","))
	}

	// For password request
	if input[0] == "Password" {
		_, _ = fmt.Fprint(stdout, "-")
	}

	return nil
}
