package setdelcommission

import (
	"gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/core"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
)

// SetDelegateCommissionContract is a system contract for setting an account's delegate commission.
// SetDelegateCommissionContract implements SystemContract.
type SetDelegateCommissionContract struct {
	core.Logic
	tx          *txns.TxSetDelegateCommission
	chainHeight uint64
}

// NewContract creates a new instance of SetDelegateCommissionContract
func NewContract() *SetDelegateCommissionContract {
	return &SetDelegateCommissionContract{}
}

func (c *SetDelegateCommissionContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeSetDelegatorCommission
}

// Init initialize the contract
func (c *SetDelegateCommissionContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxSetDelegateCommission)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *SetDelegateCommissionContract) Exec() error {

	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	acctKeeper := c.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.Get(sender)
	senderBal := senderAcct.Balance.Decimal()

	// Set the new commission
	senderAcct.DelegatorCommission, _ = c.tx.Commission.Decimal().Float64()

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(c.tx.Fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}
