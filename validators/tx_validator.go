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
	types.TxTypeValidatorTicket,
	types.TxTypeSetDelegatorCommission,
	types.TxTypeStorerTicket,
	types.TxTypeUnbondStorerTicket,
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
			return types.FieldErrorWithIndex(index, field, "invalid number; must be numeric")
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

	switch tx.Type {
	case types.TxTypeEpochSecret:
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

	// Check for unexpected fields
	if err := CheckUnexpectedFields(tx, index); err != nil {
		return err
	}

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

	// Ensure the tx secret round was not generated at
	// an earlier period (before the epoch reaches its last block).
	minsPerEpoch := (uint64(params.NumBlocksPerEpoch * params.BlockTime)) / 60
	expectedRound := highestDrandRound + minsPerEpoch
	if tx.SecretRound < expectedRound {
		return types.ErrEarlySecretRound(index)
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

	// Check for unexpected fields
	if err := CheckUnexpectedFields(tx, index); err != nil {
		return err
	}

	// The recipient's address must be set and it must be valid.
	// Ignore for: TxTypeValidatorTicket, TxTypeSetDelegatorCommission,
	// TxTypeStorerTicket, TxTypeUnbondStorerTicket
	if tx.Type != types.TxTypeValidatorTicket &&
		tx.Type != types.TxTypeSetDelegatorCommission &&
		tx.Type != types.TxTypeStorerTicket &&
		tx.Type != types.TxTypeUnbondStorerTicket {
		if err := v.Validate(tx.GetTo(),
			v.Required.Error(types.FieldErrorWithIndex(index, "to",
				"recipient address is required").Error()),
			v.By(validAddrRule(types.FieldErrorWithIndex(index, "to",
				"recipient address is not valid"))),
		); err != nil {
			return err
		}
	} else {
		// For validator or storer ticket purchasing transactions, the
		// recipient's address must be a valid public key if it is set
		if !tx.To.Empty() && (tx.Type == types.TxTypeValidatorTicket ||
			tx.Type == types.TxTypeStorerTicket) {
			if err := v.Validate(tx.To, v.By(validPubKeyRule(types.FieldErrorWithIndex(index, "to",
				"requires a valid public key to delegate to")))); err != nil {
				return err
			}
		}
	}

	// Value must be >= 0 and it must be valid number.
	// Ignore for: TxTypeUnbondStorerTicket
	if tx.Type != types.TxTypeUnbondStorerTicket {
		if err := v.Validate(tx.GetValue(),
			v.Required.Error(types.FieldErrorWithIndex(index, "value",
				"value is required").Error()), v.By(validValueRule("value", index)),
		); err != nil {
			return err
		}
	}

	// Value cannot be zero or less than the minimum commission
	// Only for: TxTypeSetDelegatorCommission
	if tx.Type == types.TxTypeSetDelegatorCommission {
		if tx.Value.Decimal().LessThan(params.MinDelegatorCommission) {
			minPct := params.MinDelegatorCommission.String()
			return types.FieldErrorWithIndex(index, "value",
				"commission rate cannot be below the minimum ("+minPct+"%%%%)")
		}

		// Value cannot be greater than 100
		if tx.Value.Decimal().GreaterThan(decimal.NewFromFloat(100)) {
			return types.FieldErrorWithIndex(index, "value", "commission rate cannot exceed 100%%%%")
		}
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

	// Ticket ID is required
	// Only for: TxTypeUnbondStorerTicket
	if tx.Type == types.TxTypeUnbondStorerTicket {
		if err := v.Validate(tx.GetTicketID(),
			v.Required.Error(types.FieldErrorWithIndex(index, "ticket",
				"ticket id is required").Error()),
		); err != nil {
			return err
		}
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

// IsSet checks whether a value has been set.
// Note: This function only checks types included in a transaction.
// Therefore, it may not be appropriate for general usage.
func IsSet(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return false
	case map[string]interface{}:
		return len(v) > 0
	case int:
		return v != 0
	case uint:
		return v != 0
	case int64:
		return v != 0
	case uint64:
		return v != 0
	case string:
		return len(v) > 0
	case util.String:
		return v != "0" && v != ""
	case []byte:
		return len(v) > 0
	default:
		return false
	}
}

// CheckUnexpectedFields checks whether unexpected fields for
// various tx type remain unset or have zero values.
func CheckUnexpectedFields(tx *types.Transaction, index int) error {
	txType := tx.GetType()

	// Generally, `meta` field is not expected for any tx type
	unExpected := [][]interface{}{
		{"meta", tx.GetMeta()},
	}

	// Check for unexpected fields for TxTypeValidatorTicket and TxTypeCoinTransfer
	if txType == types.TxTypeValidatorTicket || txType == types.TxTypeCoinTransfer {
		unExpected = append(unExpected, []interface{}{"secret", tx.Secret})
		unExpected = append(unExpected, []interface{}{"previousSecret", tx.PreviousSecret})
		unExpected = append(unExpected, []interface{}{"secretRound", tx.SecretRound})
		unExpected = append(unExpected, []interface{}{"ticketID", tx.TicketID})
		for _, item := range unExpected {
			if IsSet(item[1]) {
				return types.FieldErrorWithIndex(index, item[0].(string), "unexpected field")
			}
		}
	}

	// Check for unexpected field for types.TxTypeEpochSecret,
	if txType == types.TxTypeEpochSecret {
		unExpected = append(unExpected, []interface{}{"nonce", tx.Nonce})
		unExpected = append(unExpected, []interface{}{"to", tx.To})
		unExpected = append(unExpected, []interface{}{"senderPubKey", tx.SenderPubKey})
		unExpected = append(unExpected, []interface{}{"value", tx.Value})
		unExpected = append(unExpected, []interface{}{"timestamp", tx.Timestamp})
		unExpected = append(unExpected, []interface{}{"fee", tx.Fee})
		unExpected = append(unExpected, []interface{}{"sig", tx.Sig})
		unExpected = append(unExpected, []interface{}{"ticketID", tx.TicketID})
		for _, item := range unExpected {
			if IsSet(item[1]) {
				return types.FieldErrorWithIndex(index, item[0].(string), "unexpected field")
			}
		}
	}

	// Check for unexpected field for TxTypeSetDelegatorCommission & TxTypeStorerTicket
	if txType == types.TxTypeSetDelegatorCommission || txType == types.TxTypeStorerTicket {
		unExpected = append(unExpected, []interface{}{"secret", tx.Secret})
		unExpected = append(unExpected, []interface{}{"previousSecret", tx.PreviousSecret})
		unExpected = append(unExpected, []interface{}{"secretRound", tx.SecretRound})
		unExpected = append(unExpected, []interface{}{"ticketID", tx.TicketID})

		// Allow `to` field for TxTypeStorerTicket
		if txType != types.TxTypeStorerTicket {
			unExpected = append(unExpected, []interface{}{"to", tx.To})
		}

		for _, item := range unExpected {
			if IsSet(item[1]) {
				return types.FieldErrorWithIndex(index, item[0].(string), "unexpected field")
			}
		}
	}

	// Check for unexpected field for types.TxTypeUnbondStorerTicket,
	if txType == types.TxTypeUnbondStorerTicket {
		unExpected = append(unExpected, []interface{}{"to", tx.To})
		unExpected = append(unExpected, []interface{}{"value", tx.Value})
		unExpected = append(unExpected, []interface{}{"secret", tx.Secret})
		unExpected = append(unExpected, []interface{}{"previousSecret", tx.PreviousSecret})
		unExpected = append(unExpected, []interface{}{"secretRound", tx.SecretRound})
		for _, item := range unExpected {
			if IsSet(item[1]) {
				return types.FieldErrorWithIndex(index, item[0].(string), "unexpected field")
			}
		}
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

	// Get current block height
	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	switch tx.Type {
	case types.TxTypeSetDelegatorCommission:
		return nil
	case types.TxTypeUnbondStorerTicket:
		goto unbondStoreTicket
	}

	// For delegated ticket transactions, ensure the delegate has an active,
	// non-delegated ticket
	if (tx.Type == types.TxTypeStorerTicket || tx.Type == types.TxTypeValidatorTicket) &&
		!tx.To.Empty() {
		r, err := logic.GetTicketManager().GetActiveTicketsByProposer(tx.To.String(), tx.Type, false)
		if err != nil {
			return errors.Wrap(err, "failed to get active delegate tickets")
		} else if len(r) == 0 {
			return types.FieldErrorWithIndex(index, "to", "the delegate is not active")
		}
	}

	// Check whether the transaction is consistent with
	// the current state of the sender's account
	err = logic.Tx().CanExecCoinTransfer(tx.Type, pubKey, tx.To, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height))
	if err != nil {
		return err
	}

	return nil

unbondStoreTicket:

	// Ticket ID must be a known ticket
	ticket := logic.GetTicketManager().GetByHash(string(tx.TicketID))
	if ticket == nil {
		return types.FieldErrorWithIndex(index, "ticketID", "ticket not found")
	}

	// Ensure the tx sender is the owner of the ticket.
	// For delegated ticket, compare the delegator address with the sender
	// address
	authErr := types.FieldErrorWithIndex(index, "ticketID", "sender not authorized to "+
		"unbond this ticket")
	if ticket.Delegator == "" {
		if !tx.SenderPubKey.Equal(util.String(ticket.ProposerPubKey)) {
			return authErr
		}
	} else if ticket.Delegator != tx.GetFrom().String() {
		return authErr
	}

	// Ensure the ticket has not decayed or is decaying.
	decayBy := ticket.DecayBy
	if decayBy != 0 && decayBy > uint64(bi.Height) {
		return types.FieldErrorWithIndex(index, "ticketID", "ticket is already decaying")
	} else if decayBy != 0 && decayBy <= uint64(bi.Height) {
		return types.FieldErrorWithIndex(index, "ticketID", "ticket has already decayed")
	}

	return nil
}
