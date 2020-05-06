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

// EncodeMulti is a wraps msgpack.Encoder#EncodeMulti; It normalizes fields and performs
// pre-serialization operations
func (h SerializerHelper) EncodeMulti(enc Encoder, v ...interface{}) error {

	for i, vv := range v {

		value := reflect.ValueOf(vv)
		kind := value.Kind()

		switch kind {
		case reflect.Map:
			// Convert to map[string]interface if element is not string or interface
			_, isStrVal := vv.(map[string]string)
			_, isInterVal := vv.(map[string]interface{})
			if !isStrVal && !isInterVal {
				v[i] = ToStringMapInter(vv)
			}

		case reflect.Slice:
			// Convert to empty byte slice if element is a nil byte slice
			if value.Type().Elem().Kind() == reflect.Uint8 && value.IsNil() {
				v[i] = []uint8{}
			}
		}
	}

	return enc.EncodeMulti(v...)
}
