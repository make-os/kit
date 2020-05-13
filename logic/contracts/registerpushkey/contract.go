package registerpushkey

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
)

// RegisterPushKeyContract is a system contract for creating a repository.
// RegisterPushKeyContract implements SystemContract.
type RegisterPushKeyContract struct {
	core.Logic
	tx          *core.TxRegisterPushKey
	chainHeight uint64
}

// NewContract creates a new instance of RegisterPushKeyContract
func NewContract() *RegisterPushKeyContract {
	return &RegisterPushKeyContract{}
}

func (c *RegisterPushKeyContract) CanExec(typ types.TxCode) bool {
	return typ == core.TxTypeRegisterPushKey
}

// Init initialize the contract
func (c *RegisterPushKeyContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*core.TxRegisterPushKey)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *RegisterPushKeyContract) Exec() error {

	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Create a new PushKey
	key := state.BarePushKey()
	key.PubKey = c.tx.PublicKey
	key.Address = spk.Addr()
	key.Scopes = c.tx.Scopes
	key.FeeCap = c.tx.FeeCap

	// Store the new public key
	pushKeyID := crypto.CreatePushKeyID(c.tx.PublicKey)
	c.PushKeyKeeper().Update(pushKeyID, key)

	// Deduct fee and update account
	common.DebitAccount(c, spk, c.tx.Fee.Decimal(), c.chainHeight)

	return nil
}
