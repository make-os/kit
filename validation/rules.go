package validation

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/util"
	"github.com/themakeos/lobe/util/identifier"
)

var validAddrRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		switch v := val.(type) {
		case util.String:
			if _err := crypto.IsValidUserAddr(v.String()); _err != nil {
				return err
			}
		case string:
			if _err := crypto.IsValidUserAddr(v); _err != nil {
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
		pk := val.(crypto.PublicKey)
		if pk.Equal(crypto.EmptyPublicKey) {
			return err
		}
		if _, _err := crypto.PubKeyFromBytes(pk.Bytes()); _err != nil {
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
		case crypto.PublicKey:
			if o.Equal(crypto.EmptyPublicKey) {
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
