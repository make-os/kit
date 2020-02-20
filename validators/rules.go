package validators

import (
	"gitlab.com/makeos/mosdef/types/msgs"
	"time"

	govalidator "github.com/asaskevich/govalidator"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
)

var validAddrRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		switch v := val.(type) {
		case util.String:
			if _err := crypto.IsValidAddr(v.String()); _err != nil {
				return err
			}
		case string:
			if _err := crypto.IsValidAddr(v); _err != nil {
				return err
			}
		default:
			panic("unknown type")
		}
		return nil
	}
}

var isDerivedFromPublicKeyRule = func(tx msgs.BaseTx, err error) func(interface{}) error {
	return func(val interface{}) error {
		pk, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
		if !pk.Addr().Equal(val.(util.String)) {
			return err
		}
		return nil
	}
}

var validPubKeyRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		pk := val.(util.PublicKey)
		if pk.Equal(util.EmptyPublicKey) {
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
		case util.PublicKey:
			if o.Equal(util.EmptyPublicKey) {
				return err
			}
		}
		return nil
	}
}

var isEmptyByte64 = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		if val.(util.Bytes64).Equal(util.EmptyBytes64) {
			return err
		}
		return nil
	}
}

var validSecretRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		if len(val.([]byte)) != 64 {
			return util.FieldErrorWithIndex(index, field, "invalid length; expected 64 bytes")
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
		if !govalidator.Matches(name, "^[a-zA-Z0-9_-]+$") {
			msg := "invalid characters in name. Only alphanumeric, _ and - characters are allowed"
			return util.FieldErrorWithIndex(index, field, msg)
		} else if len(name) > 128 {
			msg := "name is too long. Maximum character length is 128"
			return util.FieldErrorWithIndex(index, field, msg)
		} else if len(name) <= 2 {
			msg := "name is too short. Must be at least 3 characters long"
			return util.FieldErrorWithIndex(index, field, msg)
		}
		return nil
	}
}

var validGPGPubKeyRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		pubKey := val.(string)
		if _, err := crypto.PGPEntityFromPubKey(pubKey); err != nil {
			return util.FieldErrorWithIndex(index, field, "invalid gpg public key")
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
