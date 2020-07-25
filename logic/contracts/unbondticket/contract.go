package unbondticket

import (
	"fmt"

	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/params"
	"github.com/themakeos/lobe/types"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
)

// TicketUnbondContract is a system contract to unbond a ticket.
// TicketUnbondContract implements SystemContract.
type TicketUnbondContract struct {
	core.Logic
	tx          *txns.TxTicketUnbond
	chainHeight uint64
}

// NewContract creates an instance of TicketUnbondContract
func NewContract() *TicketUnbondContract {
	return &TicketUnbondContract{}
}

func (c *TicketUnbondContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeUnbondHostTicket
}

func (c *TicketUnbondContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxTicketUnbond)
	c.chainHeight = curChainHeight
	return c
}

func (c *TicketUnbondContract) Exec() error {

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
