package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/remote/policy"
	remotetypes "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/remote/validation"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	fmt2 "github.com/make-os/lobe/util/colorfmt"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
)

var (
	ErrPushTokenRequired = fmt.Errorf("push token must be provided")
	ErrMalformedToken    = fmt.Errorf("malformed token")
	fe                   = util.FieldErrorWithIndex
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
func (sv *Server) handleAuth(r *http.Request, w http.ResponseWriter, repo *state.Repository,
	namespace *state.Namespace) (txDetails []*remotetypes.TxDetail, polEnforcer policy.EnforcerFunc, err error) {

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

// RemotePushTokenSetter describes a function for setting push tokens on a remote config
type RemotePushTokenSetter func(targetRepo remotetypes.LocalRepo,
	args *SetRemotePushTokenArgs) (string, error)

// SetRemotePushTokenArgs contains arguments for SetRemotePushToken
type SetRemotePushTokenArgs struct {

	// TargetRemote the name of the remote to update.
	// If unset, all remotes are updated.
	TargetRemote string

	// TxDetail is the transaction information to include in the push token.
	TxDetail *remotetypes.TxDetail

	// PushKey is the key to sign the token.
	PushKey types.StoredKey

	// SetRemotePushTokensOptionOnly forces only the tokens remote option to be set.
	SetRemotePushTokensOptionOnly bool

	// ResetTokens forces removes all tokens from all URLS before updating.
	ResetTokens bool

	Stderr io.Writer
}

// SetRemotePushToken creates and sets a push request token for
// URLS and `pushToken` option of a given remote. It will append
// the new token to the existing tokens or clear the existing tokens
// if ResetTokens is true. If TargetRemote is not set, all remotes
// will be updated with the new token.
func SetRemotePushToken(repo remotetypes.LocalRepo, args *SetRemotePushTokenArgs) (string, error) {
	if args.Stderr == nil {
		args.Stderr = ioutil.Discard
	}

	repoCfg, err := repo.Config()
	if err != nil {
		return "", errors.Wrap(err, "failed to get config")
	}

	// If target remote is set, but it is not listed in the config, return err
	if args.TargetRemote != "" && repoCfg.Remotes[args.TargetRemote] == nil {
		return "", fmt.Errorf("remote (%s): does not exist", args.TargetRemote)
	}

	found := false
	lastURLRepoName, lastURLRepoNamespace, token := "", "", ""
	for name, remote := range repoCfg.Remotes {

		if args.TargetRemote != "" && name != args.TargetRemote {
			continue
		}

		if name == args.TargetRemote {
			found = true
		}

		// Get existing push tokens from `tokens` option of the current remote
		// as long reset was not requested. Ignore bad tokens or matching tokens
		// of the target reference to avoid creating duplicate tokens.
		var existingTokens = make(map[string]struct{})
		if !args.ResetTokens {
			pt := strings.TrimSpace(repoCfg.Raw.Section("remote").Subsection(name).Option("tokens"))
			for _, t := range strings.Split(strings.Trim(pt, ","), ",") {
				detail, err := DecodePushToken(t)
				if err != nil || detail.Reference == args.TxDetail.Reference {
					continue
				}
				existingTokens[t] = struct{}{}
			}
		}

		for i, v := range remote.URLs {
			rawURL, err := url.Parse(v)
			if err != nil {
				fmt.Fprintf(args.Stderr, fmt2.RedString("Bad remote url (%s) found and skipped", v))
				continue
			}

			// Split the url path; ignore urls with less than 2 path sections
			pathPath := strings.Split(strings.Trim(rawURL.Path, "/"), "/")
			if len(pathPath) < 2 {
				continue
			}

			// Set repo name and namespace
			txp := *args.TxDetail
			txp.RepoName = pathPath[1]
			if pathPath[0] != remotetypes.DefaultNS {
				txp.RepoNamespace = pathPath[0]
			}

			// Ensure URLS of this remote do not point to different repos or namespaces.
			if i > 0 && (txp.RepoName != lastURLRepoName || txp.RepoNamespace != lastURLRepoNamespace) {
				msg := "remote (%s): multiple urls cannot point to different repositories or namespaces"
				return "", fmt.Errorf(msg, args.TargetRemote)
			}

			// Remove existing token if reset is false.
			// Create and sign new token and add to existing tokens
			token = MakePushToken(args.PushKey, &txp)
			existingTokens[token] = struct{}{}
			tokens := strings.Join(funk.Keys(existingTokens).([]string), ",")

			// Set URL username section to tokens value, if noUsername option is false
			if !args.SetRemotePushTokensOptionOnly {
				rawURL.User = url.UserPassword(tokens, "-")
				remote.URLs[i] = rawURL.String()
			}

			lastURLRepoName, lastURLRepoNamespace = txp.RepoName, txp.RepoNamespace
		}

		// Set remote.*.tokens
		repoCfg.Raw.Section("remote").Subsection(name).
			SetOption("tokens", strings.Join(funk.Keys(existingTokens).([]string), ","))

		if found {
			break
		}
	}

	if err := repo.SetConfig(repoCfg); err != nil {
		return "", errors.Wrap(err, "failed to update config")
	}

	return token, nil
}
