package validators

import (
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
)

var feI = types.FieldErrorWithIndex

// ValidateTxFunc represents a function for validating a transaction
type ValidateTxFunc func(tx types.BaseTx, i int, logic types.Logic) error

// ValidateTxs performs both syntactic and consistency
// validation on the given transactions.
func ValidateTxs(txs []types.BaseTx, logic types.Logic) error {
	for i, tx := range txs {
		if err := ValidateTx(tx, i, logic); err != nil {
			return err
		}
	}
	return nil
}

// ValidateTx validates a transaction
func ValidateTx(tx types.BaseTx, i int, logic types.Logic) error {

	if tx == nil {
		return fmt.Errorf("nil tx")
	}

	if err := ValidateTxSanity(tx, i); err != nil {
		return err
	}

	if err := ValidateTxConsistency(tx, i, logic); err != nil {
		return err
	}

	return nil
}

// ValidateTxSanity checks whether the transaction's fields and values are
// correct without checking any storage.
//
// index: index is used to indicate the index of the transaction in a slice
// managed by the caller. It is used for constructing error messages.
// Use -1 if tx is not part of a collection.
func ValidateTxSanity(tx types.BaseTx, index int) error {
	switch o := tx.(type) {
	case *types.TxCoinTransfer:
		return CheckTxCoinTransfer(o, index)
	case *types.TxTicketPurchase:
		return CheckTxTicketPurchase(o, index)
	case *types.TxSetDelegateCommission:
		return CheckTxSetDelegateCommission(o, index)
	case *types.TxTicketUnbond:
		return CheckTxUnbondTicket(o, index)
	case *types.TxRepoCreate:
		return CheckTxRepoCreate(o, index)
	case *types.TxEpochSecret:
		return CheckTxEpochSecret(o, index)
	case *types.TxAddGPGPubKey:
		return CheckTxAddGPGPubKey(o, index)
	case *types.TxPush:
		return CheckTxPush(o, index)
	case *types.TxNamespaceAcquire:
		return CheckTxNSPurchase(o, index)
	default:
		return feI(index, "type", "unsupported transaction type")
	}
}

// ValidateTxConsistency checks whether the transaction includes
// values that are consistent with the current state of the app
//
// CONTRACT: Sender public key must be validated by the caller.
func ValidateTxConsistency(tx types.BaseTx, index int, logic types.Logic) error {
	switch o := tx.(type) {
	case *types.TxCoinTransfer:
		return CheckTxCoinTransferConsistency(o, index, logic)
	case *types.TxTicketPurchase:
		return CheckTxTicketPurchaseConsistency(o, index, logic)
	case *types.TxSetDelegateCommission:
		return CheckTxSetDelegateCommissionConsistency(o, index, logic)
	case *types.TxTicketUnbond:
		return CheckTxUnbondTicketConsistency(o, index, logic)
	case *types.TxRepoCreate:
		return CheckTxRepoCreateConsistency(o, index, logic)
	case *types.TxEpochSecret:
		return CheckTxEpochSecretConsistency(o, index, logic)
	case *types.TxAddGPGPubKey:
		return CheckTxAddGPGPubKeyConsistency(o, index, logic)
	case *types.TxPush:
		return CheckTxPushConsistency(o, index, logic)
	case *types.TxNamespaceAcquire:
		return CheckTxNSAcquireConsistency(o, index, logic)
	default:
		return feI(index, "type", "unsupported transaction type")
	}
}

// checkSignature checks whether the signature is valid.
// Expects the transaction to have a valid sender public key.
// The argument index is used to describe the position in
// the slice this transaction was accessed when constructing
// error messages; Use -1 if tx is not part of a collection.
//
// CONTRACT: Sender public key must be validated by the caller.
func checkSignature(tx types.BaseTx, index int) (errs []error) {
	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	valid, err := pubKey.Verify(tx.GetBytesNoSig(), tx.GetSignature())
	if err != nil {
		errs = append(errs, feI(index, "sig", err.Error()))
	} else if !valid {
		errs = append(errs, feI(index, "sig", "signature is not valid"))
	}

	return
}
