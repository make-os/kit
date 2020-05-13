package createrepo

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
)

// CreateRepoContract is a system contract for creating a repository.
// CreateRepoContract implements SystemContract.
type CreateRepoContract struct {
	core.Logic
	tx          *core.TxRepoCreate
	chainHeight uint64
}

// NewContract creates a new instance of CreateRepoContract
func NewContract() *CreateRepoContract {
	return &CreateRepoContract{}
}

func (c *CreateRepoContract) CanExec(typ types.TxCode) bool {
	return typ == core.TxTypeRepoCreate
}

// Init initialize the contract
func (c *CreateRepoContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*core.TxRepoCreate)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *CreateRepoContract) Exec() error {

	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Create the repo object; Set the config to default if
	// the passed config is unset.
	newRepo := state.BareRepository()
	newRepo.Config = state.MakeDefaultRepoConfig()
	newRepo.Config.MergeMap(c.tx.Config)

	// Apply default policies when none is set
	if len(newRepo.Config.Policies) == 0 {
		policy.AddDefaultPolicies(newRepo.Config)
	}

	voterType := newRepo.Config.Governance.Voter

	// Register sender as owner only if proposer type is ProposerOwner
	// Register sender as a veto owner if proposer type is ProposerNetStakeholdersAndVetoOwner
	if voterType == state.VoterOwner || voterType == state.VoterNetStakersAndVetoOwner {
		newRepo.AddOwner(spk.Addr().String(), &state.RepoOwner{
			Creator:  true,
			Veto:     voterType == state.VoterNetStakersAndVetoOwner,
			JoinedAt: c.chainHeight + 1,
		})
	}

	// Store the new repo
	c.RepoKeeper().Update(c.tx.Name, newRepo)

	// Deduct fee from sender
	common.DebitAccount(c, spk, c.tx.Fee.Decimal(), c.chainHeight)

	return nil
}
