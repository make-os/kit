package depositproposalfee

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/logic/contracts/common"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
)

// Contract implements core.SystemContract. It is a system contract for adding deposit fee to a proposal.
type Contract struct {
	core.Keepers
	tx          *txns.TxRepoProposalSendFee
	chainHeight uint64
	contracts   []core.SystemContract
}

// NewContract creates a new instance of DepositProposalFeeContract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoProposalSendFee
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxRepoProposalSendFee)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {
	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Get the repo and proposal
	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.RepoName)
	prop := repo.Proposals.Get(c.tx.ID)

	// Register proposal fee if set.
	// If the sender already deposited, update their deposit.
	if c.tx.Value != "0" {
		addr := spk.Addr().String()
		if !prop.Fees.Has(addr) {
			prop.Fees.Add(addr, c.tx.Value.String())
		} else {
			existingFee := prop.Fees.Get(addr)
			updFee := existingFee.Decimal().Add(c.tx.Value.Decimal())
			prop.Fees.Add(addr, updFee.String())
		}
	}

	// Deduct network fee + proposal fee from sender
	totalFee := c.tx.Fee.Decimal().Add(c.tx.Value.Decimal())
	common.DebitAccount(c, spk, totalFee, c.chainHeight)

	repoKeeper.Update(c.tx.RepoName, repo)

	return nil
}
