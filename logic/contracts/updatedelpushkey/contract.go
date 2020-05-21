package updatedelpushkey

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/txns"
)

// PushKeyUpdateDeleteContract is a system contract to update or delete a push key.
// PushKeyUpdateDeleteContract implements SystemContract.
type PushKeyUpdateDeleteContract struct {
	core.Logic
	tx          *txns.TxUpDelPushKey
	chainHeight uint64
}

// NewContract creates a new instance of PushKeyUpdateDeleteContract
func NewContract() *PushKeyUpdateDeleteContract {
	return &PushKeyUpdateDeleteContract{}
}

func (c *PushKeyUpdateDeleteContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeUpDelPushKey
}

// Init initialize the contract
func (c *PushKeyUpdateDeleteContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxUpDelPushKey)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *PushKeyUpdateDeleteContract) Exec() error {

	pushKeyKeeper := c.PushKeyKeeper()
	key := pushKeyKeeper.Get(c.tx.ID)

	// If delete is requested, delete immediately and return.
	if c.tx.Delete {
		pushKeyKeeper.Remove(c.tx.ID)
		goto debit_fee
	}

	// If there are scopes to remove, remove them
	for c, i := range c.tx.RemoveScopes {
		i = i - c
		key.Scopes = key.Scopes[:i+copy(key.Scopes[i:], key.Scopes[i+1:])]
	}

	// If there are scopes to add, add them
	for _, s := range c.tx.AddScopes {
		key.Scopes = append(key.Scopes, s)
	}

	// Set fee cap if set
	if c.tx.FeeCap != "" {
		key.FeeCap = c.tx.FeeCap
	}

	// Update push key
	pushKeyKeeper.Update(c.tx.ID, key)

debit_fee:

	// Deduct network fee + proposal fee from sender
	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	common.DebitAccount(c, spk, c.tx.Fee.Decimal(), c.chainHeight)

	return nil
}
