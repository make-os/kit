package gasmaker

import (
	"fmt"

	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
)

// Contract implements core.SystemContract.
// It is a system contract for creating new gas supply in response to valid proof of work nonce.
type Contract struct {
	core.Keepers
	tx          *txns.TxSubmitWork
	chainHeight uint64
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeSubmitWork
}

// Init initialize the contract
func (c *Contract) Init(logic core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = logic
	c.tx = tx.(*txns.TxSubmitWork)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	ak := c.AccountKeeper()
	sender := c.tx.GetSenderPubKey().MustAddress()

	// Get sender account.
	// Return error if sender account does not exist (should never happen).
	senderAcct := ak.Get(sender)
	if senderAcct.IsNil() {
		return fmt.Errorf("sender account not found")
	}

	// Add new gas reward
	if senderAcct.Gas.Empty() {
		senderAcct.Gas = "0"
	}
	senderAcct.Gas = util.String(senderAcct.Gas.Decimal().Add(params.GasReward.Decimal()).String())

	// Deduct fee from the sender's coin balance and increment sender's nonce
	senderCoinBal := senderAcct.Balance.Decimal()
	senderAcct.Balance = util.String(senderCoinBal.Sub(c.tx.Fee.Decimal()).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up and update the sender's account
	senderAcct.Clean(c.chainHeight)
	ak.Update(sender, senderAcct)

	// Index work nonce for epoch
	if err := c.SysKeeper().RegisterWorkNonce(c.tx.Epoch, c.tx.WorkNonce); err != nil {
		return err
	}

	return nil
}
