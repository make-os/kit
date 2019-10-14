package logic

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/validators"

	"github.com/makeos/mosdef/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// Transaction implements types.TxLogic. Provides functionalities for executing transactions.
type Transaction struct {
	logic types.Logic
}

// PrepareExec decodes the transaction from the abci request,
// performs final validation before executing the transaction.
func (t *Transaction) PrepareExec(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Decode tx bytes to types.Transaction
	tx, err := types.NewTxFromBytes(req.Tx)
	if err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeFailedDecode,
			Log:  "failed to decode transaction from bytes",
		}
	}

	// Validate the transaction
	if err = validators.ValidateTx(tx, -1, t.logic); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeFailedDecode,
			Log:  "tx failed validation: " + err.Error(),
		}
	}

	// Execute the transaction
	if err = t.Exec(tx); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeExecFailure,
			Log:  "failed to execute tx: " + err.Error(),
		}
	}

	return abcitypes.ResponseDeliverTx{Code: 0}
}

// Exec execute a transaction that modifies the state.
// It returns error if the transaction is unknown.
func (t *Transaction) Exec(tx *types.Transaction) error {
	switch tx.Type {
	case types.TxTypeTransferCoin:
		return t.transferCoin(tx.SenderPubKey, tx.To, tx.Value, tx.Fee, tx.GetNonce())
	case types.TxTypeGetTicket:
		return t.stakeCoinAsValidator(tx.SenderPubKey, tx.Value, tx.Fee, tx.GetNonce())
	case types.TxTypeUnbondTicket:
		return t.unStakeCoinAsValidator(tx.SenderPubKey, tx.TicketID)
	case types.TxTypeSetDelegatorCommission:
		return t.setDelegatorCommission(tx.SenderPubKey, tx.Value)
	case types.TxTypeEpochSecret:
		return nil
	default:
		return fmt.Errorf("unknown transaction type")
	}
}

// CanTransferCoin checks whether the sender can transfer the value
// and fee of the transaction based on the current state of their
// account. It also ensures that the transaction's nonce is the
// next/expected nonce value.
func (t *Transaction) CanTransferCoin(
	txType int,
	senderPubKey *crypto.PubKey,
	recipientAddr,
	value,
	fee util.String,
	nonce uint64) error {

	var err error

	// Ensure recipient address is valid.
	// Ignore for ticket purchases tx as a recipient address is not required.
	if txType != types.TxTypeGetTicket {
		if err = crypto.IsValidAddr(recipientAddr.String()); err != nil {
			return fmt.Errorf("invalid recipient address: %s", err)
		}
	}

	// For validator ticket transaction:
	// The tx value must be equal or greater than the current ticket price.
	if txType == types.TxTypeGetTicket {
		curTicketPrice := t.logic.Sys().GetCurValidatorTicketPrice()
		if value.Decimal().LessThan(decimal.NewFromFloat(curTicketPrice)) {
			return fmt.Errorf("sender's spendable account balance is insufficient to cover "+
				"ticket price (%f)", curTicketPrice)
		}
	}

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	sender := senderPubKey.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Ensure the transaction nonce is the next expected nonce
	expectedNonce := senderAcct.Nonce + 1
	if expectedNonce != nonce {
		return fmt.Errorf("tx has invalid nonce (%d), expected (%d)", nonce, expectedNonce)
	}

	// Ensure sender has enough spendable balance to pay transfer value + fee
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.GetSpendableBalance().Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return fmt.Errorf("sender's spendable account balance is insufficient")
	}

	return nil
}

// stakeCoinAsValidator moves a given amount from the spendable
// balance of an account to the validator stake balance.
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) stakeCoinAsValidator(
	senderPubKey,
	value,
	fee util.String,
	nonce uint64) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Get the current validator stakes, add the new value to it
	// and update the account
	curValStake := senderAcct.Stakes.Get(types.StakeNameValidator)
	newValStake := curValStake.Decimal().Add(value.Decimal()).String()
	senderAcct.Stakes.Add(types.StakeNameValidator, util.String(newValStake))

	// Deduct the transaction fee and increment nonce
	senderBal := senderAcct.Balance.Decimal()
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	return nil
}

// unStakeCoinAsValidator reclaims validator stake on the sender account.
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) unStakeCoinAsValidator(senderPubKey util.String, ticketID []byte) error {

	// Get the ticket
	ticketHash := util.ToHex(ticketID)
	ticketQuery := types.Ticket{Hash: ticketHash}
	ticket, err := t.logic.GetTicketManager().QueryOne(ticketQuery)
	if err != nil {
		return errors.Wrap(err, "failed to get ticket")
	}

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Get the current validator stakes, subtract the ticket value from
	// it and update the account and increment nonce
	curValStake := senderAcct.Stakes.Get(types.StakeNameValidator)
	ticketVal := util.String(ticket.Value)
	newValStake := curValStake.Decimal().Sub(ticketVal.Decimal()).String()
	senderAcct.Stakes.Add(types.StakeNameValidator, util.String(newValStake))
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	// Mark the ticket as 'unbonded'
	if err := t.logic.GetTicketManager().MarkAsUnbonded(ticketHash); err != nil {
		return fmt.Errorf("failed to unbond ticket: %s", err)
	}

	return nil
}

// transferCoin transfer units of the native currency
// from a sender account to another account
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) transferCoin(senderPubKey, recipientAddr, value, fee util.String,
	nonce uint64) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)
	recipientAcct := acctKeeper.GetAccount(recipientAddr)
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the spend amount from the sender's account and increment nonce
	senderAcct.Balance = util.String(senderBal.Sub(spendAmt).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	// Update the recipient account
	recipientBal := recipientAcct.Balance.Decimal()
	recipientAcct.Balance = util.String(recipientBal.Add(value.Decimal()).String())
	acctKeeper.Update(recipientAddr, recipientAcct)

	return nil
}

// setDelegatorCommission sets the delegator commission of an account
func (t *Transaction) setDelegatorCommission(senderPubKey, value util.String) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Set the new commission and increment nonce
	senderAcct.DelegatorCommission, _ = value.Decimal().Float64()
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	return nil
}
