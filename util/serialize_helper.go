package util

import (
	"reflect"
)

type Decoder interface {
	DecodeMulti(v ...interface{}) error
	Skip() error
}

type Encoder interface {
	EncodeMulti(v ...interface{}) error
}

// CodecUtil provides convenient methods to serialize and deserialize objects.
// It is expected to be embedded in a struct to provided code utilities.
type CodecUtil struct {
	// Version is the host object version. When the host object is encoded, this
	// version is added to the encoding before any other data fields.
	Version string

	// versionDecoded indicate that the version of the encoded data was decoded.
	versionDecoded bool

	// DecodeVersion is the version of the encoded data.
	DecodedVersion string
}

func (h *CodecUtil) ResetCodec() {
	h.Version = ""
	h.versionDecoded = false
	h.DecodedVersion = ""
}

// DecodeVersion decodes and returns the encoding's version
func (h *CodecUtil) DecodeVersion(dec Decoder) (string, error) {
	var version string
	if err := dec.DecodeMulti(&version); err != nil {
		return "", err
	}
	h.DecodedVersion = version
	h.versionDecoded = true
	return version, nil
}

// DecodeMulti wraps msgpack.Decoder#DecodeMulti.
// It skips over version information.
// It ignores EOF errors.
func (h *CodecUtil) DecodeMulti(dec Decoder, v ...interface{}) error {

	var err error

	// Skip version only if it has not been decoded
	if !h.versionDecoded {
		_, err = h.DecodeVersion(dec)
		if err != nil {
			return err
		}
	}

	err = dec.DecodeMulti(v...)
	if err != nil {
		if err.Error() != "EOF" {
			return err
		}
	}

	return nil
}

// EncodeMulti is a wraps msgpack.Encoder#EncodeMulti; It normalizes fields and performs
// pre-serialization operations
func (h *CodecUtil) EncodeMulti(enc Encoder, v ...interface{}) error {

	// Encode the version first
	enc.EncodeMulti(h.Version)

	for i, vv := range v {

		value := reflect.ValueOf(vv)
		kind := value.Kind()

		switch kind {
		case reflect.Map:
			// Convert to maps with non-interface value type to  map[string]interface
			if value.Type().Elem().Kind() != reflect.String && value.Type().Elem().Kind() != reflect.Interface {
				v[i] = ToStringMapInter(vv)
			}
		}
	}

	return enc.EncodeMulti(v...)
}
