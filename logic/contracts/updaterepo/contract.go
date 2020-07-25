package updaterepo

import (
	"github.com/pkg/errors"
	"gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/logic/contracts/common"
	"gitlab.com/makeos/lobe/logic/proposals"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/constants"
	"gitlab.com/makeos/lobe/types/core"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
)

// UpdateRepoContract is a system contract that creates a proposal to
// update a repository. UpdateRepoContract implements ProposalContract.
type UpdateRepoContract struct {
	core.Logic
	tx          *txns.TxRepoProposalUpdate
	chainHeight uint64
	contracts   *[]core.SystemContract
}

// NewContract creates a new instance of UpdateRepoContract
func NewContract(contracts *[]core.SystemContract) *UpdateRepoContract {
	return &UpdateRepoContract{contracts: contracts}
}

func (c *UpdateRepoContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoProposalUpdate
}

// Init initialize the contract
func (c *UpdateRepoContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxRepoProposalUpdate)
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
	proposal := proposals.MakeProposal(spk.Addr().String(), repo, c.tx.ID, c.tx.Value, c.chainHeight)
	proposal.Action = txns.TxTypeRepoProposalUpdate
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
		Contracts:   *c.contracts,
	})
	if err != nil {
		return errors.Wrap(err, common.ErrFailedToApplyProposal)
	} else if applied {
		goto update
	}

	// Index the proposal against its end height so it
	// can be tracked and finalized at that height.
	if err = repoKeeper.IndexProposalEnd(c.tx.RepoName, proposal.ID, proposal.EndAt.UInt64()); err != nil {
		return errors.Wrap(err, common.ErrFailedToIndexProposal)
	}

update:
	repoKeeper.Update(c.tx.RepoName, repo)
	return nil
}

// Apply applies the proposal action
func (c *UpdateRepoContract) Apply(args *core.ProposalApplyArgs) error {
	var cfgUpd map[string]interface{}
	if err := util.ToObject(args.Proposal.GetActionData()[constants.ActionDataKeyCFG], &cfgUpd); err != nil {
		return err
	}
	return args.Repo.Config.MergeMap(cfgUpd)
}
