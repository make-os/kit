package registerpushkey

import (
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/logic/contracts/common"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
)

// Contract implements core.SystemContract. It is a system contract for creating a repository.
type Contract struct {
	core.Keepers
	tx                 *txns.TxRegisterPushKey
	chainHeight        uint64
	disableSenderDebit bool
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

// NewContractWithNoSenderUpdate is like NewContract but disables fee debit and nonce
// update on the sender's account. This is meant to be used when calling the
// contract from other contract that intend to handle sender account update
// them self.
func NewContractWithNoSenderUpdate() *Contract {
	return &Contract{disableSenderDebit: true}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRegisterPushKey
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxRegisterPushKey)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	spk, _ := ed25519.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Create a new PushKey
	key := state.BarePushKey()
	key.PubKey = c.tx.PublicKey
	key.Address = spk.Addr()
	key.Scopes = c.tx.Scopes
	key.FeeCap = c.tx.FeeCap

	// Store the new public key
	pushKeyID := ed25519.CreatePushKeyID(c.tx.PublicKey)
	c.PushKeyKeeper().Update(pushKeyID, key)

	// Deduct fee and update account
	if !c.disableSenderDebit {
		common.DebitAccount(c, spk, c.tx.Fee.Decimal(), c.chainHeight)
	}

	return nil
}
