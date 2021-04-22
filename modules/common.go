package modules

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/make-os/kit/crypto/ed25519"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	errors2 "github.com/make-os/kit/util/errors"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	StatusCodeServerErr             = "server_err"
	StatusCodeInvalidPass           = "invalid_passphrase"
	StatusCodeAddressRequire        = "addr_required"
	StatusCodeAccountNotFound       = "account_not_found"
	StatusCodeInvalidParam          = "invalid_param"
	StatusCodeInvalidProposerPubKey = "invalid_proposer_pub_key"
	StatusCodeMempoolAddFail        = "err_mempool"
	StatusCodePushKeyNotFound       = "push_key_not_found"
	StatusCodeRepoNotFound          = "repo_not_found"
	StatusCodePathNotFound          = "path_not_found"
	StatusCodePathNotAFile          = "path_not_file"
	StatusCodeReferenceNotFound     = "reference_not_found"
	StatusCodeBranchNotFound        = "branch_not_found"
	StatusCodeCommitNotFound        = "commit_not_found"
	StatusCodeTxNotFound            = "tx_not_found"
)

var se = errors2.ReqErr

// parseOptions parse module options
// If only 1 option, and it is a boolean = payload only instruction.
// If more than 1 options, and it is a string = that's the key
// If more than 1 option = [0] is expected to be the key and [1] the payload only instruction.
// Panics if types are not expected.
// Panics if key is not a valid private key.
func parseOptions(options ...interface{}) (pk *ed25519.PrivKey, payloadOnly bool) {

	var key string
	if len(options) == 1 {
		if v, ok := options[0].(bool); ok {
			payloadOnly = v
		}

		if v, ok := options[0].(string); ok {
			key = v
		}
	}

	if len(options) > 1 {
		var ok bool
		key, ok = options[0].(string)
		if !ok {
			panic(types.ErrIntSliceArgDecode("string", 0, -1))
		}

		payloadOnly, ok = options[1].(bool)
		if !ok {
			panic(types.ErrIntSliceArgDecode("bool", 1, -1))
		}

	}

	if key != "" {
		var err error
		if pk, err = ed25519.PrivKeyFromBase58(key); err != nil {
			panic(errors.Wrap(err, types.ErrInvalidPrivKey.Error()))
		}
	}

	return
}

// finalizeTx sets the public key, timestamp, nonce and signs the transaction.
//
//  - If nonce is not set, it will use the keepers to query the compute the next nonce.
//  - If nonce and keepers are not set, it will use rpcClient to query and compute the next nonce.
//  - It will not alter fields already set.
//  - It will not sign the tx if keeper is not set but RPC client is; This means the
//    call will have to sign the tx with the client.
//
//  - options[0]: <string|bool> 	- key or payloadOnly request
//  - options[1]: [<bool>] 		- payload request
func finalizeTx(tx types.BaseTx, keepers core.Keepers, rpcClient types2.Client, options ...interface{}) (bool, *ed25519.PrivKey) {

	key, payloadOnly := parseOptions(options...)

	// Set sender public key if unset and key was provided
	if tx.GetSenderPubKey().IsEmpty() && key != nil {
		tx.SetSenderPubKey(ed25519.NewKeyFromPrivKey(key).PubKey().MustBytes())
	}

	// Set timestamp if not already set
	if tx.GetTimestamp() == 0 {
		tx.SetTimestamp(time.Now().Unix())
	}

	// If keepers are provider and nonce is unset, compute next nonce of the
	// sending account by using the account using the keeper.
	if tx.GetNonce() == 0 && key != nil && keepers != nil {
		senderAcct := keepers.AccountKeeper().Get(tx.GetFrom())
		if senderAcct.IsNil() {
			panic(se(400, StatusCodeInvalidParam, "senderPubKey", "sender account was not found"))
		}
		tx.SetNonce(senderAcct.Nonce.UInt64() + 1)
	}

	// If nonce is still unset and an RPC client is provided, compute next nonce by
	// using the RPC client to query the sending account nonce.
	if tx.GetNonce() == 0 && key != nil && keepers == nil && rpcClient != nil {
		senderAcct, err := rpcClient.User().Get(tx.GetFrom().String())
		if err != nil {
			panic(err)
		}
		tx.SetNonce(senderAcct.Nonce.UInt64() + 1)
	}

	// Sign the tx only if unsigned, if we have a key and keepers
	if len(tx.GetSignature()) == 0 && key != nil && keepers != nil {
		sig, err := tx.Sign(key.Base58())
		if err != nil {
			panic(se(400, StatusCodeInvalidParam, "key", "failed to sign transaction"))
		}
		tx.SetSignature(sig)
	}

	return payloadOnly, key
}

// Select selects fields and their value from the given JSON string using dot notation.
func Select(json string, selectors ...string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	for i, selector := range selectors {
		match, _ := regexp.MatchString("(?i)^[a-z0-9/_.-]+([.][a-z0-9/_.-]+)*?$", selector)
		if !match {
			return nil, fmt.Errorf("selector at index=%d is malformed", i)
		}

		res := gjson.Get(json, selector)
		selectorParts := strings.Split(selector, ".")
		var m = out
		for i, part := range selectorParts {
			if len(selectorParts) == (i + 1) {
				m[part] = res.Value()
				continue
			}
			if _, ok := m[part]; !ok {
				m[part] = map[string]interface{}{}
			}
			m = m[part].(map[string]interface{})
		}
	}
	return out, nil
}
