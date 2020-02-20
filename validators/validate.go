package validators

import (
	"fmt"
	types3 "gitlab.com/makeos/mosdef/logic/types"
	"gitlab.com/makeos/mosdef/repo/types/core"
	"gitlab.com/makeos/mosdef/types/msgs"
	"gitlab.com/makeos/mosdef/util"
	"path/filepath"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/repo"
)

var feI = util.FieldErrorWithIndex

// ValidateTxFunc represents a function for validating a transaction
type ValidateTxFunc func(tx msgs.BaseTx, i int, logic types3.Logic) error

// ValidateTxs performs both syntactic and consistency
// validation on the given transactions.
func ValidateTxs(txs []msgs.BaseTx, logic types3.Logic) error {
	for i, tx := range txs {
		if err := ValidateTx(tx, i, logic); err != nil {
			return err
		}
	}
	return nil
}

// ValidateTx validates a transaction
func ValidateTx(tx msgs.BaseTx, i int, logic types3.Logic) error {

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
func ValidateTxSanity(tx msgs.BaseTx, index int) error {
	switch o := tx.(type) {
	case *msgs.TxCoinTransfer:
		return CheckTxCoinTransfer(o, index)
	case *msgs.TxTicketPurchase:
		return CheckTxTicketPurchase(o, index)
	case *msgs.TxSetDelegateCommission:
		return CheckTxSetDelegateCommission(o, index)
	case *msgs.TxTicketUnbond:
		return CheckTxUnbondTicket(o, index)
	case *msgs.TxRepoCreate:
		return CheckTxRepoCreate(o, index)
	case *msgs.TxAddGPGPubKey:
		return CheckTxAddGPGPubKey(o, index)
	case *msgs.TxPush:
		return CheckTxPush(o, index)
	case *msgs.TxNamespaceAcquire:
		return CheckTxNSAcquire(o, index)
	case *msgs.TxNamespaceDomainUpdate:
		return CheckTxNamespaceDomainUpdate(o, index)
	case *msgs.TxRepoProposalUpsertOwner:
		return CheckTxRepoProposalUpsertOwner(o, index)
	case *msgs.TxRepoProposalVote:
		return CheckTxVote(o, index)
	case *msgs.TxRepoProposalUpdate:
		return CheckTxRepoProposalUpdate(o, index)
	case *msgs.TxRepoProposalFeeSend:
		return CheckTxRepoProposalSendFee(o, index)
	case *msgs.TxRepoProposalMergeRequest:
		return CheckTxRepoProposalMergeRequest(o, index)
	default:
		return feI(index, "type", "unsupported transaction type")
	}
}

// ValidateTxConsistency checks whether the transaction includes
// values that are consistent with the current state of the app
//
// CONTRACT: Sender public key must be validated by the caller.
func ValidateTxConsistency(tx msgs.BaseTx, index int, logic types3.Logic) error {
	switch o := tx.(type) {
	case *msgs.TxCoinTransfer:
		return CheckTxCoinTransferConsistency(o, index, logic)
	case *msgs.TxTicketPurchase:
		return CheckTxTicketPurchaseConsistency(o, index, logic)
	case *msgs.TxSetDelegateCommission:
		return CheckTxSetDelegateCommissionConsistency(o, index, logic)
	case *msgs.TxTicketUnbond:
		return CheckTxUnbondTicketConsistency(o, index, logic)
	case *msgs.TxRepoCreate:
		return CheckTxRepoCreateConsistency(o, index, logic)
	case *msgs.TxAddGPGPubKey:
		return CheckTxAddGPGPubKeyConsistency(o, index, logic)
	case *msgs.TxPush:
		return CheckTxPushConsistency(o, index, logic, func(name string) (core.BareRepo, error) {
			return repo.GetRepo(filepath.Join(logic.Cfg().GetRepoRoot(), name))
		})
	case *msgs.TxNamespaceAcquire:
		return CheckTxNSAcquireConsistency(o, index, logic)
	case *msgs.TxNamespaceDomainUpdate:
		return CheckTxNamespaceDomainUpdateConsistency(o, index, logic)
	case *msgs.TxRepoProposalUpsertOwner:
		return CheckTxRepoProposalUpsertOwnerConsistency(o, index, logic)
	case *msgs.TxRepoProposalVote:
		return CheckTxVoteConsistency(o, index, logic)
	case *msgs.TxRepoProposalUpdate:
		return CheckTxRepoProposalUpdateConsistency(o, index, logic)
	case *msgs.TxRepoProposalFeeSend:
		return CheckTxRepoProposalSendFeeConsistency(o, index, logic)
	case *msgs.TxRepoProposalMergeRequest:
		return CheckTxRepoProposalMergeRequestConsistency(o, index, logic)
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
func checkSignature(tx msgs.BaseTx, index int) (errs []error) {
	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	valid, err := pubKey.Verify(tx.GetBytesNoSig(), tx.GetSignature())
	if err != nil {
		errs = append(errs, feI(index, "sig", err.Error()))
	} else if !valid {
		errs = append(errs, feI(index, "sig", "signature is not valid"))
	}

	return
}
