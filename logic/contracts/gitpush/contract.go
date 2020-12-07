package gitpush

import (
	"github.com/make-os/kit/logic/contracts/common"
	"github.com/make-os/kit/logic/contracts/mergerequest"
	"github.com/make-os/kit/remote/plumbing"
	pushtypes "github.com/make-os/kit/remote/push/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/thoas/go-funk"
)

// Contract implements core.SystemContract. It is a system contract to process a push transaction.
type Contract struct {
	core.Keepers
	tx          *txns.TxPush
	chainHeight uint64
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypePush
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxPush)
	c.chainHeight = curChainHeight
	return c
}

func (c *Contract) execReference(repo *state.Repository, repoName string, ref *pushtypes.PushedReference) error {

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

		// Set close status if set in reference payload
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

		// Set close status if set in reference payload
		if ref.Data.Close != nil {
			r.Data.Closed = *ref.Data.Close
		}

		// Execute merge request contract
		if err := mergerequest.NewContract(&mergerequest.Data{
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
		}).Init(c.Keepers, nil, c.chainHeight).Exec(); err != nil {
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
	repo.UpdatedAt = util.UInt64(c.chainHeight + 1)

	return nil
}

// Exec executes the contract
func (c *Contract) Exec() error {
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
