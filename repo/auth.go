package repo

import (
	"bytes"
	"encoding/hex"
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
	"golang.org/x/crypto/openpgp"
)

type policyEnforcer func(gpgID, object, action string) bool

// authorize performs authorization checks and returns a policy
// enforcer for later authentication checks.
func authorize(
	pushReqToken *PushRequestTokenData,
	repo *state.Repository,
	namespace *state.Namespace,
	keepers core.Keepers) (policyEnforcer, error) {

	// Check the transaction parameters
	if err := checkPushRequestTokenData(pushReqToken, keepers); err != nil {
		return nil, err
	}

	// Get the GPG key that signed the transaction params object
	signer := keepers.GPGPubKeyKeeper().Get(pushReqToken.GPGID)
	if signer.IsNil() {
		return nil, fmt.Errorf("pusher gpg key is unknown")
	}

	// Using the GPG key, verify the transaction parameters signature
	entity, err := crypto.PGPEntityFromPubKey(signer.PubKey)
	if err != nil {
		return nil, fmt.Errorf("signer public key from network is invalid") // should never happen
	}
	ok, err := crypto.VerifyGPGSignature(entity, pushReqToken.Sig, pushReqToken.GetSigningMsg())
	if err != nil || !ok {
		return nil, fmt.Errorf("signature is not valid") // should never happen
	}

	// The pusher is not authorized:
	// - if they are not among repo's contributors.
	// - and namespace is default.
	// - or they are not part of the contributors of the non-nil namespace.
	if !repo.Contributors.Has(pushReqToken.GPGID) &&
		(namespace == nil || !namespace.Contributors.Has(pushReqToken.GPGID)) {
		return nil, fmt.Errorf(color.RedString("GPG key (%s) is not "+
			"authorized to perform this action", pushReqToken.GPGID))
	}

	var policies []*state.RepoACLPolicy

	// Gather the pusher's policies from the repo's namespace and the repo itself.
	if repo.Contributors.Has(pushReqToken.GPGID) {
		for _, p := range repo.Contributors[pushReqToken.GPGID].Policies {
			policies = append(policies, p)
		}
	}
	if namespace != nil && namespace.Contributors.Has(pushReqToken.GPGID) {
		for _, p := range namespace.Contributors[pushReqToken.GPGID].Policies {
			policies = append(policies, p)
		}
	}

	return getPolicyEnforcer(policies), nil
}

// handleAuth validates a request using the request token provided in the url username.
// The request token is a base58 encode of the serialized transaction information which
// contains the fee, keystore nonce and request signature.
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

	return func(gpgID, object, action string) bool {
		return enforcer.HasPolicy(gpgID, object, action)
	}
}

// aclCheckCanDelete perform ACL check to determine whether the given GPG key
// is permitted to perform a reference delete operation.
func aclCheckCanDelete(gpgID string, polEnforcer policyEnforcer) error {
	pp.Println("Check deletion")
	return nil
}

// PushRequestTokenData represents the information used to create a push request token
type PushRequestTokenData struct {
	GPGID string `json:"gpgID" mapstructure:"gpgID" msgpack:"gpgID"`
	Nonce uint64 `json:"nonce" mapstructure:"nonce" msgpack:"nonce"`
	Fee   string `json:"fee" mapstructure:"fee" msgpack:"fee"`
	Sig   []byte `json:"sig" mapstructure:"sig" msgpack:"sig"`
}

// GetSigningMsg returns the formatted msg to be signed
func (p *PushRequestTokenData) GetSigningMsg() []byte {
	return []byte(fmt.Sprintf("%s,%d,%s", p.GPGID, p.Nonce, p.Fee))
}

// PushRequestTokenDataFromByte creates and populates a PushRequestTokenData object from a byte slice
func PushRequestTokenDataFromByte(v []byte) (*PushRequestTokenData, error) {
	bzParts := bytes.Split(v, []byte(","))
	if len(bzParts) != 4 {
		return nil, fmt.Errorf("invalid push request token format")
	}

	p := PushRequestTokenData{}
	p.GPGID = string(bzParts[0])
	p.Fee = string(bzParts[2])

	nn, err := strconv.ParseUint(string(bzParts[1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid push request token format: bad next nonce part")
	}
	p.Nonce = nn

	sig, err := hex.DecodeString(string(bzParts[3]))
	if err != nil {
		return nil, fmt.Errorf("invalid push request token format: bad signature part")
	}
	p.Sig = sig

	return &p, nil
}

// MakePushRequestToken creates a push request token
func MakePushRequestToken(gpgID string, nonce uint64, fee string, pk *openpgp.Entity) (string, error) {
	msg := (&PushRequestTokenData{GPGID: gpgID, Nonce: nonce, Fee: fee}).GetSigningMsg()
	sig, err := crypto.GPGSign(pk, msg)
	if err != nil {
		return "", errors.Wrap(err, "sign failure")
	}
	finalData := []byte(fmt.Sprintf("%s,%d,%s,%x", gpgID, nonce, fee, sig))
	return base58.Encode(finalData), err
}

// createAndSetRequestTokenToRemoteURLs creates a push request token and updates the URLs of all remotes.
func createAndSetRequestTokenToRemoteURLs(
	signingKey string,
	targetRepo core.BareRepo,
	txParams *util.TxParams) error {

	// Get the GPG private key
	pk, err := crypto.GetGPGPrivateKey(signingKey, targetRepo.GetConfig("gpg.program"), "")
	if err != nil {
		return errors.Wrap(err, "failed to get GPG private key")
	}

	// Create a pusher request token used for authentication
	pushReqToken, err := MakePushRequestToken(txParams.GPGID, txParams.Nonce, txParams.Fee.String(), pk)
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
