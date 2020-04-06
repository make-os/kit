package repo

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var (
	ErrPushTokenRequired = fmt.Errorf("push token must be provided")
	ErrMalformedToken    = fmt.Errorf("malformed token")
)

// policyEnforcer describes a function used for checking policies.
// subject: The policy subject
// object: The policy object
// action: The policy action
type policyEnforcer func(subject, object, action string) (bool, int)

// authenticator describes a function for performing authentication.
// txDetails: The transaction details for pushed references
// repo: The target repository state.
// namespace: The target namespace.
// keepers: The application states keeper
type authenticator func(
	txDetails []*types.TxDetail,
	repo *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers) (policyEnforcer, error)

// authenticate performs authentication checks and returns a policy
// enforcer for later authorization checks.
func authenticate(
	txDetails []*types.TxDetail,
	repoState *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers) (policyEnforcer, error) {

	// Check all the transaction parameters individually
	for i, txDetail := range txDetails {
		if err := checkTxDetail(txDetail, keepers, true); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("token(%d,ref=%s)", i, txDetail.Reference))
		}
	}

	var lastPushKeyID, lastRepoName, lastRepoNamespace string
	var lastAcctNonce uint64

	// Proceed to policy enforcer creation when only one tx details was passed
	if len(txDetails) == 1 {
		goto makeEnforcer
	}

	// When there are multiple pushed references, specific fields must be the same.
	for i, detail := range txDetails {
		if i > 0 && detail.PushKeyID != lastPushKeyID {
			return nil, fmt.Errorf("pushed references cannot be signed with different push keys")
		}
		if i > 0 && detail.RepoName != lastRepoName {
			return nil, fmt.Errorf("pushed references cannot target different repositories")
		}
		if i > 0 && detail.Nonce != lastAcctNonce {
			return nil, fmt.Errorf("pushed references cannot have different nonce")
		}
		if i > 0 && detail.RepoNamespace != lastRepoNamespace {
			return nil, fmt.Errorf("pushed references cannot target different namespaces")
		}
		lastPushKeyID, lastRepoName, lastRepoNamespace = detail.PushKeyID, detail.RepoName, detail.RepoNamespace
		lastAcctNonce = detail.Nonce
	}

makeEnforcer:

	// The pusher is not authorized:
	// - if they are not among repo's contributors.
	// - and namespace is default.
	// - or they are not part of the contributors of the non-nil namespace.
	pushKeyID := txDetails[0].PushKeyID
	if !repoState.Contributors.Has(pushKeyID) &&
		(namespace == nil || !namespace.Contributors.Has(pushKeyID)) {
		return nil, fmt.Errorf("push key (%s) is not a repo (%s) contributor", pushKeyID, txDetails[0].RepoName)
	}

	return getPolicyEnforcer(makePusherPolicyGroups(pushKeyID, repoState, namespace)), nil
}

// makePusherPolicyGroups creates a policy group contain the different category of policies
// a pusher can have. Currently, we have 3 policy levels namely, repo default policies,
// namespace contributor policies and repo contributor policies. Policies of lower slice
// indices take precedence than those at higher indices.
//
// Policy levels:
// - 0: Repo's contributor policy collection (highest precedence)
// - 1: Repo's namespace's contributor policy collection
// - 2: Repo's config policy collection
func makePusherPolicyGroups(
	pushKeyID string,
	repoState *state.Repository,
	namespace *state.Namespace) [][]*state.Policy {

	// Gather the policies into groups
	var groups = make([][]*state.Policy, 3)

	// Find policies in the config-level policies where the subject is "all" or the pusher key ID
	// and also whose object is points to a reference path or name
	for _, pol := range repoState.Config.Policies {
		if (pol.Subject == "all" || pol.Subject == pushKeyID) && isReference(pol.Object) {
			groups[2] = append(groups[2], pol)
		}
	}

	// Add the pusher's namespace-level contributor policies
	if namespace != nil && namespace.Contributors.Has(pushKeyID) {
		for _, p := range namespace.Contributors[pushKeyID].Policies {
			groups[1] = append(groups[1], &state.Policy{Subject: pushKeyID, Object: p.Object, Action: p.Action})
		}
	}

	// Add the pusher's repo-level contributor policies
	if repoState.Contributors.Has(pushKeyID) {
		for _, p := range repoState.Contributors[pushKeyID].Policies {
			groups[0] = append(groups[0], &state.Policy{Subject: pushKeyID, Object: p.Object, Action: p.Action})
		}
	}

	return groups
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
	repo *state.Repository,
	namespace *state.Namespace) (txDetails []*types.TxDetail, polEnforcer policyEnforcer, err error) {

	if r.Method == "GET" {
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
			return nil, nil, fmt.Errorf("malformed push token at index %d. Unable to decode", i)
		}
		txDetails = append(txDetails, txDetail)
	}

	// Perform authorization checks
	polEnforcer, err = m.authenticate(txDetails, repo, namespace, m.logic)
	if err != nil {
		return nil, nil, err
	}

	return
}

// getPolicyEnforcer returns a policy enforcer function used for enforcing policies against a subject.
func getPolicyEnforcer(policyGroup [][]*state.Policy) policyEnforcer {
	enforcer := newPolicyEnforcer(policyGroup)
	return func(subject, object, action string) (bool, int) {
		return enforcer.Enforce(subject, object, action)
	}
}

// policyChecker describes a function for enforcing repository policy
type policyChecker func(enforcer policyEnforcer, pushKeyID, reference, action string) error

// checkPolicy performs ACL checks to determine whether the given push key
// is permitted to perform the given action on the reference subject.
func checkPolicy(enforcer policyEnforcer, pushKeyID, reference, action string) error {

	dir := "refs/"
	if isBranch(reference) {
		dir = dir + "heads"
	} else if isTag(reference) {
		dir = dir + "tags"
	} else if isNote(reference) {
		dir = dir + "notes"
	} else {
		return fmt.Errorf("unknown reference (%s)", reference)
	}

	var negativeAct = "deny-" + action
	var allowed bool
	var highestLvl = 999 // Set default to a random, high number greater than all levels

	// Check if all push keys can or cannot perform the action to the target reference
	res, lvl := enforcer("all", reference, action)
	if lvl > -1 {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer("all", reference, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Check if all push keys can or cannot perform the action on the target reference directory
	res, lvl = enforcer("all", dir, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer("all", dir, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Check if the push key can or cannot perform the action on the reference
	res, lvl = enforcer(pushKeyID, reference, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer(pushKeyID, reference, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Check if the push key can or cannot perform the action on the reference directory
	res, lvl = enforcer(pushKeyID, dir, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer(pushKeyID, dir, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	if !allowed {
		return fmt.Errorf("reference (%s): not authorized to perform '%s' action", reference, action)
	}

	return nil
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
		if pathPath[0] != DefaultNS {
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
