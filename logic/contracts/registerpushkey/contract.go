package registerpushkey

import (
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/logic/contracts/common"
	"github.com/themakeos/lobe/types"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
)

// RegisterPushKeyContract is a system contract for creating a repository.
// RegisterPushKeyContract implements SystemContract.
type RegisterPushKeyContract struct {
	core.Logic
	tx                 *txns.TxRegisterPushKey
	chainHeight        uint64
	disableSenderDebit bool
}

// NewContract creates a new instance of RegisterPushKeyContract
func NewContract() *RegisterPushKeyContract {
	return &RegisterPushKeyContract{}
}

// NewContractWithNoSenderUpdate is like NewContract but disables fee debit and nonce
// update on the sender's account. This is meant to be used when calling the
// contract from other contract that intend to handle sender account update
// them self.
func NewContractWithNoSenderUpdate() *RegisterPushKeyContract {
	return &RegisterPushKeyContract{disableSenderDebit: true}
}

func (c *RegisterPushKeyContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRegisterPushKey
}

// Init initialize the contract
func (c *RegisterPushKeyContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxRegisterPushKey)
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
	if !c.disableSenderDebit {
		common.DebitAccount(c, spk, c.tx.Fee.Decimal(), c.chainHeight)
	}

	return nil
}
