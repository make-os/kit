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

// RemotePushTokenSetter describes a function for setting push tokens on a remote config
type RemotePushTokenSetter func(targetRepo remotetypes.LocalRepo, args *GenSetPushTokenArgs) (string, error)

// GenSetPushTokenArgs contains arguments for GenSetPushToken
type GenSetPushTokenArgs struct {

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

// GenSetPushToken generates a push request token for a target remote and
// updates the credential file with a url that includes the token. It will
// also store the token under the <remote>.tokens option in .git/repocfg
// for future/third-party use.
//
// Existing tokens found in <remote>.tokens of repocfg are concatenated and
// used to update the credential file. Existing tokens that point to same
// reference as the target reference of a call will be replaced with the
// token generated in the call.
func GenSetPushToken(repo remotetypes.LocalRepo, args *GenSetPushTokenArgs) (string, error) {
	if args.Stderr == nil {
		args.Stderr = ioutil.Discard
	}

	// Get the repo configuration
	gitCfg, err := repo.Config()
	if err != nil {
		return "", errors.Wrap(err, "failed to get config")
	}

	// If target remote is set, but it is not listed in the config, return err
	remote := gitCfg.Remotes[args.TargetRemote]
	if remote == nil {
		return "", fmt.Errorf("remote was not found")
	}

	// Get the content of repocfg
	repoCfg, err := repo.GetRepoConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to read repocfg file")
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

	var lastRepoName, lastRepoNS, token string
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

		// Ensure URLS do not point to different repos or namespaces.
		if i > 0 && (txp.RepoName != lastRepoName || txp.RepoNamespace != lastRepoNS) {
			msg := "remote (%s): multiple urls cannot point to different repositories or namespaces"
			return "", fmt.Errorf(msg, args.TargetRemote)
		}
		lastRepoName, lastRepoNS = txp.RepoName, txp.RepoNamespace

		// Create, sign new token and add to existing tokens list
		token = MakePushToken(args.PushKey, &txp)
		existingTokens[token] = struct{}{}

		// Use tokens as URL username and update credential file
		rawURL.User = url.UserPassword(strings.Join(funk.Keys(existingTokens).([]string), ","), "-")
		repo.UpdateCredentialFile(rawURL.String())
	}

	// Update <remote>.tokens in repocfg
	repoCfg.Tokens[remote.Name] = append([]string{}, funk.Keys(existingTokens).([]string)...)
	if err = repo.UpdateRepoConfig(repoCfg); err != nil {
		return "", errors.Wrap(err, "failed to save token(s)")
	}

	return token, nil
}
