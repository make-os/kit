package validation

import (
	"fmt"

	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util/errors"

	"github.com/make-os/kit/crypto/ed25519"
)

var feI = errors.FieldErrorWithIndex

// ValidateTxFunc represents a function for validating a transaction
type ValidateTxFunc func(tx types.BaseTx, i int, logic core.Logic) error

// ValidateTx validates a transaction
func ValidateTx(tx types.BaseTx, i int, logic core.Logic) error {

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
	case *txns.TxCoinTransfer:
		return CheckTxCoinTransfer(o, index)
	case *txns.TxTicketPurchase:
		return CheckTxTicketPurchase(o, index)
	case *txns.TxSetDelegateCommission:
		return CheckTxSetDelegateCommission(o, index)
	case *txns.TxTicketUnbond:
		return CheckTxUnbondTicket(o, index)
	case *txns.TxRepoCreate:
		return CheckTxRepoCreate(o, index)
	case *txns.TxRegisterPushKey:
		return CheckTxRegisterPushKey(o, index)
	case *txns.TxUpDelPushKey:
		return CheckTxUpDelPushKey(o, index)
	case *txns.TxPush:
		return CheckTxPush(o, index)
	case *txns.TxNamespaceRegister:
		return CheckTxNamespaceAcquire(o, index)
	case *txns.TxNamespaceDomainUpdate:
		return CheckTxNamespaceDomainUpdate(o, index)
	case *txns.TxRepoProposalUpsertOwner:
		return CheckTxRepoProposalUpsertOwner(o, index)
	case *txns.TxRepoProposalVote:
		return CheckTxVote(o, index)
	case *txns.TxRepoProposalUpdate:
		return CheckTxRepoProposalUpdate(o, index)
	case *txns.TxRepoProposalSendFee:
		return CheckTxRepoProposalSendFee(o, index)
	case *txns.TxRepoProposalRegisterPushKey:
		return CheckTxRepoProposalRegisterPushKey(o, index)
	case *txns.TxSubmitWork:
		return CheckTxSubmitWork(o, index)
	case *txns.TxBurnGasForCoin:
		return CheckTxBurnGasForCoin(o, index)
	case *txns.TxBurnForSwap:
		return CheckTxBurnForSwap(o, index)
	default:
		return feI(index, "type", "unsupported transaction type")
	}
}

// ValidateTxConsistency checks whether the transaction includes
// values that are consistent with the current state of the app
//
// CONTRACT: Sender public key must be validated by the caller.
func ValidateTxConsistency(tx types.BaseTx, index int, logic core.Logic) error {
	switch o := tx.(type) {
	case *txns.TxCoinTransfer:
		return CheckTxCoinTransferConsistency(o, index, logic)
	case *txns.TxTicketPurchase:
		return CheckTxTicketPurchaseConsistency(o, index, logic)
	case *txns.TxSetDelegateCommission:
		return CheckTxSetDelegateCommissionConsistency(o, index, logic)
	case *txns.TxTicketUnbond:
		return CheckTxUnbondTicketConsistency(o, index, logic)
	case *txns.TxRepoCreate:
		return CheckTxRepoCreateConsistency(o, index, logic)
	case *txns.TxRegisterPushKey:
		return CheckTxRegisterPushKeyConsistency(o, index, logic)
	case *txns.TxUpDelPushKey:
		return CheckTxUpDelPushKeyConsistency(o, index, logic)
	case *txns.TxPush:
		return CheckTxPushConsistency(o, index, logic)
	case *txns.TxNamespaceRegister:
		return CheckTxNSAcquireConsistency(o, index, logic)
	case *txns.TxNamespaceDomainUpdate:
		return CheckTxNamespaceDomainUpdateConsistency(o, index, logic)
	case *txns.TxRepoProposalUpsertOwner:
		return CheckTxRepoProposalUpsertOwnerConsistency(o, index, logic)
	case *txns.TxRepoProposalVote:
		return CheckTxVoteConsistency(o, index, logic)
	case *txns.TxRepoProposalUpdate:
		return CheckTxRepoProposalUpdateConsistency(o, index, logic)
	case *txns.TxRepoProposalSendFee:
		return CheckTxRepoProposalSendFeeConsistency(o, index, logic)
	case *txns.TxRepoProposalRegisterPushKey:
		return CheckTxRepoProposalRegisterPushKeyConsistency(o, index, logic)
	case *txns.TxSubmitWork:
		return CheckTxSubmitWorkConsistency(o, index, logic)
	case *txns.TxBurnGasForCoin:
		return CheckTxBurnGasForCoinConsistency(o, index, logic)
	case *txns.TxBurnForSwap:
		return CheckTxBurnForSwapConsistency(o, index, logic)
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
	pubKey, _ := ed25519.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	valid, err := pubKey.Verify(tx.GetBytesNoSig(), tx.GetSignature())
	if err != nil {
		errs = append(errs, feI(index, "sig", err.Error()))
	} else if !valid {
		errs = append(errs, feI(index, "sig", "signature is not valid"))
	}

	return
}
