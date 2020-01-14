package jsmodules

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/thoas/go-funk"
)

// type castPanick

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
