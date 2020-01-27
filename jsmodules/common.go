package jsmodules

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
)

func castPanic(field string) {
	if err := recover(); err != nil {
		if strings.HasPrefix(err.(error).Error(), "interface conversion") {
			msg := fmt.Sprintf("field '%s' has invalid value type: %s", field, err)
			msg = strings.ReplaceAll(msg, "interface conversion: interface {} is", "has")
			msg = strings.ReplaceAll(msg, "not", "want")
			panic(fmt.Errorf(msg))
		}
		panic(err)
	}
}

// Decode common fields like nonce, fee, timestamp into tx
func decodeCommon(tx types.BaseTx, params map[string]interface{}) {
	if nonce, ok := params["nonce"]; ok {
		defer castPanic("nonce")
		tx.SetNonce(uint64(nonce.(int64)))
	}

	if fee, ok := params["fee"]; ok {
		defer castPanic("fee")
		tx.SetFee(util.String(fee.(string)))
	}

	if timestamp, ok := params["timestamp"]; ok {
		defer castPanic("timestamp")
		tx.SetTimestamp(timestamp.(int64))
	}
}

// finalizeTx sets the public key, timestamp and signs the transaction.
func finalizeTx(tx types.BaseTx, service types.Service, options ...interface{}) {

	key := checkAndGetKey(options...)

	// Set tx public key
	pk, _ := crypto.PrivKeyFromBase58(key)
	tx.SetSenderPubKey(crypto.NewKeyFromPrivKey(pk).PubKey().MustBytes())

	// Set timestamp if not already set
	if tx.GetTimestamp() == 0 {
		tx.SetTimestamp(time.Now().Unix())
	}

	// Set nonce if nonce is not provided
	if tx.GetNonce() == 0 {
		nonce, err := service.GetNonce(tx.GetFrom())
		if err != nil {
			panic(errors.Wrap(err, "failed to get sender's nonce"))
		}
		tx.SetNonce(nonce + 1)
	}

	// Sign the tx
	sig, err := tx.Sign(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to sign transaction"))
	}
	tx.SetSignature(sig)
}

func checkAndGetKey(options ...interface{}) string {
	// - Expect options[0] to be the private key (base58 encoded)
	// - options[0] must be a string
	// - options[0] must be a valid key
	var key string
	var ok bool
	if len(options) > 0 {
		key, ok = options[0].(string)
		if !ok {
			panic(types.ErrArgDecode("string", 1))
		} else if err := crypto.IsValidPrivKey(key); err != nil {
			panic(errors.Wrap(err, types.ErrInvalidPrivKey.Error()))
		}
	} else {
		panic(fmt.Errorf("key is required"))
	}

	return key
}

// EncodeForJS takes a struct and converts
// selected types to values that are compatible in the
// JS environment. It returns a map and will panic
// if obj is not a map/struct.
// Set fieldToIgnore to ignore matching fields
func EncodeForJS(obj interface{}, fieldToIgnore ...string) interface{} {

	if obj == nil {
		return obj
	}

	var m map[string]interface{}

	// If object is a struct, convert to map
	structs.DefaultTagName = "json"
	if structs.IsStruct(obj) {
		m = structs.Map(obj)
	} else {
		m = obj.(map[string]interface{})
	}

	for k, v := range m {
		if funk.InStrings(fieldToIgnore, k) {
			continue
		}

		switch o := v.(type) {
		case int8, []byte:
			m[k] = fmt.Sprintf("0x%x", o)
		case *big.Int, int, int64, uint64:
			m[k] = fmt.Sprintf("%d", o)
		case map[string]interface{}:
			m[k] = EncodeForJS(o)
		case []interface{}:
			for i, item := range o {
				o[i] = EncodeForJS(item)
			}

		// byte types
		case util.BlockNonce:
			m[k] = util.ToHex(o[:])
		case util.Bytes32:
			m[k] = o.HexStr()
		case util.Bytes64:
			m[k] = o.HexStr()

		// map and struct types
		case types.References:
			m[k] = EncodeForJS(util.CloneMap(o))
		case *types.Reference:
			m[k] = EncodeForJS(structs.Map(o))
		}
	}

	return m
}

// EncodeManyForJS is like EncodeForJS but accepts a slice of objects
func EncodeManyForJS(objs interface{}, fieldToIgnore ...string) []interface{} {
	var many []interface{}

	t := reflect.TypeOf(objs)
	if t.Kind() != reflect.Slice {
		panic("not a slice")
	}

	s := reflect.ValueOf(objs)
	for i := 0; i < s.Len(); i++ {
		many = append(many, EncodeForJS(s.Index(i).Interface()))
	}

	return many
}
