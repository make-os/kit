package createrepo

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// CreateRepoContract is a system contract for creating a repository.
// CreateRepoContract implements SystemContract.
type CreateRepoContract struct {
	core.Logic
	tx          *txns.TxRepoCreate
	chainHeight uint64
}

// NewContract creates a new instance of CreateRepoContract
func NewContract() *CreateRepoContract {
	return &CreateRepoContract{}
}

func (c *CreateRepoContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoCreate
}

// Init initialize the contract
func (c *CreateRepoContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxRepoCreate)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *CreateRepoContract) Exec() error {

	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Create an empty repository
	newRepo := state.BareRepository()

	// Add config
	newRepo.Config = state.NewDefaultRepoConfigFromMap(c.tx.Config)

	// Apply default policies when none is set
	if len(newRepo.Config.Policies) == 0 {
		policy.AddDefaultPolicies(newRepo.Config)
	}

	// Add transaction value to repo balance
	if !c.tx.Value.IsZero() {
		newRepoBal := newRepo.Balance.Decimal().Add(c.tx.Value.Decimal())
		newRepo.Balance = util.String(newRepoBal.String())
	}

	// Register sender as owner only if proposer type is ProposerOwner
	// Register sender as a veto owner if proposer type is ProposerNetStakeholdersAndVetoOwner
	voterType := newRepo.Config.Governance.Voter
	if voterType == state.VoterOwner || voterType == state.VoterNetStakersAndVetoOwner {
		newRepo.AddOwner(spk.Addr().String(), &state.RepoOwner{
			Creator:  true,
			Veto:     voterType == state.VoterNetStakersAndVetoOwner,
			JoinedAt: util.UInt64(c.chainHeight) + 1,
		})
	}

	// Store the new repo
	c.RepoKeeper().Update(c.tx.Name, newRepo)

	// Deduct fee+value from sender
	deductible := c.tx.Value.Decimal().Add(c.tx.Fee.Decimal())
	common.DebitAccount(c, spk, deductible, c.chainHeight)

	return nil
}
