package logic

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/state"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/validators"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/makeos/mosdef/types"
)

// Transaction implements types.TxLogic. Provides functionalities for executing transactions.
type Transaction struct {
	logic core.Logic
}

// ExecTx decodes the transaction from the abci request,
// performs final validation before executing the transaction.
// chainHeight: The height of the block chain
func (t *Transaction) ExecTx(tx types.BaseTx, chainHeight uint64) abcitypes.ResponseDeliverTx {

	// Validate the transaction
	if err := validators.ValidateTx(tx, -1, t.logic); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeFailedDecode,
			Log:  "tx failed validation: " + err.Error(),
		}
	}

	// Execute the transaction
	if err := t.exec(tx, chainHeight); err != nil {

		code := types.ErrCodeExecFailure
		if errors.Cause(err).Error() == dht.ErrObjNotFound.Error() {
			code = state.ErrCodeReExecBlock
		}

		return abcitypes.ResponseDeliverTx{
			Code: code,
			Log:  "failed to execute tx: " + err.Error(),
		}
	}

	return abcitypes.ResponseDeliverTx{Code: 0}
}

// exec execute a transaction that modifies the state.
// It returns error if the transaction is unknown.
// tx: The transaction to be processed
// chainHeight: The height of the block chain
func (t *Transaction) exec(tx types.BaseTx, chainHeight uint64) error {
	spk := tx.GetSenderPubKey().ToBytes32()

	switch o := tx.(type) {
	case *core.TxCoinTransfer:
		return t.execCoinTransfer(
			spk,
			o.To,
			o.Value,
			o.Fee,
			chainHeight)

	case *core.TxTicketPurchase:
		if o.Is(core.TxTypeValidatorTicket) {
			return t.execValidatorStake(
				spk,
				o.Value,
				o.Fee,
				chainHeight)
		}
		if o.Is(core.TxTypeHostTicket) {
			return t.execHostStake(
				spk,
				o.Value,
				o.Fee,
				chainHeight)
		}
		return fmt.Errorf("unknown transaction type")

	case *core.TxSetDelegateCommission:
		return t.execSetDelegatorCommission(
			spk,
			o.Commission,
			o.Fee,
			chainHeight)

	case *core.TxTicketUnbond:
		return t.execUnbond(
			spk,
			o.TicketHash,
			o.Fee,
			chainHeight)

	case *core.TxRepoCreate:
		return t.execRepoCreate(
			spk,
			o.Name,
			o.Config,
			o.Fee,
			chainHeight)

	case *core.TxRepoProposalUpsertOwner:
		return t.execRepoProposalUpsertOwner(
			spk,
			o.RepoName,
			o.ProposalID,
			o.Addresses,
			o.Veto,
			o.Value,
			o.Fee,
			chainHeight)

	case *core.TxRepoProposalVote:
		return t.execRepoProposalVote(
			spk,
			o.RepoName,
			o.ProposalID,
			o.Vote,
			o.Fee,
			chainHeight)

	case *core.TxRepoProposalSendFee:
		return t.execRepoProposalFeeDeposit(
			spk,
			o.RepoName,
			o.ProposalID,
			o.Value,
			o.Fee,
			chainHeight)

	case *core.TxRepoProposalUpdate:
		return t.execRepoProposalUpdate(
			spk,
			o.RepoName,
			o.ProposalID,
			o.Config,
			o.Value,
			o.Fee,
			chainHeight)

	case *core.TxRepoProposalRegisterGPGKey:
		return t.execRepoProposalRegisterGPGKeys(
			spk,
			o.RepoName,
			o.ProposalID,
			o.KeyIDs,
			o.FeeMode,
			o.FeeCap,
			o.Policies,
			o.Namespace,
			o.NamespaceOnly,
			o.Value,
			o.Fee,
			chainHeight)

	case *core.TxRepoProposalMergeRequest:
		return t.execRepoProposalMergeRequest(
			spk,
			o.RepoName,
			o.ProposalID,
			o.BaseBranch,
			o.BaseBranchHash,
			o.TargetBranch,
			o.TargetBranchHash,
			o.Value,
			o.Fee,
			chainHeight)

	case *core.TxRegisterGPGPubKey:
		return t.execRegisterGPGKey(
			spk,
			o.PublicKey,
			o.Scopes,
			o.FeeCap,
			o.Fee,
			chainHeight)

	case *core.TxPush:
		pn := o.PushNote
		err := t.execPush(
			pn.RepoName,
			pn.References,
			pn.GetFee(),
			pn.PusherKeyID,
			chainHeight)
		if err != nil {
			return err
		}
		// Execute the tx against the repository's local state
		return t.logic.GetRepoManager().ExecTxPush(o)

	case *core.TxNamespaceAcquire:
		return t.execAcquireNamespace(
			spk,
			o.Name,
			o.Value,
			o.Fee,
			o.TransferToRepo,
			o.TransferToAccount,
			o.Domains,
			chainHeight)

	case *core.TxNamespaceDomainUpdate:
		return t.execUpdateNamespaceDomains(
			spk,
			o.Name,
			o.Fee,
			o.Domains,
			chainHeight)

	default:
		return fmt.Errorf("unknown transaction type")
	}
}
