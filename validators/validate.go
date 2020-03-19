package validators

import (
	"fmt"
	"path/filepath"

	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/repo"
)

var feI = util.FieldErrorWithIndex

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
	case *core.TxCoinTransfer:
		return CheckTxCoinTransfer(o, index)
	case *core.TxTicketPurchase:
		return CheckTxTicketPurchase(o, index)
	case *core.TxSetDelegateCommission:
		return CheckTxSetDelegateCommission(o, index)
	case *core.TxTicketUnbond:
		return CheckTxUnbondTicket(o, index)
	case *core.TxRepoCreate:
		return CheckTxRepoCreate(o, index)
	case *core.TxRegisterPushKey:
		return CheckTxRegisterPushKey(o, index)
	case *core.TxUpDelGPGPubKey:
		return CheckTxUpDelGPGPubKey(o, index)
	case *core.TxPush:
		return CheckTxPush(o, index)
	case *core.TxNamespaceAcquire:
		return CheckTxNSAcquire(o, index)
	case *core.TxNamespaceDomainUpdate:
		return CheckTxNamespaceDomainUpdate(o, index)
	case *core.TxRepoProposalUpsertOwner:
		return CheckTxRepoProposalUpsertOwner(o, index)
	case *core.TxRepoProposalVote:
		return CheckTxVote(o, index)
	case *core.TxRepoProposalUpdate:
		return CheckTxRepoProposalUpdate(o, index)
	case *core.TxRepoProposalSendFee:
		return CheckTxRepoProposalSendFee(o, index)
	case *core.TxRepoProposalMergeRequest:
		return CheckTxRepoProposalMergeRequest(o, index)
	case *core.TxRepoProposalRegisterPushKey:
		return CheckTxRepoProposalRegisterGPGKey(o, index)
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
	case *core.TxCoinTransfer:
		return CheckTxCoinTransferConsistency(o, index, logic)
	case *core.TxTicketPurchase:
		return CheckTxTicketPurchaseConsistency(o, index, logic)
	case *core.TxSetDelegateCommission:
		return CheckTxSetDelegateCommissionConsistency(o, index, logic)
	case *core.TxTicketUnbond:
		return CheckTxUnbondTicketConsistency(o, index, logic)
	case *core.TxRepoCreate:
		return CheckTxRepoCreateConsistency(o, index, logic)
	case *core.TxRegisterPushKey:
		return CheckTxRegisterPushKeyConsistency(o, index, logic)
	case *core.TxUpDelGPGPubKey:
		return CheckTxUpDelGPGPubKeyConsistency(o, index, logic)
	case *core.TxPush:
		return CheckTxPushConsistency(o, index, logic, func(name string) (core.BareRepo, error) {
			return repo.GetRepo(filepath.Join(logic.Cfg().GetRepoRoot(), name))
		})
	case *core.TxNamespaceAcquire:
		return CheckTxNSAcquireConsistency(o, index, logic)
	case *core.TxNamespaceDomainUpdate:
		return CheckTxNamespaceDomainUpdateConsistency(o, index, logic)
	case *core.TxRepoProposalUpsertOwner:
		return CheckTxRepoProposalUpsertOwnerConsistency(o, index, logic)
	case *core.TxRepoProposalVote:
		return CheckTxVoteConsistency(o, index, logic)
	case *core.TxRepoProposalUpdate:
		return CheckTxRepoProposalUpdateConsistency(o, index, logic)
	case *core.TxRepoProposalSendFee:
		return CheckTxRepoProposalSendFeeConsistency(o, index, logic)
	case *core.TxRepoProposalMergeRequest:
		return CheckTxRepoProposalMergeRequestConsistency(o, index, logic)
	case *core.TxRepoProposalRegisterPushKey:
		return CheckTxRepoProposalRegisterGPGKeyConsistency(o, index, logic)
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
