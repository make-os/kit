package updatenamespacedomains

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
)

// NamespaceDomainUpdateContract is a system contract to update the domains of a namespace.
// NamespaceDomainUpdateContract implements SystemContract.
type NamespaceDomainUpdateContract struct {
	core.Logic
	tx          *txns.TxNamespaceDomainUpdate
	chainHeight uint64
}

// NewContract creates a new instance of NamespaceDomainUpdateContract
func NewContract() *NamespaceDomainUpdateContract {
	return &NamespaceDomainUpdateContract{}
}

func (c *NamespaceDomainUpdateContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeNamespaceDomainUpdate
}

// Init initialize the contract
func (c *NamespaceDomainUpdateContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxNamespaceDomainUpdate)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *NamespaceDomainUpdateContract) Exec() error {
	spk := crypto.MustPubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Get the current namespace object and update.
	// Remove existing domain if it is referenced in the update and has not target.
	ns := c.NamespaceKeeper().Get(c.tx.Name)
	for domain, target := range c.tx.Domains {
		if _, ok := ns.Domains[domain]; !ok {
			ns.Domains[domain] = target
			continue
		}
		if target != "" {
			ns.Domains[domain] = target
			continue
		}
		delete(ns.Domains, domain)
	}

	// Update the namespace
	c.NamespaceKeeper().Update(c.tx.Name, ns)

	// Get the account of the sender
	acctKeeper := c.AccountKeeper()
	senderAcct := acctKeeper.Get(spk.Addr())

	// Deduct the fee + value
	senderAcctBal := senderAcct.Balance.Decimal()
	spendAmt := c.tx.Fee.Decimal()
	senderAcct.Balance = util.String(senderAcctBal.Sub(spendAmt).String())

	// Increment sender nonce, clean up and update
	senderAcct.Nonce = senderAcct.Nonce + 1
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	return nil
}
