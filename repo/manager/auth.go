package manager

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/repo/plumbing"
	"gitlab.com/makeos/mosdef/repo/policy"
	"gitlab.com/makeos/mosdef/repo/repo"
	"gitlab.com/makeos/mosdef/repo/validator"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
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
	txDetails []*types.TxDetail,
	repo *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers,
	checkTxDetail validator.TxDetailChecker) (policy.EnforcerFunc, error)

// authenticate performs authentication checks and returns a policy
// enforcer for later authorization checks.
func authenticate(
	txDetails []*types.TxDetail,
	repoState *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers,
	checkTxDetail validator.TxDetailChecker) (policy.EnforcerFunc, error) {

	var lastPushKeyID, lastRepoName, lastRepoNamespace string
	var lastAcctNonce uint64
	for i, detail := range txDetails {
		pushKeyID := detail.PushKeyID

		// When there are multiple transaction details, some fields are expected to be the same.
		if i > 0 && pushKeyID != lastPushKeyID {
			return nil, fe(i, "pkID", "token siblings must be signed with the same push key")
		}
		if i > 0 && detail.RepoName != lastRepoName {
			return nil, fe(i, "repoName", "all push tokens must target the same repository")
		}
		if i > 0 && detail.Nonce != lastAcctNonce {
			return nil, fe(i, "nonce", "all push tokens must have the same nonce")
		}
		if i > 0 && detail.RepoNamespace != lastRepoNamespace {
			return nil, fe(i, "repoNamespace", "all push tokens must target the same namespace")
		}

		// Validate the transaction detail
		if err := checkTxDetail(detail, keepers, i); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("token error"))
		}

		// Check if pusher is an authorized contributor.
		// The pusher is not authorized:
		// - if they are not among repo's contributors.
		// - and namespace is default.
		// - or they are not part of the contributors of the non-nil namespace.
		// Do not check if
		// - detail is a merge push
		// - and target reference is not an issue reference.
		if detail.MergeProposalID == "" && !plumbing.IsIssueBranch(detail.Reference) {
			if !repoState.Contributors.Has(pushKeyID) && (namespace == nil || !namespace.Contributors.Has(pushKeyID)) {
				return nil, fe(-1, "pkID", "pusher is not a contributor")
			}
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

// handleAuth validates a request using the request token provided in the url username.
// The request token is a base58 encode of the serialized transaction information which
// contains the fee, account nonce and request signature.
//
// ARGS:
// - r: The http request
// - repo: The target repository
// - namespace: The namespace object. Nil means default namespace.
func (m *Manager) handleAuth(
	r *http.Request,
	w http.ResponseWriter,
	repo *state.Repository,
	namespace *state.Namespace) (txDetails []*types.TxDetail, polEnforcer policy.EnforcerFunc, err error) {

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
	txDetails = []*types.TxDetail{}
	for i, token := range strings.Split(tokens, ",") {
		txDetail, err := DecodePushToken(token)
		if err != nil {
			err = fmt.Errorf("malformed push token at index %d. Unable to decode", i)
			w.Header().Set("Err", color.RedString(err.Error()))
			return nil, nil, err
		}
		txDetails = append(txDetails, txDetail)
	}

	// Perform authorization checks
	polEnforcer, err = m.authenticate(txDetails, repo, namespace, m.logic, validator.CheckTxDetail)
	if err != nil {
		w.Header().Set("Err", color.RedString(err.Error()))
		return nil, nil, err
	}

	return
}

// DecodePushToken decodes a push request token.
func DecodePushToken(v string) (*types.TxDetail, error) {
	bz, err := base58.Decode(v)
	if err != nil {
		return nil, ErrMalformedToken
	}

	var txDetail types.TxDetail
	if err = util.ToObject(bz, &txDetail); err != nil {
		return nil, ErrMalformedToken
	}

	return &txDetail, nil
}

// MakePushToken creates a push request token
func MakePushToken(key core.StoredKey, txDetail *types.TxDetail) string {
	sig, _ := key.GetKey().PrivKey().Sign(txDetail.BytesNoSig())
	txDetail.Signature = base58.Encode(sig)
	return base58.Encode(txDetail.Bytes())
}

// setPushTokenToRemotes creates a push request token and updates the URLs of all remotes.
// targetRepo: The target repository
// targetRemotes: A list of target remotes whose push URLs will include a push token.
// txDetail: The push request parameters
// pushKey: The push key to use to sign the token.
func setPushTokenToRemotes(
	targetRepo core.BareRepo,
	targetRemote string,
	txDetail *types.TxDetail,
	pushKey core.StoredKey,
	reset bool) (string, error) {

	repoCfg, err := targetRepo.Config()
	if err != nil {
		return "", errors.Wrap(err, "failed to get config")
	}

	remotes := repoCfg.Remotes
	remote, ok := remotes[targetRemote]
	if !ok {
		return "", fmt.Errorf("remote (%s): does not exist", targetRemote)
	}

	// Create and apply tokens to every push URL.
	// For remote with multiple push URLs, ensure the target repositories and namespaces
	// are not different. This is forbidden because the signed reference must include the repository
	// name and namespace in the signature header and if it varies across URLs we won't know which
	// sets of repo and namespace to use.
	// If pushing to different namespaces/repositories is desirable, I recommend creating new remotes as needed.
	lastURLRepoName, lastURLRepoNamespace, token := "", "", ""
	for i, v := range remote.URLs {
		rawURL, err := url.Parse(v)
		if err != nil {
			continue
		}

		// Split the url path; ignore urls with less than 2 path sections
		pathPath := strings.Split(strings.Trim(rawURL.Path, "/"), "/")
		if len(pathPath) < 2 {
			continue
		}

		// Set repo name and namespace
		txp := *txDetail
		txp.RepoName = pathPath[1]
		if pathPath[0] != repo.DefaultNS {
			txp.RepoNamespace = pathPath[0]
		}

		// For remote with multiple push URLs, ensure the target repositories and namespaces
		// are not different. This is forbidden because the signed reference must include the repository
		// name and namespace in the signature header and if it varies across URLs we won't know which
		// sets of repo and namespace to use.
		if i > 0 && (txp.RepoName != lastURLRepoName || txp.RepoNamespace != lastURLRepoNamespace) {
			return "", fmt.Errorf("remote (%s): cannot have multiple urls pointing to "+
				"different repository/namespace", targetRemote)
		}

		// Remove any existing token for the target reference only if reset is false
		var existingTokens []string
		if !reset {
			existingTokens = strings.Split(rawURL.User.Username(), ",")
			existingTokens = funk.FilterString(existingTokens, func(t string) bool {
				if t == "" {
					return false
				}
				txp, err := DecodePushToken(t)
				if err != nil {
					return false
				}
				return txp.Reference != txDetail.Reference
			})
		}

		// Create the signed token and add to existing tokens
		token = MakePushToken(pushKey, &txp)
		existingTokens = append(existingTokens, token)
		rawURL.User = url.UserPassword(strings.Join(existingTokens, ","), "-")
		remote.URLs[i] = rawURL.String()

		lastURLRepoName, lastURLRepoNamespace = txp.RepoName, txp.RepoNamespace
	}

	// Set the push token to the env so that other processes can use it.
	// E.g the signing command called by git needs it for creating a signature.
	if token != "" {
		os.Setenv(fmt.Sprintf("%s_LAST_PUSH_TOKEN", config.AppName), token)
	}

	if err := targetRepo.SetConfig(repoCfg); err != nil {
		return "", errors.Wrap(err, "failed to update config")
	}

	return token, nil
}
