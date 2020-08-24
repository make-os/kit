package modules

import (
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/fatih/structs"
	"github.com/make-os/lobe/api/rpc/client"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
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
	StatusCodeTxNotFound            = "tx_not_found"
)

var se = util.ReqErr

// parseOptions parse module options
// If only 1 option, and it is a boolean = payload only instruction.
// If more than 1 options, and it is a string = that's the key
// If more than 1 option = [0] is expected to be the key and [1] the payload only instruction.
// Panics if types are not expected.
// Panics if key is not a valid private key.
func parseOptions(options ...interface{}) (pk *crypto.PrivKey, payloadOnly bool) {

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
		if pk, err = crypto.PrivKeyFromBase58(key); err != nil {
			panic(errors.Wrap(err, types.ErrInvalidPrivKey.Error()))
		}
	}

	return
}

// finalizeTx sets the public key, timestamp, nonce and signs the transaction.
// If nonce is not set, it will use the keepers to query the compute the next nonce.
// If nonce and keepers are not set, it will use rpcClient to query and compute the next nonce.
// It will not reset fields already set.
// options[0]: <string|bool> 	- key or payloadOnly request
// options[1]: [<bool>] 		- payload request
func finalizeTx(tx types.BaseTx, keepers core.Keepers, rpcClient client.Client, options ...interface{}) (bool, *crypto.PrivKey) {

	key, payloadOnly := parseOptions(options...)

	// Set sender public key if unset and key was provided
	if tx.GetSenderPubKey().IsEmpty() && key != nil {
		tx.SetSenderPubKey(crypto.NewKeyFromPrivKey(key).PubKey().MustBytes())
	}

	// Set timestamp if not already set
	if tx.GetTimestamp() == 0 {
		tx.SetTimestamp(time.Now().Unix())
	}

	// If nonce us unset, compute next nonce by using the account keeper to query
	// the sender account only if keepers is set.
	if tx.GetNonce() == 0 && key != nil && keepers != nil {
		senderAcct := keepers.AccountKeeper().Get(tx.GetFrom())
		if senderAcct.IsNil() {
			panic(se(400, StatusCodeInvalidParam, "senderPubKey", "sender account was not found"))
		}
		tx.SetNonce(senderAcct.Nonce.UInt64() + 1)
	}

	// If nonce us unset, compute next nonce by using the RPC client to query
	// the sender account only if keepers is unset.
	if tx.GetNonce() == 0 && key != nil && keepers == nil && rpcClient != nil {
		senderAcct, err := rpcClient.GetAccount(tx.GetFrom().String())
		if err != nil {
			panic(err)
		}
		tx.SetNonce(senderAcct.Nonce.UInt64() + 1)
	}

	// Sign the tx only if unsigned
	if len(tx.GetSignature()) == 0 && key != nil {
		sig, err := tx.Sign(key.Base58())
		if err != nil {
			panic(se(400, StatusCodeInvalidParam, "key", "failed to sign transaction"))
		}
		tx.SetSignature(sig)
	}

	return payloadOnly, key
}

// Normalize normalizes a map, struct or slice of struct/map.
func Normalize(res interface{}, ignoreFields ...string) interface{} {

	// Return nil result is nil
	if res == nil {
		panic("nil result not allowed")
	}

	// Convert input object to map
	m := make(map[string]interface{})
	val := reflect.ValueOf(res)
	switch val.Kind() {

	case reflect.Ptr:
		return Normalize(val.Elem().Interface(), ignoreFields...)

	// Convert struct to map
	case reflect.Struct:
		m = util.ToMap(res, "json")

	// Convert map to map[string]interface{}
	case reflect.Map:
		for _, k := range val.MapKeys() {
			m[k.String()] = val.MapIndex(k).Interface()
		}

	// Normalize each elements in the slice.
	// Panics if element is not a struct, slice of map/struct and map type
	case reflect.Slice:
		var res []util.Map
		for i := 0; i < val.Len(); i++ {
			res = append(res, Normalize(val.Index(i).Interface(), ignoreFields...).(util.Map))
		}
		return res

	default:
		panic("only struct, map or map slice are allowed")
	}

	for k, v := range m {
		if funk.InStrings(ignoreFields, k) {
			continue
		}

		switch o := v.(type) {
		case int8, []byte:
			m[k] = fmt.Sprintf("0x%x", o)
		case *big.Int, uint32, int64, uint64:
			m[k] = fmt.Sprintf("%d", o)
		case float64:
			m[k] = fmt.Sprintf("%s", decimal.NewFromFloat(o).String())
		case map[string][]byte:
			m[k] = Normalize(v, ignoreFields...)
		case map[string]interface{}:
			if len(o) > 0 { // no need adding empty maps
				if util.IsMapOrStruct(o) {
					m[k] = Normalize(o, ignoreFields...)
				}
			}
		case []interface{}:
			for i, item := range o {
				if util.IsMapOrStruct(item) {
					o[i] = Normalize(item, ignoreFields...)
				}
			}

		// byte types
		case util.BlockNonce:
			m[k] = util.ToHex(o[:])
		case util.Bytes32:
			m[k] = o.HexStr()
		case util.Bytes:
			m[k] = o.HexStr()
		case util.Bytes64:
			m[k] = o.HexStr()
		case crypto.PublicKey:
			m[k] = crypto.MustPubKeyFromBytes(o[:]).Base58()
		case crypto.PushKey:
			m[k] = crypto.BytesToPushKeyID(o[:])

		// custom wrapped map[string]struct
		// custom wrapped map[string]string
		default:
			v := reflect.ValueOf(o)
			kind := v.Kind()
			if kind == reflect.Map {
				newMap := make(map[string]interface{})
				for _, key := range v.MapKeys() {
					mapVal := v.MapIndex(key)
					if structs.IsStruct(mapVal.Interface()) {
						newMap[key.String()] = structs.Map(mapVal.Interface())
					} else if mapValStr, ok := mapVal.Interface().(string); ok {
						newMap[key.String()] = mapValStr
					}
				}
				m[k] = Normalize(newMap, ignoreFields...)
			} else if kind == reflect.Struct {
				m[k] = Normalize(structs.Map(o), ignoreFields...)
			}
		}
	}

	return util.Map(m)
}
