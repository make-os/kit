package repo

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/casbin/casbin"
	"github.com/fatih/color"
	"github.com/k0kubun/pp"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

type policyEnforcer func(pushKeyID, object, action string) bool

// authorize performs authorization checks and returns a policy
// enforcer for later authentication checks.
func authorize(
	reqToken *PushRequestTokenData,
	repo *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers) (policyEnforcer, error) {

	// Check the transaction parameters
	if err := checkPushRequestTokenData(reqToken, keepers); err != nil {
		return nil, err
	}

	// Get the push key that signed request token
	signer := keepers.PushKeyKeeper().Get(reqToken.PushKeyID)
	if signer.IsNil() {
		return nil, fmt.Errorf("pusher key is unknown")
	}

	// Using the key, verify the request token signature
	pubKey, err := crypto.PubKeyFromBytes(signer.PubKey.Bytes())
	if err != nil {
		return nil, fmt.Errorf("signer public key from network is invalid") // should never happen
	} else if ok, err := pubKey.Verify(reqToken.GetSigningMsg(), reqToken.Sig); err != nil || !ok {
		return nil, fmt.Errorf("signature is not valid") // should never happen
	}

	// The pusher is not authorized:
	// - if they are not among repo's contributors.
	// - and namespace is default.
	// - or they are not part of the contributors of the non-nil namespace.
	if !repo.Contributors.Has(reqToken.PushKeyID) &&
		(namespace == nil || !namespace.Contributors.Has(reqToken.PushKeyID)) {
		return nil, fmt.Errorf(color.RedString("push key (%s) is not "+
			"authorized to perform this action", reqToken.PushKeyID))
	}

	var policies []*state.RepoACLPolicy

	// Gather the pusher's policies from the repo's namespace and the repo itself.
	if repo.Contributors.Has(reqToken.PushKeyID) {
		for _, p := range repo.Contributors[reqToken.PushKeyID].Policies {
			policies = append(policies, p)
		}
	}
	if namespace != nil && namespace.Contributors.Has(reqToken.PushKeyID) {
		for _, p := range namespace.Contributors[reqToken.PushKeyID].Policies {
			policies = append(policies, p)
		}
	}

	return getPolicyEnforcer(policies), nil
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
	namespace *state.Namespace) (*PushRequestTokenData, policyEnforcer, error) {

	// Get the request
	pushReqToken, _, _ := r.BasicAuth()

	// We expect push request to be provided
	if pushReqToken == "" {
		return nil, nil, fmt.Errorf(color.RedString("Push request token must be provided"))
	}

	// Decode the push request token
	bz, err := base58.Decode(pushReqToken)
	if err != nil {
		return nil, nil, fmt.Errorf(color.RedString("malformed request token. Unable to decode"))
	}
	pushReqTokenData, err := PushRequestTokenDataFromByte(bz)
	if err != nil {
		return nil, nil, fmt.Errorf(color.RedString("malformed request token. Unable to decode"))
	}

	// Perform authorization checks
	enforcer, err := authorize(pushReqTokenData, repo, namespace, m.logic)
	if err != nil {
		return nil, nil, fmt.Errorf(color.RedString(err.Error()))
	}

	return pushReqTokenData, enforcer, nil
}

// getPolicyEnforcer creates and returns a policy enforcer function
// ARGS:
// - policies: A list of the contributor's policies.
func getPolicyEnforcer(policies []*state.RepoACLPolicy) policyEnforcer {
	model := casbin.NewModel(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act`)
	enforcer := casbin.NewEnforcer()
	enforcer.SetModel(model)

	for _, policy := range policies {
		enforcer.AddPolicy(policy.Subject, policy.Object, policy.Action)
	}

	return func(pushKeyID, object, action string) bool {
		return enforcer.HasPolicy(pushKeyID, object, action)
	}
}

// aclCheckCanDelete perform ACL check to determine whether the given GPG key
// is permitted to perform a reference delete operation.
func aclCheckCanDelete(pushKeyID string, polEnforcer policyEnforcer) error {
	pp.Println("Check deletion")
	return nil
}

// PushRequestTokenData represents the information used to create a push request token.
type PushRequestTokenData struct {
	PushKeyID string `json:"pkID" mapstructure:"pkID" msgpack:"pkID"`
	Nonce     uint64 `json:"nonce" mapstructure:"nonce" msgpack:"nonce"`
	Fee       string `json:"fee" mapstructure:"fee" msgpack:"fee"`
	Sig       []byte `json:"sig" mapstructure:"sig" msgpack:"sig"`
}

// GetSigningMsg returns the formatted msg to be signed
func (p *PushRequestTokenData) GetSigningMsg() []byte {
	return []byte(fmt.Sprintf("%s,%d,%s", p.PushKeyID, p.Nonce, p.Fee))
}

// PushRequestTokenDataFromByte deserializes a push request token to a PushRequestTokenData object.
func PushRequestTokenDataFromByte(v []byte) (*PushRequestTokenData, error) {
	bzParts := bytes.Split(v, []byte(","))
	if len(bzParts) != 4 {
		return nil, fmt.Errorf("invalid push request token format")
	}

	p := PushRequestTokenData{}
	p.PushKeyID = string(bzParts[0])
	p.Fee = string(bzParts[2])

	nn, err := strconv.ParseUint(string(bzParts[1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid push request token format: bad next nonce part")
	}
	p.Nonce = nn

	sig, err := base58.Decode(string(bzParts[3]))
	if err != nil {
		return nil, fmt.Errorf("invalid push request token format: bad signature part")
	}
	p.Sig = sig

	return &p, nil
}

// MakePushRequestToken creates a push request token
func MakePushRequestToken(
	pushKeyID string,
	pushKey core.StoredKey,
	nonce uint64,
	fee string) (string, error) {

	msg := (&PushRequestTokenData{PushKeyID: pushKeyID, Nonce: nonce, Fee: fee}).GetSigningMsg()
	sig, err := pushKey.GetKey().PrivKey().Sign(msg)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign")
	}

	finalData := []byte(fmt.Sprintf("%s,%d,%s,%s", pushKeyID, nonce, fee, base58.Encode(sig)))
	return base58.Encode(finalData), err
}

// createAndSetRequestTokenToRemoteURLs creates a push request token and updates the URLs of all remotes.
func createAndSetRequestTokenToRemoteURLs(
	pushKey core.StoredKey,
	targetRepo core.BareRepo,
	txParams *util.TxParams) error {

	// Create a pusher request token
	pushReqToken, err := MakePushRequestToken(txParams.PushKeyID, pushKey, txParams.Nonce, txParams.Fee.String())
	if err != nil {
		return errors.Wrap(err, "failed to create push request token")
	}

	// Get all remotes
	remotes, err := targetRepo.GetRemotes()
	if err != nil {
		return errors.Wrap(err, "failed to get current repo remotes")
	}

	// For each remote, add the request token to every URL.
	// For bad URLS, do nothing.
	for _, remote := range remotes {
		targetRepo.DeleteRemoteURLs(remote.Name)
		for _, v := range remote.URLs {
			rawURL, err := url.Parse(v)
			if err != nil {
				targetRepo.SetRemoteURL(remote.Name, v)
				continue
			}
			rawURL.User = url.UserPassword(pushReqToken, "-")
			targetRepo.SetRemoteURL(remote.Name, rawURL.String())
		}
	}

	return nil
}
