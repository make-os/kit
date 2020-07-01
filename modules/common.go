package modules

import (
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/fatih/structs"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

const (
	StatusCodeAppErr                = "app_err"
	StatusCodeInvalidPass           = "invalid_passphrase"
	StatusCodeAddressRequire        = "addr_required"
	StatusCodeAccountNotFound       = "account_not_found"
	StatusCodeInvalidParam          = "invalid_param"
	StatusCodeInvalidProposerPubKey = "invalid_proposer_pub_key"
	StatusCodeMempoolAddFail        = "mempool_add_err"
	StatusCodePushKeyNotFound       = "push_key_not_found"
	StatusCodeRepoNotFound          = "repo_not_found"
	StatusCodeTxNotFound            = "tx_not_found"
)

var se = util.NewStatusError

// parseOptions parse module options
// If only 1 option, and it is a boolean = payload only instruction.
// If more than 1 options, and it is a string = that's the key
// If more than 1 option = [0] is expected to be the key and [1] the payload only instruction.
// Panics if types are not expected.
// Panics if key is not a valid private key.
func parseOptions(options ...interface{}) (key string, payloadOnly bool) {

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
			panic(types.ErrIntSliceArgDecode("string", 1, 0))
		}

		payloadOnly, ok = options[1].(bool)
		if !ok {
			panic(types.ErrIntSliceArgDecode("bool", 1, 0))
		}

		if err := crypto.IsValidPrivKey(key); err != nil {
			panic(errors.Wrap(err, types.ErrInvalidPrivKey.Error()))
		}
	}

	return
}

// finalizeTx sets the public key, timestamp and signs the transaction.
//
// options[0] is expected to be a base58 private key
//
// If options[1] is set to true, true is returned; meaning the user only wants
// the finalized payload and does not want to send the transaction to the network
func finalizeTx(tx types.BaseTx, keepers core.Keepers, options ...interface{}) bool {

	key, payloadOnly := parseOptions(options...)

	// Set timestamp if not already set
	if tx.GetTimestamp() == 0 {
		tx.SetTimestamp(time.Now().Unix())
	}

	// Set nonce if nonce is not provided
	if tx.GetNonce() == 0 {
		if tx.GetSenderPubKey().IsEmpty() {
			panic(se(400, StatusCodeInvalidParam, "senderPubKey", "sender public key was not set"))
		}

		senderAcct := keepers.AccountKeeper().Get(tx.GetFrom())
		if senderAcct.IsNil() {
			panic(se(400, StatusCodeInvalidParam, "senderPubKey", "sender account was not found"))
		}
		tx.SetNonce(senderAcct.Nonce + 1)
	}

	// If no key, we can't sign, so return.
	if key == "" {
		return payloadOnly
	}

	// Set tx public key only if unset
	if tx.GetSenderPubKey().IsEmpty() {
		pk, _ := crypto.PrivKeyFromBase58(key)
		tx.SetSenderPubKey(crypto.NewKeyFromPrivKey(pk).PubKey().MustBytes())
	}

	// Sign the tx only if unsigned
	if len(tx.GetSignature()) == 0 {
		sig, err := tx.Sign(key)
		if err != nil {
			panic(se(400, StatusCodeInvalidParam, "key", "failed to sign transaction"))
		}
		tx.SetSignature(sig)
	}

	return payloadOnly
}

// normalizeUtilMap normalizes a struct or a map for specific environment,
// returning a util.Map object. Panics if res is not a map or struct.
func normalizeUtilMap(env modulestypes.Env, res interface{}, fieldToIgnore ...string) util.Map {
	return Normalize(env, res, fieldToIgnore...).(util.Map)
}

// normalizeSliceUtilMap normalizes a slice of struct or a slice map for specific
// environment, returning a slice of util.Map object.
// Panics if res is not a slice of map or struct.
func normalizeSliceUtilMap(env modulestypes.Env, res interface{}, fieldToIgnore ...string) []util.Map {
	nRes := Normalize(env, res, fieldToIgnore...)
	if nRes == nil {
		return []util.Map{}
	}
	return nRes.([]util.Map)
}

// Normalize normalizes a map, struct or slice of struct/map for a given environment.
// Panics if res is not a slice of map or struct.
func Normalize(env modulestypes.Env, res interface{}, ignoreFields ...string) interface{} {

	// Return nil result is nil
	if res == nil {
		panic("nil result not allowed")
	}

	// Convert input object to map
	m := make(map[string]interface{})
	val := reflect.ValueOf(res)
	switch val.Kind() {

	case reflect.Ptr:
		return Normalize(env, val.Elem().Interface(), ignoreFields...)

	// Convert struct to map
	case reflect.Struct:
		m = util.StructToMap(res, "json")

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
			res = append(res, Normalize(env, val.Index(i).Interface(), ignoreFields...).(util.Map))
		}
		return res

	default:
		panic("only struct, map or map slice are allowed")
	}

	// If environment is RPC, return object immediately.
	// We don't need to Normalize RPC result client response.
	if env == modulestypes.NORMAL {
		return util.Map(m)
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
			m[k] = Normalize(env, v, ignoreFields...)
		case map[string]interface{}:
			if len(o) > 0 { // no need adding empty maps
				if util.IsMapOrStruct(o) {
					m[k] = Normalize(env, o, ignoreFields...)
				}
			}
		case []interface{}:
			for i, item := range o {
				if util.IsMapOrStruct(item) {
					o[i] = Normalize(env, item, ignoreFields...)
				}
			}

		// byte types
		case util.BlockNonce:
			m[k] = util.ToHex(o[:])
		case util.Bytes32:
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
				m[k] = Normalize(env, newMap, ignoreFields...)
			} else if kind == reflect.Struct {
				m[k] = Normalize(env, structs.Map(o), ignoreFields...)
			}
		}
	}

	return util.Map(m)
}
