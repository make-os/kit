package logic

import (
	"fmt"

	"github.com/tendermint/tendermint/state"

	"github.com/makeos/mosdef/dht"
	"github.com/makeos/mosdef/validators"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// Transaction implements types.TxLogic. Provides functionalities for executing transactions.
type Transaction struct {
	logic types.Logic
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
	spk := tx.GetSenderPubKey()

	switch o := tx.(type) {
	case *types.TxCoinTransfer:
		return t.execCoinTransfer(spk, o.To, o.Value, o.Fee, chainHeight)

	case *types.TxTicketPurchase:
		if o.Is(types.TxTypeValidatorTicket) {
			return t.execValidatorStake(spk, o.Value, o.Fee, chainHeight)
		}
		if o.Is(types.TxTypeStorerTicket) {
			return t.execStorerStake(spk, o.Value, o.Fee, chainHeight)
		}
		return fmt.Errorf("unknown transaction type")

	case *types.TxSetDelegateCommission:
		return t.execSetDelegatorCommission(spk, o.Commission, o.Fee, chainHeight)

	case *types.TxTicketUnbond:
		return t.execUnbond(o.TicketHash, spk, o.Fee, chainHeight)

	case *types.TxRepoCreate:
		return t.execRepoCreate(spk, o.Name, o.Fee, chainHeight)

	case *types.TxAddGPGPubKey:
		return t.execAddGPGKey(o.PublicKey, spk, o.Fee, chainHeight)

	case *types.TxPush:
		pn := o.PushNote
		err := t.execPush(pn.RepoName, pn.References, pn.TotalFee(), pn.PusherKeyID, chainHeight)
		if err != nil {
			return err
		}
		// Execute the tx against the repository's local state
		return t.logic.GetRepoManager().ExecTxPush(o)

	case *types.TxNamespaceAcquire:
		return t.execAcquireNamespace(spk, o.Name, o.Value, o.Fee, o.TransferToRepo,
			o.TransferToAccount,
			o.Domains,
			chainHeight)

	case *types.TxNamespaceDomainUpdate:
		return t.execUpdateNamespaceDomains(spk, o.Name, o.Fee, o.Domains, chainHeight)

	case *types.TxEpochSeed:
		return nil

	default:
		return fmt.Errorf("unknown transaction type")
	}
}
