package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/make-os/kit/keystore/types"
	"github.com/make-os/kit/remote/policy"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/remote/validation"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	errors2 "github.com/make-os/kit/util/errors"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
)

var (
	ErrPushTokenRequired = fmt.Errorf("push token must be provided")
	ErrMalformedToken    = fmt.Errorf("malformed token")
	fe                   = errors2.FieldErrorWithIndex
)

// AuthenticatorFunc describes a function for performing authentication.
// txDetails: The transaction details for pushed references
// repo: The target repository state.
// namespace: The target namespace.
// keepers: The application states keeper
type AuthenticatorFunc func(
	txDetails []*remotetypes.TxDetail,
	repo *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers,
	checkTxDetail validation.TxDetailChecker) (policy.EnforcerFunc, error)

// authenticate performs authentication checks and returns a policy
// enforcer for later authorization checks.
func authenticate(
	txDetails []*remotetypes.TxDetail,
	repoState *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers,
	checkTxDetail validation.TxDetailChecker) (policy.EnforcerFunc, error) {

	var lastPushKeyID, lastRepoName, lastRepoNamespace string
	var lastAcctNonce uint64
	for i, detail := range txDetails {
		pushKeyID := detail.PushKeyID

		// When there are multiple transaction details, some fields are expected to be the same.
		if i > 0 && pushKeyID != lastPushKeyID {
			return nil, fe(i, "pkID", "all push tokens must be signed with the same push key")
		}
		if i > 0 && detail.RepoName != lastRepoName {
			return nil, fe(i, "repo", "all push tokens must target the same repository")
		}
		if i > 0 && detail.Nonce != lastAcctNonce {
			return nil, fe(i, "nonce", "all push tokens must have the same nonce")
		}
		if i > 0 && detail.RepoNamespace != lastRepoNamespace {
			return nil, fe(i, "namespace", "all push tokens must target the same namespace")
		}

		// Validate the transaction detail
		if err := checkTxDetail(detail, keepers, i); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("token error (%s)", detail.Reference))
		}

		lastPushKeyID, lastRepoName, lastRepoNamespace, lastAcctNonce = pushKeyID, detail.RepoName,
			detail.RepoNamespace, detail.Nonce
	}

	return policy.GetPolicyEnforcer(policy.MakePusherPolicyGroups(txDetails[0].PushKeyID, repoState, namespace)), nil
}

// isPullRequest checks whether a request is a pull request
func isPullRequest(r *http.Request) bool {
	return r.Method == "GET" || strings.Index(r.URL.Path, "git-upload-pack") != -1
}

// handleAuth validates a request using the push request token provided in the url username.
// The push request token is a base58 encode of the serialized transaction information which
// contains the fee, account nonce and request signature.
//
// ARGS:
// - r: The http request
// - repo: The target repository
// - namespace: The namespace object. Nil means default namespace.
func (sv *Server) handleAuth(r *http.Request, repo *state.Repository, namespace *state.Namespace) (txDetails []*remotetypes.TxDetail, polEnforcer policy.EnforcerFunc, err error) {

	// Do not require auth for pull request (yet)
	if isPullRequest(r) {
		return nil, nil, nil
	}

	// Get the request
	tokens, _, _ := r.BasicAuth()

	// We expect push token(s) to be provided
	if tokens == "" {
		return nil, nil, ErrPushTokenRequired
	}

	// Decode the push request token(s)
	txDetails = []*remotetypes.TxDetail{}
	for i, token := range strings.Split(tokens, ",") {
		txDetail, err := DecodePushToken(token)
		if err != nil {
			err = fmt.Errorf("malformed push token at index %d. Unable to decode", i)
			return nil, nil, err
		}
		txDetails = append(txDetails, txDetail)
	}

	// Perform authentication checks
	polEnforcer, err = sv.authenticate(txDetails, repo, namespace, sv.logic, validation.CheckTxDetail)
	if err != nil {
		return nil, nil, err
	}

	return
}

// DecodePushToken decodes a push request token.
func DecodePushToken(v string) (*remotetypes.TxDetail, error) {
	bz, err := base58.Decode(v)
	if err != nil {
		return nil, ErrMalformedToken
	}

	var txDetail remotetypes.TxDetail
	if err = util.ToObject(bz, &txDetail); err != nil {
		return nil, ErrMalformedToken
	}

	return &txDetail, nil
}

// MakePushToken creates a push request token
func MakePushToken(key types.StoredKey, txDetail *remotetypes.TxDetail) string {
	sig, _ := key.GetKey().PrivKey().Sign(txDetail.BytesNoSig())
	txDetail.Signature = base58.Encode(sig)
	return base58.Encode(txDetail.Bytes())
}

// MakeAndApplyPushTokenToRemoteFunc describes a function for creating, signing and
// applying push tokens on a repo's remote(s)
type MakeAndApplyPushTokenToRemoteFunc func(targetRepo remotetypes.LocalRepo, args *MakeAndApplyPushTokenToRemoteArgs) error

// MakeAndApplyPushTokenToRemoteArgs contains arguments for MakeAndApplyPushTokenToRemote
type MakeAndApplyPushTokenToRemoteArgs struct {

	// TargetRemote the name of the remote to update.
	TargetRemote string

	// TxDetail is the transaction information to include in the push token.
	TxDetail *remotetypes.TxDetail

	// PushKey is the key to sign the token.
	PushKey types.StoredKey

	// ResetTokens forces removes all tokens from all URLS before updating.
	ResetTokens bool

	Stderr io.Writer
}

