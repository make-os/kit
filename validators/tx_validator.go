package validators

import (
	"fmt"
	"time"

	"github.com/makeos/mosdef/params"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"

	v "github.com/go-ozzo/ozzo-validation"
	"github.com/makeos/mosdef/types"
	// "github.com/go-ozzo/ozzo-validation/is"
)

// ValidateTxFunc represents a function for validating a transaction
type ValidateTxFunc func(tx *types.Transaction, i int, logic types.Logic) error

// KnownTransactionTypes are the supported transaction types
var KnownTransactionTypes = []int{
	types.TxTypeCoinTransfer,
	types.TxTypeTicketValidator,
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
			return types.FieldErrorWithIndex(index, field, "could not convert to decimal")
		}
		if dVal.LessThan(decimal.Zero) {
			return types.FieldErrorWithIndex(index, field, "negative figure not allowed")
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
func ValidateTxs(txs []*types.Transaction, logic types.Logic) error {
	for i, tx := range txs {
		if err := ValidateTx(tx, i, logic); err != nil {
			return err
		}
	}
	return nil
}

// ValidateTx validates a transaction
func ValidateTx(tx *types.Transaction, i int, logic types.Logic) error {

	if tx.Type == types.TxTypeEpochSecret {
		goto checkEpochSecret
	}

	if err := ValidateTxSyntax(tx, i); err != nil {
		return err
	}

	if err := ValidateTxConsistency(tx, i, logic); err != nil {
		return err
	}

	return nil

checkEpochSecret:

	if err := ValidateEpochSecretTx(tx, i, logic); err != nil {
		return err
	}

	if err := ValidateEpochSecretTxConsistency(tx, i, logic); err != nil {
		return err
	}

	return nil
}

// ValidateEpochSecretTx validates TxTypeEpochSecret transaction.
func ValidateEpochSecretTx(tx *types.Transaction, index int, logic types.Logic) error {

	// Secret must be set and must be 64-bytes in length
	if err := v.Validate(tx.GetSecret(),
		v.Required.Error(types.FieldErrorWithIndex(index, "secret",
			"secret is required").Error()), v.By(validSecretRule("secret", index)),
	); err != nil {
		return err
	}

	// Previous secret must be set and must be 64-bytes in length
	if err := v.Validate(tx.GetPreviousSecret(),
		v.Required.Error(types.FieldErrorWithIndex(index, "previousSecret",
			"previous secret is required").Error()), v.By(validSecretRule("previousSecret", index)),
	); err != nil {
		return err
	}

	// Previous secret must be set and must be 64-bytes in length
	if err := v.Validate(tx.GetSecretRound(),
		v.Required.Error(types.FieldErrorWithIndex(index, "secretRound",
			"secret round is required").Error()),
	); err != nil {
		return err
	}

	return nil
}

// ValidateEpochSecretTxConsistency validates TxTypeEpochSecret
// transaction to ensure the drand secret is valid and the round 
// is obtained within an expected window. 
func ValidateEpochSecretTxConsistency(tx *types.Transaction, index int, logic types.Logic) error {

	err := logic.GetDRand().Verify(tx.Secret, tx.PreviousSecret, tx.SecretRound)
	if err != nil {
		return types.FieldErrorWithIndex(index, "secret", "epoch secret is invalid")
	}

	// We need to ensure that the drand round is greater 
	// than the last known highest drand round.
	highestDrandRound, err := logic.SysKeeper().GetHighestDrandRound()
	if err != nil {
		return errors.Wrap(err, "failed to get highest drand round")
	} else if tx.SecretRound <= highestDrandRound {
		return types.ErrStaleSecretRound(index)
	}

	// Get the last committed block
	// bi, err := logic.SysKeeper().GetLastBlockInfo()
	// if err != nil {
	// 	return errors.Wrap(err, "failed to get last committed block")
	// }

	// // Determine the height of the last block of the last epoch,
	// // then fetch the block info
	// curBlockHeight := bi.Height + 1
	// lastEpochBlockHeight := curBlockHeight - int64(params.NumBlocksPerEpoch)
	// lastEpochBlockInfo, err := logic.SysKeeper().GetBlockInfo(lastEpochBlockHeight)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to get last block of last epoch")
	// }

	// Ensure the tx secret round was not generated at
	// an earlier period (before the epoch reaches its last block).
	// minsPerEpoch := uint64(params.NumBlocksPerEpoch / 60)
	// expectedRound := lastEpochBlockInfo.EpochRound + minsPerEpoch
	// pp.Println("Expected", expectedRound, "Actual", tx.SecretRound)
	// if tx.SecretRound < expectedRound {
	// 	pp.Println("Too early")
	// 	return types.FieldErrorWithIndex(index, "secretRound", "round was generated too early")
	// }

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

	// For non ticket purchasing transactions,
	// The recipient's address must be set and it must be valid.
	if tx.Type != types.TxTypeTicketValidator {
		if err := v.Validate(tx.GetTo(),
			v.Required.Error(types.FieldErrorWithIndex(index, "to",
				"recipient address is required").Error()),
			v.By(validAddrRule(types.FieldErrorWithIndex(index, "to",
				"recipient address is not valid"))),
		); err != nil {
			return err
		}
	}

	// Value must be >= 0 and it must be valid number
	if err := v.Validate(tx.GetValue(),
		v.Required.Error(types.FieldErrorWithIndex(index, "value",
			"value is required").Error()), v.By(validValueRule("value", index)),
	); err != nil {
		return err
	}

	// Fee must be >= 0 and it must be valid number
	if err := v.Validate(tx.GetFee(),
		v.Required.Error(types.FieldErrorWithIndex(index, "fee",
			"fee is required").Error()), v.By(validValueRule("fee", index)),
	); err != nil {
		return err
	}

	// Fee must be at least equal to the base fee
	txSize := decimal.NewFromFloat(float64(tx.GetSizeNoFee()))
	baseFee := params.FeePerByte.Mul(txSize)
	if tx.Fee.Decimal().LessThan(baseFee) {
		return types.FieldErrorWithIndex(index, "fee",
			fmt.Sprintf("fee cannot be lower than the base price of %s",
				baseFee.StringFixed(4)))
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

	valid, err := pubKey.Verify(tx.GetBytesNoSig(), tx.GetSignature())
	if err != nil {
		errs = append(errs, types.FieldErrorWithIndex(index, "sig", err.Error()))
	} else if !valid {
		errs = append(errs, types.FieldErrorWithIndex(index, "sig", "signature is not valid"))
	}

	return
}

// ValidateTxConsistency checks whether the transaction includes
// values that are consistent with the current state of the app
func ValidateTxConsistency(tx *types.Transaction, index int, logic types.Logic) error {

	pubKey, err := crypto.PubKeyFromBase58(tx.GetSenderPubKey().String())
	if err != nil {
		return types.FieldErrorWithIndex(index, "senderPubKey", err.Error())
	}

	// Check whether the transaction is consistent with
	// the current state of the sender's account
	err = logic.Tx().CanTransferCoin(tx.Type, pubKey, tx.To, tx.Value, tx.Fee, tx.GetNonce())
	if err != nil {
		return err
	}

	return nil
}
