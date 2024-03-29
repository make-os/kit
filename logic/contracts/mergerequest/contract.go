package mergerequest

import (
	"fmt"

	"github.com/make-os/kit/logic/contracts/common"
	"github.com/make-os/kit/logic/proposals"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	"github.com/pkg/errors"
)

// MakeMergeRequestProposalID returns the full proposal ID of a given merge request ID
func MakeMergeRequestProposalID(id interface{}) string {
	return fmt.Sprintf("MR%v", id)
}

type Data struct {

	// RepoName is the target repo
	RepoName string

	// ProposalID is the unique proposal ID
	ProposalID string

	// ProposerFee is the amount to pay as proposal fee
	ProposerFee util.String

	// Fee is the network fee
	Fee util.String

	// CreatorAddress is the address of the proposal creator
	CreatorAddress identifier.Address

	// BaseBranch is the destination branch name
	BaseBranch string

	// BaseBranchHash is the destination branch current hash
	BaseBranchHash string

	// TargetBranch is the name of the source branch
	TargetBranch string

	// TargetBranchHash is the hash of the source branch
	TargetBranchHash string

	// repo is the target repository state
	Repo *state.Repository
}

// MergeRequestContract is a system contract that creates a merge request proposal.
// MergeRequestContract implements core.SystemContract.
type Contract struct {
	core.Keepers
	chainHeight uint64
	data        *Data
}

// NewContract creates a new instance of MergeRequestContract
func NewContract(mergeData *Data) *Contract {
	return &Contract{data: mergeData}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeMergeRequestProposalAction
}

// Init initialize the contract
func (c *Contract) Init(logic core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = logic
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	var id = MakeMergeRequestProposalID(c.data.ProposalID)
	var proposal = c.data.Repo.Proposals.Get(id)

	// Create new proposal if it does not exist already
	if proposal == nil {
		proposal = proposals.MakeProposal(c.data.CreatorAddress.String(), c.data.Repo, id, c.data.ProposerFee, c.chainHeight)
		proposal.Action = txns.TxTypeMergeRequestProposalAction
		proposal.ActionData = map[string]util.Bytes{
			constants.ActionDataKeyBaseBranch:   []byte(c.data.BaseBranch),
			constants.ActionDataKeyBaseHash:     []byte(c.data.BaseBranchHash),
			constants.ActionDataKeyTargetBranch: []byte(c.data.TargetBranch),
			constants.ActionDataKeyTargetHash:   []byte(c.data.TargetBranchHash),
		}

		// Attempt to apply the proposal action
		applied, err := proposals.MaybeApplyProposal(&proposals.ApplyProposalArgs{
			Keepers:     c,
			Proposal:    proposal,
			Repo:        c.data.Repo,
			ChainHeight: c.chainHeight,
		})
		if err != nil {
			return errors.Wrap(err, common.ErrFailedToApplyProposal)
		} else if applied {
			goto end
		}

		// Index the proposal against its end height so it can be tracked and
		// finalized at that height.
		repoKeeper := c.RepoKeeper()
		if err = repoKeeper.IndexProposalEnd(c.data.RepoName, proposal.ID, proposal.EndAt.UInt64()); err != nil {
			return errors.Wrap(err, common.ErrFailedToIndexProposal)
		}

		goto end
	}

	// At this point, the proposal exist, allow update only if it has not been finalized.
	if !proposal.IsFinalized() {
		if c.data.BaseBranch != "" {
			proposal.ActionData[constants.ActionDataKeyBaseBranch] = []byte(c.data.BaseBranch)
		}
		if c.data.BaseBranchHash != "" {
			proposal.ActionData[constants.ActionDataKeyBaseHash] = []byte(c.data.BaseBranchHash)
		}
		if c.data.TargetBranch != "" {
			proposal.ActionData[constants.ActionDataKeyTargetBranch] = []byte(c.data.TargetBranch)
		}
		if c.data.TargetBranchHash != "" {
			proposal.ActionData[constants.ActionDataKeyTargetHash] = []byte(c.data.TargetBranchHash)
		}
	}

end:
	return nil
}
