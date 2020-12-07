package unbondticket

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

// Contract implements core.SystemContract. It is a system contract to unbond a ticket.
type Contract struct {
	core.Keepers
	tx          *txns.TxTicketUnbond
	chainHeight uint64
}

// NewContract creates an instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeUnbondHostTicket
}

func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxTicketUnbond)
	c.chainHeight = curChainHeight
	return c
}

func (c *Contract) Exec() error {

	// Get sender account
	acctKeeper := c.AccountKeeper()
	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	senderAcct := acctKeeper.Get(spk.Addr(), c.chainHeight)
	senderBal := senderAcct.Balance.Decimal()

	// Get the ticket
	ticket := c.GetTicketManager().GetByHash(c.tx.TicketHash)
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	// Set new unbond height
	newUnbondHeight := c.chainHeight + 1 + uint64(params.NumBlocksInHostThawPeriod)
	senderAcct.Stakes.UpdateUnbondHeight(state.StakeTypeHost, ticket.Value, 0, newUnbondHeight)

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(c.tx.Fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	return nil
}
