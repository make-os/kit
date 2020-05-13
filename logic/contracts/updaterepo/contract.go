package updaterepo

import (
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/logic/proposals"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// UpdateRepoContract is a system contract that creates a proposal to
// update a repository. UpdateRepoContract implements ProposalContract.
type UpdateRepoContract struct {
	core.Logic
	tx          *core.TxRepoProposalUpdate
	chainHeight uint64
	contracts   []core.SystemContract
}

// NewContract creates a new instance of UpdateRepoContract
func NewContract(contracts []core.SystemContract) *UpdateRepoContract {
	return &UpdateRepoContract{contracts: contracts}
}

func (c *UpdateRepoContract) CanExec(typ types.TxCode) bool {
	return typ == core.TxTypeRepoProposalUpdate
}

// Init initialize the contract
func (c *UpdateRepoContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*core.TxRepoProposalUpdate)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *UpdateRepoContract) Exec() error {

	// Get the repo
	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.RepoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	proposal := proposals.MakeProposal(spk, repo, c.tx.ProposalID, c.tx.Value, c.chainHeight)
	proposal.Action = core.TxTypeRepoProposalUpdate
	proposal.ActionData[constants.ActionDataKeyCFG] = util.ToBytes(c.tx.Config)

	// Deduct network fee + proposal fee from sender
	totalFee := c.tx.Fee.Decimal().Add(c.tx.Value.Decimal())
	common.DebitAccount(c, spk, totalFee, c.chainHeight)

	// Attempt to apply the proposal action
	applied, err := proposals.MaybeApplyProposal(&proposals.ApplyProposalArgs{
		Keepers:     c,
		Proposal:    proposal,
		Repo:        repo,
		ChainHeight: c.chainHeight,
		Contracts:   c.contracts,
	})
	if err != nil {
		return errors.Wrap(err, "failed to apply proposal")
	} else if applied {
		goto update
	}

	// Index the proposal against its end height so it
	// can be tracked and finalized at that height.
	if err = repoKeeper.IndexProposalEnd(c.tx.RepoName, proposal.ID, proposal.EndAt); err != nil {
		return errors.Wrap(err, "failed to index proposal against end height")
	}

update:
	repoKeeper.Update(c.tx.RepoName, repo)
	return nil
}

// Apply applies the proposal
func (c *UpdateRepoContract) Apply(args *core.ProposalApplyArgs) error {
	var cfgUpd map[string]interface{}
	util.ToObject(args.Proposal.GetActionData()[constants.ActionDataKeyCFG], &cfgUpd)
	args.Repo.Config.MergeMap(cfgUpd)
	return nil
}
