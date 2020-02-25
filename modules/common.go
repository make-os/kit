package modules

import (
	"fmt"
	"math/big"
	"reflect"
	"time"

	types2 "gitlab.com/makeos/mosdef/services/types"

	"github.com/fatih/structs"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
)

// finalizeTx sets the public key, timestamp and signs the transaction.
//
// options[0] is expected to be a base58 private key
//
// If options[1] is set to true, true is returned; meaning the user only wants
// the finalized payload and does not want to send the transaction to the network
func finalizeTx(tx types.BaseTx, service types2.Service, options ...interface{}) (payloadOnly bool) {

	// Set timestamp if not already set
	if tx.GetTimestamp() == 0 {
		tx.SetTimestamp(time.Now().Unix())
	}

	if len(options) > 1 {
		payloadOnly, _ = options[1].(bool)
	}

	// Set public key and sign the transaction only when a key is provided.
	if len(options) > 0 {

		key := checkAndGetKey(options...)

		// Set tx public key only if unset
		if tx.GetSenderPubKey().IsEmpty() {
			pk, _ := crypto.PrivKeyFromBase58(key)
			tx.SetSenderPubKey(crypto.NewKeyFromPrivKey(pk).PubKey().MustBytes())
		}

		// Set nonce if nonce is not provided
		if tx.GetNonce() == 0 {
			nonce, err := service.GetNonce(tx.GetFrom())
			if err != nil {
				panic(errors.Wrap(err, "failed to get sender's nonce"))
			}
			tx.SetNonce(nonce + 1)
		}

		// Sign the tx only if unsigned
		if len(tx.GetSignature()) == 0 {
			sig, err := tx.Sign(key)
			if err != nil {
				panic(errors.Wrap(err, "failed to sign transaction"))
			}
			tx.SetSignature(sig)
		}
	}

	return payloadOnly
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

type Map map[string]interface{}

// EncodeForJS takes a struct and converts
// selected types to values that are compatible in the
// JS environment. It returns a map and will panic
// if obj is not a map/struct.
// Set fieldToIgnore to ignore matching fields
func EncodeForJS(obj interface{}, fieldToIgnore ...string) map[string]interface{} {

	if obj == nil {
		return nil
	}

	var m map[string]interface{}
	structs.DefaultTagName = "json"
	if structs.IsStruct(obj) {
		st := structs.New(obj)
		m = st.Map()
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
		case *big.Int, uint32, int64, uint64:
			m[k] = fmt.Sprintf("%d", o)
		case float64:
			m[k] = fmt.Sprintf("%f", o)
		case map[string]interface{}:
			if len(o) > 0 { // no need adding empty maps
				m[k] = EncodeForJS(o)
			}
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
		case util.PublicKey:
			m[k] = crypto.MustPubKeyFromBytes(o[:]).Base58()

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
				m[k] = EncodeForJS(newMap)
			} else if kind == reflect.Struct {
				m[k] = EncodeForJS(structs.Map(o))
			}
		}
	}

	return m
}

// EncodeManyForJS is like EncodeForJS but accepts a slice of objects
func EncodeManyForJS(objs interface{}, fieldToIgnore ...string) []Map {
	var many []Map

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
