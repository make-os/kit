package validators

import (
	"crypto/rsa"
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

// CheckTxCoinTransferConsistency performs consistency checks on TxCoinTransfer
func CheckTxCoinTransferConsistency(
	tx *types.TxCoinTransfer,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	pubKey, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxTicketPurchaseConsistency performs consistency checks on TxTicketPurchase
func CheckTxTicketPurchaseConsistency(
	tx *types.TxTicketPurchase,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	// When delegate is set, the delegate must have an active, non-delegated ticket
	if tx.Delegate != "" {
		r, err := logic.GetTicketManager().GetActiveTicketsByProposer(tx.Delegate, tx.Type, false)
		if err != nil {
			return errors.Wrap(err, "failed to get active delegate tickets")
		} else if len(r) == 0 {
			return feI(index, "delegate", "specified delegate is not active")
		}
	}

	// For validator ticket transaction, the value must not be lesser than
	// the current price per ticket
	if tx.Type == types.TxTypeValidatorTicket {
		curTicketPrice := logic.Sys().GetCurValidatorTicketPrice()
		if tx.Value.Decimal().LessThan(decimal.NewFromFloat(curTicketPrice)) {
			return feI(index, "value", fmt.Sprintf("value is lower than the"+
				" minimum ticket price (%f)", curTicketPrice))
		}
	}

	pubKey, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxUnbondTicketConsistency performs consistency checks on TxTicketUnbond
func CheckTxUnbondTicketConsistency(
	tx *types.TxTicketUnbond,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	// The ticket must exist
	ticket := logic.GetTicketManager().GetByHash(tx.TicketHash)
	if ticket == nil {
		return feI(index, "hash", "ticket not found")
	}

	// Ensure the tx creator is the owner of the ticket.
	// For delegated ticket, compare the delegator address with the sender address
	authErr := feI(index, "hash", "sender not authorized to unbond this ticket")
	if ticket.Delegator == "" {
		if tx.SenderPubKey != ticket.ProposerPubKey {
			return authErr
		}
	} else if ticket.Delegator != tx.GetFrom().String() {
		return authErr
	}

	// Ensure the ticket is still active
	decayBy := ticket.DecayBy
	if decayBy != 0 && decayBy > uint64(bi.Height) {
		return feI(index, "hash", "ticket is already decaying")
	} else if decayBy != 0 && decayBy <= uint64(bi.Height) {
		return feI(index, "hash", "ticket has already decayed")
	}

	pubKey, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoCreateConsistency performs consistency checks on TxRepoCreate
func CheckTxRepoCreateConsistency(
	tx *types.TxRepoCreate,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	repo := logic.RepoKeeper().GetRepo(tx.Name)
	if !repo.IsNil() {
		msg := "name is not available. choose another"
		return feI(index, "name", msg)
	}

	pubKey, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxEpochSecretConsistency performs consistency checks on TxEpochSecret
func CheckTxEpochSecretConsistency(
	tx *types.TxEpochSecret,
	index int,
	logic types.Logic) error {

	err := logic.GetDRand().Verify(tx.Secret, tx.PreviousSecret, tx.SecretRound)
	if err != nil {
		return feI(index, "secret", "epoch secret is invalid")
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

// CheckTxSetDelegateCommissionConsistency performs consistency checks on TxSetDelegateCommission
func CheckTxSetDelegateCommissionConsistency(
	tx *types.TxSetDelegateCommission,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	pubKey, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxAddGPGPubKeyConsistency performs consistency checks on TxAddGPGPubKey
func CheckTxAddGPGPubKeyConsistency(
	tx *types.TxAddGPGPubKey,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	entity, _ := crypto.PGPEntityFromPubKey(tx.PublicKey)
	pk := entity.PrimaryKey.PublicKey.(*rsa.PublicKey)

	// Ensure bit length is not less than 256
	if pk.Size() < 256 {
		msg := "gpg public key bit length must be at least 2048 bits"
		return feI(index, "pubKey", msg)
	}

	// Check whether there is a matching gpg key already existing
	pkID := util.RSAPubKeyID(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
	gpgPubKey := logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if !gpgPubKey.IsNil() {
		msg := "gpg public key already registered"
		return feI(index, "pubKey", msg)
	}

	pubKey, _ := crypto.PubKeyFromBase58(tx.GetSenderPubKey())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}
