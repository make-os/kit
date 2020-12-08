package updatedelpushkey

import (
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/logic/contracts/common"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
)

// Contract implements core.SystemContract. It is a system contract to update or delete a push key.
type Contract struct {
	core.Keepers
	tx          *txns.TxUpDelPushKey
	chainHeight uint64
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeUpDelPushKey
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxUpDelPushKey)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

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
	spk, _ := ed25519.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	common.DebitAccount(c, spk, c.tx.Fee.Decimal(), c.chainHeight)

	return nil
}
