package burnforswap

import (
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
)

var (
	fe = errors.FieldError
)

// Contract implements core.SystemContract. It is a system contract for
// burning coin or gas balance such that an external system may swap
// the burned balance within an external system.
type Contract struct {
	core.Keepers
	tx          *txns.TxBurnForSwap
	chainHeight uint64
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeBurnForSwap
}

// Init initialize the contract
func (c *Contract) Init(logic core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = logic
	c.tx = tx.(*txns.TxBurnForSwap)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	acctKeeper := c.AccountKeeper()
	senderAddr := c.tx.GetFrom()
	senderAcct := acctKeeper.Get(senderAddr)
	fee := c.tx.Fee.Decimal()

	// Deduct the amount to be burned from the gas balance
	amountToBurn := c.tx.Amount.Decimal()
	if c.tx.Gas {
		senderAcct.SetGasBalance(senderAcct.GetGasBalance().Decimal().Sub(amountToBurn).String())
	} else {
		senderAcct.SetBalance(senderAcct.GetBalance().Decimal().Sub(amountToBurn).String())
	}

	// Deduct fee and update nonce
	senderAcct.Balance = util.String(senderAcct.Balance.Decimal().Sub(fee).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up the recipient object and save the new state of the object
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(senderAddr, senderAcct)

	return nil
}
