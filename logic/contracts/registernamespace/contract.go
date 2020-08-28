package registernamespace

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
)

// Contract implements core.SystemContract. It is a system contract to register a namespace.
type Contract struct {
	core.Keepers
	tx          *txns.TxNamespaceRegister
	chainHeight uint64
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeNamespaceRegister
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxNamespaceRegister)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {
	spk := crypto.MustPubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Get the current namespace object and re-populate it
	ns := c.NamespaceKeeper().Get(c.tx.Name)
	ns.Owner = spk.Addr().String()
	ns.ExpiresAt.Set(c.chainHeight + uint64(params.NamespaceTTL))
	ns.GraceEndAt.Set(ns.ExpiresAt.UInt64() + uint64(params.NamespaceGraceDur))
	ns.Domains = c.tx.Domains
	ns.Owner = c.tx.To

	// Set default owner to sender.
	if ns.Owner == "" {
		ns.Owner = spk.Addr().String()
	}

	// Get the account of the sender
	acctKeeper := c.AccountKeeper()
	senderAcct := acctKeeper.Get(spk.Addr())

	// Deduct the fee + value
	senderAcctBal := senderAcct.Balance.Decimal()
	spendAmt := c.tx.Value.Decimal().Add(c.tx.Fee.Decimal())
	senderAcct.Balance = util.String(senderAcctBal.Sub(spendAmt).String())

	// Increment sender nonce, clean up and update
	senderAcct.Nonce = senderAcct.Nonce + 1
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	// Send the value to the treasury
	treasuryAcct := acctKeeper.Get(identifier.Address(params.TreasuryAddress), c.chainHeight)
	treasuryBal := treasuryAcct.Balance.Decimal()
	treasuryAcct.Balance = util.String(treasuryBal.Add(c.tx.Value.Decimal()).String())
	treasuryAcct.Clean(c.chainHeight)
	acctKeeper.Update(identifier.Address(params.TreasuryAddress), treasuryAcct)

	// Update the namespace
	c.NamespaceKeeper().Update(c.tx.Name, ns)

	return nil
}
