package util

import (
	"reflect"
)

type Decoder interface {
	DecodeMulti(v ...interface{}) error
}

type Encoder interface {
	EncodeMulti(v ...interface{}) error
}

// SerializerHelper provides convenient methods to serialize and deserialize objects
type SerializerHelper struct{}

// DecodeMulti wraps msgpack.Decoder#DecodeMulti to ignore EOF error
func (h SerializerHelper) DecodeMulti(dec Decoder, v ...interface{}) error {
	err := dec.DecodeMulti(v...)
	if err != nil {
		if err.Error() != "EOF" {
			return err
		}
	}
	return nil
}

// EncodeMulti wraps msgpack.Encoder#EncodeMulti to normalize the encoded values
func (h SerializerHelper) EncodeMulti(enc Encoder, v ...interface{}) error {

	// Normalize map types to map[string]interface{}
	for i, vv := range v {
		if reflect.TypeOf(vv).Kind() != reflect.Map {
			continue
		}
		_, isStrVal := vv.(map[string]string)
		_, isInterVal := vv.(map[string]interface{})
		if !isStrVal && !isInterVal {
			v[i] = ToMapSI(vv)
		}
	}

	return enc.EncodeMulti(v...)
}
