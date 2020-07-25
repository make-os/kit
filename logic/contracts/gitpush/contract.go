package gitpush

import (
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/lobe/logic/contracts/common"
	"gitlab.com/makeos/lobe/logic/contracts/mergerequest"
	"gitlab.com/makeos/lobe/remote/plumbing"
	pushtypes "gitlab.com/makeos/lobe/remote/push/types"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/core"
	"gitlab.com/makeos/lobe/types/state"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
)

// GitPush is a system contract to process a push transaction.
// GitPush implements SystemContract.
type GitPush struct {
	core.Logic
	tx          *txns.TxPush
	chainHeight uint64
}

// NewContract creates a new instance of GitPush
func NewContract() *GitPush {
	return &GitPush{}
}

func (c *GitPush) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypePush
}

// Init initialize the contract
func (c *GitPush) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxPush)
	c.chainHeight = curChainHeight
	return c
}

func (c *GitPush) execReference(repo *state.Repository, repoName string, ref *pushtypes.PushedReference) error {

	// When the reference needs to be deleted, remove from repo reference
	r := repo.References.Get(ref.Name)
	if !r.IsNil() && ref.IsDeletable() {
		delete(repo.References, ref.Name)
		return nil
	}

	// Set pusher as creator if reference is new
	isNewRef := r.IsNil()
	if isNewRef {
		r.Creator = c.tx.Note.GetPusherKeyID()
	}

	// Set issue data for issue reference
	if plumbing.IsIssueReference(ref.Name) {
		if ref.Data.Close != nil {
			r.Data.Closed = *ref.Data.Close
		}

		// Process labels (new and negated labels)
		if ref.Data.Labels != nil {
			for _, label := range *ref.Data.Labels {
				if label[0] == '-' {
					r.Data.Labels = util.RemoveFromStringSlice(r.Data.Labels, label[1:])
					continue
				}
				if !funk.ContainsString(r.Data.Labels, label) {
					r.Data.Labels = append(r.Data.Labels, label)
				}
			}
		}

		// Process assignees (new and negated assignees)
		if ref.Data.Assignees != nil {
			for _, assignee := range *ref.Data.Assignees {
				if assignee[0] == '-' {
					r.Data.Assignees = util.RemoveFromStringSlice(r.Data.Assignees, assignee[1:])
					continue
				}
				if !funk.ContainsString(r.Data.Assignees, assignee) {
					r.Data.Assignees = append(r.Data.Assignees, assignee)
				}
			}
		}
	}

	// For only new merge request reference, call the merge request contract to handle it
	if plumbing.IsMergeRequestReference(ref.Name) {
		if err := mergerequest.NewContract(&mergerequest.MergeRequestData{
			Repo:             repo,
			RepoName:         repoName,
			ProposalID:       plumbing.GetReferenceShortName(ref.Name),
			ProposerFee:      ref.Value,
			Fee:              ref.Fee,
			CreatorAddress:   c.tx.GetFrom(),
			BaseBranch:       ref.Data.BaseBranch,
			BaseBranchHash:   ref.Data.BaseBranchHash,
			TargetBranch:     ref.Data.TargetBranch,
			TargetBranchHash: ref.Data.TargetBranchHash,
		}).Init(c.Logic, nil, c.chainHeight).Exec(); err != nil {
			return err
		}

		// If there is a secondary fee, deduct it only when reference is new
		if isNewRef && ref.Value != "" {
			totalFee := ref.Value.Decimal()
			common.DebitAccountByAddress(c, c.tx.GetFrom(), totalFee, c.chainHeight)
		}
	}

	// Deduct reference push fee
	totalFee := ref.Fee.Decimal()
	common.DebitAccountByAddress(c, c.tx.GetFrom(), totalFee, c.chainHeight)

	r.Nonce = r.Nonce + 1
	r.Hash = util.MustFromHex(ref.NewHash)
	repo.References[ref.Name] = r

	return nil
}

// Exec executes the contract
func (c *GitPush) Exec() error {
	repoName := c.tx.Note.GetRepoName()
	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(repoName)

	// Register or update references
	for _, ref := range c.tx.Note.GetPushedReferences() {
		if err := c.execReference(repo, repoName, ref); err != nil {
			return err
		}
	}

	// Update the repo
	repoKeeper.Update(repoName, repo)

	return nil
}
