package validators

import (
	"time"

	govalidator "github.com/asaskevich/govalidator"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
)

var validAddrRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		if _err := crypto.IsValidAddr(val.(util.String).String()); _err != nil {
			return err
		}
		return nil
	}
}

var isDerivedFromPublicKeyRule = func(tx types.BaseTx, err error) func(interface{}) error {
	return func(val interface{}) error {
		pk, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey())
		if !pk.Addr().Equal(val.(util.String)) {
			return err
		}
		return nil
	}
}

var validPubKeyRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		if _, _err := crypto.PubKeyFromBase58(val.(string)); _err != nil {
			return err
		}
		return nil
	}
}

var validSecretRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		if len(val.([]byte)) != 64 {
			return types.FieldErrorWithIndex(index, field, "invalid length; expected 64 bytes")
		}
		return nil
	}
}

var validValueRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		dVal, _err := decimal.NewFromString(val.(util.String).String())
		if _err != nil {
			return types.FieldErrorWithIndex(index, field, "invalid number; must be numeric")
		}
		if dVal.LessThan(decimal.Zero) {
			return types.FieldErrorWithIndex(index, field, "negative figure not allowed")
		}
		return nil
	}
}

var validRepoNameRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		name := val.(string)
		if !govalidator.Matches(name, "^[a-zA-Z0-9_-]+$") {
			msg := "invalid characters in name. Only alphanumeric, _ and - characters are allowed"
			return types.FieldErrorWithIndex(index, field, msg)
		} else if len(name) > 128 {
			msg := "name is too long. Maximum character length is 128"
			return types.FieldErrorWithIndex(index, field, msg)
		}
		return nil
	}
}

var validGPGPubKeyRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		pubKey := val.(string)
		if _, err := crypto.PGPEntityFromPubKey(pubKey); err != nil {
			return types.FieldErrorWithIndex(index, field, "invalid gpg public key")
		}
		return nil
	}
}

var validTimestampRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		if time.Unix(val.(int64), 0).After(time.Now()) {
			return types.FieldErrorWithIndex(index, field, "timestamp cannot be a future time")
		}
		return nil
	}
}