// MakeAndApplyPushTokenToRemote creates and applies signed push token(s) to each remote passed to it.
// It applies the token(s) to the username part of one or more URLs of a remote. Note:
//  - The tokens are cached in the repocfg file.
//  - A URL can have multiple tokens for different references applied to it.
// 	- If the target reference has an existing token, it is replaced with a new one.
//  - Setting args.ResetTokens will remove all existing tokens.
func MakeAndApplyPushTokenToRemote(repo remotetypes.LocalRepo, args *MakeAndApplyPushTokenToRemoteArgs) error {

	gitCfg, err := repo.Config()
	if err != nil {
		return errors.Wrap(err, "failed to get repo config")
	}

	// Get and cleanly split the target remotes
	var chosenRemotes []string
	if targets := strings.ReplaceAll(strings.TrimSpace(args.TargetRemote), " ", ""); targets != "" {
		chosenRemotes = strings.Split(targets, ",")
	}

	// For each current repository remote:
	// - Check that the remote was chosen, if at least one one was chosen.
	// - Prepare the remote URL(s)
	for name, remote := range gitCfg.Remotes {
		if len(chosenRemotes) > 0 && !funk.ContainsString(chosenRemotes, name) {
			continue
		}
		if err := makeAndApplyPushTokenToRepoRemote(args, repo, remote, gitCfg); err != nil {
			return err
		}
	}

	return nil
}

// makeAndApplyPushTokenToRepoRemote creates signed push token(s)
// and adds it/them to one more URLs of a remote. Note:
//  - The tokens are cached in the repocfg file.
//  - A URL can have multiple tokens for different references applied to it.
// 	- If the target reference has an existing token, it is replaced with a new one.
//  - Setting args.ResetTokens will remove all existing tokens.
func makeAndApplyPushTokenToRepoRemote(
	args *MakeAndApplyPushTokenToRemoteArgs,
	repo remotetypes.LocalRepo,
	remote *config.RemoteConfig,
	gitCfg *config.Config,
) error {

	if args.Stderr == nil {
		args.Stderr = ioutil.Discard
	}

	// If the remote has a `kitignore=true` option, return nil
	kitIgnore := gitCfg.Raw.Section("remote").Subsection(remote.Name).Option("kitignore")
	if cast.ToBool(kitIgnore) {
		return nil
	}

	// Get the content of repocfg
	repoCfg, err := repo.GetRepoConfig()
	if err != nil {
		return errors.Wrap(err, "failed to read repocfg file")
	} else if _, ok := repoCfg.Tokens[remote.Name]; !ok {
		repoCfg.Tokens[remote.Name] = []string{}
	}

	// Get existing push tokens from `tokens` option of the remote as long as reset was
	// not requested. Ignore bad tokens or matching tokens of the target reference to
	// avoid creating duplicate tokens.
	var existingTokens = make(map[string]struct{})
	if !args.ResetTokens {
		for _, t := range repoCfg.Tokens[remote.Name] {
			detail, err := DecodePushToken(t)
			if err != nil || detail.Reference == args.TxDetail.Reference {
				continue
			}
			existingTokens[t] = struct{}{}
		}
	}

	var lastRepoName, lastRepoNS string
	for i, v := range remote.URLs {
		remoteUrl, err := url.Parse(v)
		if err != nil {
			_, _ = fmt.Fprintf(args.Stderr, fmt2.RedString("Bad remote remoteUrl (%s) found and skipped", v))
			continue
		}

		// Split the remoteUrl path; ignore urls with less than 2 path sections
		pathPath := strings.Split(strings.Trim(remoteUrl.Path, "/"), "/")
		if len(pathPath) < 2 {
			continue
		}

		// Set repo name and namespace
		txp := *args.TxDetail
		txp.RepoName = pathPath[1]
		if pathPath[0] != remotetypes.DefaultNS {
			txp.RepoNamespace = pathPath[0]
		}

		// Ensure URLS do not point to different repos or namespaces.
		if i > 0 && (txp.RepoName != lastRepoName || txp.RepoNamespace != lastRepoNS) {
			msg := "remote (%s): multiple urls cannot point to different repositories or namespaces"
			return fmt.Errorf(msg, args.TargetRemote)
		}
		lastRepoName, lastRepoNS = txp.RepoName, txp.RepoNamespace

		// Create, sign new token and add to existing tokens list
		existingTokens[MakePushToken(args.PushKey, &txp)] = struct{}{}

		// Use tokens as URL username
		remoteUrl.User = url.UserPassword(strings.Join(funk.Keys(existingTokens).([]string), ","), "-")

		// Added url.insteadOf option
		urlCopy, _ := url.Parse(remoteUrl.String())
		setInstanceOf(gitCfg, urlCopy)
	}

	if err := repo.SetConfig(gitCfg); err != nil {
		return errors.Wrap(err, "failed to update repo config")
	}

	// Update <remote>.tokens in repocfg
	repoCfg.Tokens[remote.Name] = append([]string{}, funk.Keys(existingTokens).([]string)...)
	if err = repo.UpdateRepoConfig(repoCfg); err != nil {
		return errors.Wrap(err, "failed to save token(s)")
	}

	return nil
}

// setInstanceOf sets the instanceOf option for a target url.
// It will remove existing instanceOf sections with a matching target hostname.
func setInstanceOf(cfg *config.Config, targetUrl *url.URL) {

	sections := cfg.Raw.Section("url").Subsections
	for _, sec := range sections {
		secUrl, err := url.Parse(sec.Name)
		if err != nil {
			continue
		}
		if secUrl.Host == targetUrl.Host {
			cfg.Raw.RemoveSubsection("url", sec.Name)
		}
	}

	targetUrl.Path = ""
	subSecVal := targetUrl.String()
	ss := cfg.Raw.Section("url").Subsection(subSecVal)
	targetUrl.User = nil
	ss.AddOption("insteadOf", targetUrl.String())
}
