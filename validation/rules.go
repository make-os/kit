package validation

import (
	"time"

	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	"github.com/shopspring/decimal"
)

var validAddrRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		switch v := val.(type) {
		case util.String:
			if _err := ed25519.IsValidUserAddr(v.String()); _err != nil {
				return err
			}
		case string:
			if _err := ed25519.IsValidUserAddr(v); _err != nil {
				return err
			}
		default:
			panic("unknown type")
		}
		return nil
	}
}

var validPubKeyRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		pk := val.(ed25519.PublicKey)
		if pk.Equal(ed25519.EmptyPublicKey) {
			return err
		}
		if _, _err := ed25519.PubKeyFromBytes(pk.Bytes()); _err != nil {
			return err
		}
		return nil
	}
}

var isEmptyByte32 = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		switch o := val.(type) {
		case util.Bytes32:
			if o.Equal(util.EmptyBytes32) {
				return err
			}
		case ed25519.PublicKey:
			if o.Equal(ed25519.EmptyPublicKey) {
				return err
			}
		}
		return nil
	}
}

var validValueRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		dVal, _err := decimal.NewFromString(val.(util.String).String())
		if _err != nil {
			return util.FieldErrorWithIndex(index, field, "invalid number; must be numeric")
		}
		if dVal.LessThan(decimal.Zero) {
			return util.FieldErrorWithIndex(index, field, "negative figure not allowed")
		}
		return nil
	}
}

var validObjectNameRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		name := val.(string)
		err := identifier.IsValidResourceName(name)
		if err != nil {
			return util.FieldErrorWithIndex(index, field, err.Error())
		}
		return nil
	}
}

var validTimestampRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		if time.Unix(val.(int64), 0).After(time.Now()) {
			return util.FieldErrorWithIndex(index, field, "timestamp cannot be a future time")
		}
		return nil
	}
}
