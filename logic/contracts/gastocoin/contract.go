package gastocoin

import (
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/shopspring/decimal"
)

var (
	fe = errors.FieldError
)

// Contract implements core.SystemContract. It is a system contract for converting
// gas balance to coin balance.
type Contract struct {
	core.Keepers
	tx          *txns.TxBurnGasForCoin
	chainHeight uint64
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeBurnGasForCoin
}

// Init initialize the contract
func (c *Contract) Init(logic core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = logic
	c.tx = tx.(*txns.TxBurnGasForCoin)
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
	senderAcct.SetGasBalance(senderAcct.GetGasBalance().Decimal().Sub(amountToBurn).String())

	// Calculate the exchange amount and update balance
	exAmt := amountToBurn.Mul(decimal.NewFromFloat(params.GasToCoinExRate))
	senderAcct.Balance = util.String(senderAcct.Balance.Decimal().Add(exAmt).String())

	// Deduct fee and update nonce
	senderAcct.Balance = util.String(senderAcct.Balance.Decimal().Sub(fee).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up the recipient object and save the new state of the object
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(senderAddr, senderAcct)

	return nil
}
