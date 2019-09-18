package validators

import (
	"fmt"
	"time"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"

	v "github.com/go-ozzo/ozzo-validation"
	"github.com/makeos/mosdef/types"
	// "github.com/go-ozzo/ozzo-validation/is"
)

// KnownTransactionTypes are the supported transaction types
var KnownTransactionTypes = []int{
	types.TxTypeCoin,
}

var validTypeRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		if !funk.ContainsInt(KnownTransactionTypes, val.(int)) {
			return err
		}
		return nil
	}
}

var validAddrRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		if _err := crypto.IsValidAddr(val.(util.String).String()); _err != nil {
			return err
		}
		return nil
	}
}

var isDerivedFromPublicKeyRule = func(tx *types.Transaction, err error) func(interface{}) error {
	return func(val interface{}) error {
		pk, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey().String())
		if !pk.Addr().Equal(val.(util.String)) {
			return err
		}
		return nil
	}
}

var validPubKeyRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		if _, _err := crypto.PubKeyFromBase58(val.(util.String).String()); _err != nil {
			return err
		}
		return nil
	}
}

var requireHashRule = func(err error) func(interface{}) error {
	return func(val interface{}) error {
		if val.(util.Hash).IsEmpty() {
			return err
		}
		return nil
	}
}

var validValueRule = func(field string, index int) func(interface{}) error {
	return func(val interface{}) error {
		dVal, _err := decimal.NewFromString(val.(util.String).String())
		if _err != nil {
			return types.FieldErrorWithIndex(index, field, "could not convert to decimal")
		}
		if dVal.LessThan(decimal.Zero) {
			return types.FieldErrorWithIndex(index, field, "negative value not allowed")
		}
		return nil
	}
}

var isSameHashRule = func(val2 util.Hash, err error) func(interface{}) error {
	return func(val interface{}) error {
		if !val.(util.Hash).Equal(val2) {
			return err
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

// ValidateTxs performs both syntactic and consistency
// validation on the given transactions.
func ValidateTxs(txs []*types.Transaction) error {
	for i, tx := range txs {
		if err := ValidateTx(tx, i); err != nil {
			return err
		}
	}
	return nil
}

// ValidateTx validates a transaction
func ValidateTx(tx *types.Transaction, i int) error {

	// Perform syntactic checks
	if err := ValidateTxSyntax(tx, i); err != nil {
		return err
	}

	// Perform consistency check
	if err := ValidateTxConsistency(tx, i); err != nil {
		return err
	}

	return nil
}

// ValidateTxSyntax checks whether the transaction's
// fields and values are expected and correct.
// The argument index is used to describe the position in
// the slice this transaction was accessed when constructing
// error messages; Use -1 if tx is not part of a collection.
func ValidateTxSyntax(tx *types.Transaction, index int) error {

	// Transaction must not be nil
	if tx == nil {
		return fmt.Errorf("nil tx")
	}

	// Transaction type is required and must match the known types
	if err := v.Validate(tx.GetType(), v.By(validTypeRule(types.FieldErrorWithIndex(index, "type",
		"unsupported transaction type"))),
	); err != nil {
		return err
	}

	// Recipient's address must be set and it must be valid
	if err := v.Validate(tx.GetTo(),
		v.Required.Error(types.FieldErrorWithIndex(index, "to",
			"recipient address is required").Error()),
		v.By(validAddrRule(types.FieldErrorWithIndex(index, "to",
			"recipient address is not valid"))),
	); err != nil {
		return err
	}

	// Value must be >= 0 and it must be valid number
	if err := v.Validate(tx.GetValue(),
		v.Required.Error(types.FieldErrorWithIndex(index, "value",
			"value is required").Error()), v.By(validValueRule("value", index)),
	); err != nil {
		return err
	}

	// Timestamp is required.
	if err := v.Validate(tx.GetTimestamp(),
		v.Required.Error(types.FieldErrorWithIndex(index, "timestamp",
			"timestamp is required").Error()), v.By(validTimestampRule("timestamp", index)),
	); err != nil {
		return err
	}

	// Sender's public key is required and must be a valid base58 encoded key
	if err := v.Validate(tx.GetSenderPubKey(),
		v.Required.Error(types.FieldErrorWithIndex(index, "senderPubKey",
			"sender public key is required").Error()),
		v.By(validPubKeyRule(types.FieldErrorWithIndex(index, "senderPubKey",
			"sender public key is not valid"))),
	); err != nil {
		return err
	}

	// Hash is required. It must also be correct
	if err := v.Validate(tx.GetHash(),
		v.By(requireHashRule(types.FieldErrorWithIndex(index, "hash",
			"hash is required"))),
		v.By(isSameHashRule(tx.ComputeHash(), types.FieldErrorWithIndex(index,
			"hash", "hash is not correct"))),
	); err != nil {
		return err
	}

	// Signature must be set
	if err := v.Validate(tx.GetSignature(),
		v.Required.Error(types.FieldErrorWithIndex(index, "sig",
			"signature is required").Error()),
	); err != nil {
		return err
	}

	// Check signature correctness
	if sigErr := checkSignature(tx, index); len(sigErr) > 0 {
		return sigErr[0]
	}

	return nil
}

// checkSignature checks whether the signature is valid.
// Expects the transaction to have a valid sender public key.
// The argument index is used to describe the position in
// the slice this transaction was accessed when constructing
// error messages; Use -1 if tx is not part of a collection.
func checkSignature(tx *types.Transaction, index int) (errs []error) {

	pubKey, err := crypto.PubKeyFromBase58(tx.GetSenderPubKey().String())
	if err != nil {
		errs = append(errs, types.FieldErrorWithIndex(index,
			"senderPubKey", err.Error()))
		return
	}

	valid, err := pubKey.Verify(tx.GetBytesNoHashAndSig(), tx.GetSignature())
	if err != nil {
		errs = append(errs, types.FieldErrorWithIndex(index, "sig", err.Error()))
	} else if !valid {
		errs = append(errs, types.FieldErrorWithIndex(index, "sig", "signature is not valid"))
	}

	return
}

// ValidateTxConsistency checks whether the transaction includes
// values that are consistent with the current state of the app
func ValidateTxConsistency(tx *types.Transaction, index int) error {
	return nil
}
