package purchaseticket

import (
	"fmt"

	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
)

// Contract implements core.SystemContract. It is a system contract for purchasing a ticket.
type Contract struct {
	core.Keepers
	tx          *txns.TxTicketPurchase
	chainHeight uint64
}

// NewContract creates an instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeValidatorTicket || typ == txns.TxTypeHostTicket
}

func (c *Contract) Init(logic core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = logic
	c.tx = tx.(*txns.TxTicketPurchase)
	c.chainHeight = curChainHeight
	return c
}

func (c *Contract) Exec() error {

	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	acctKeeper := c.AccountKeeper()
	fee := c.tx.Fee
	txType := c.tx.GetType()
	value := c.tx.Value

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.Get(sender)

	// Deduct the transaction fee and increment nonce
	senderBal := senderAcct.Balance.Decimal()
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Register a stake entry
	unbondHeight := uint64(0)
	switch txType {
	case txns.TxTypeValidatorTicket:

		// Determine unbond height. The unbond height is height of the next block
		// (or proposed block) plus minimum ticket maturation duration, max ticket
		// active duration + thawing period.
		unbondHeight = c.chainHeight + 1 + uint64(params.MinTicketMatDur) +
			uint64(params.MaxTicketActiveDur) +
			uint64(params.NumBlocksInThawPeriod)
		senderAcct.Stakes.Add(state.StakeTypeValidator, value, unbondHeight)

	case txns.TxTypeHostTicket:
		senderAcct.Stakes.Add(state.StakeTypeHost, value, unbondHeight)

	default:
		return fmt.Errorf("unknown transaction type")
	}

	// Update the sender's account
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}
