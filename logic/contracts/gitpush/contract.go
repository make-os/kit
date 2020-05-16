package gitpush

import (
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// GitPush is a system contract to process a push transaction.
// GitPush implements SystemContract.
type GitPush struct {
	core.Logic
	tx          *core.TxPush
	chainHeight uint64
}

// NewContract creates a new instance of GitPush
func NewContract() *GitPush {
	return &GitPush{}
}

func (c *GitPush) CanExec(typ types.TxCode) bool {
	return typ == core.TxTypePush
}

// Init initialize the contract
func (c *GitPush) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*core.TxPush)
	c.chainHeight = curChainHeight
	return c
}

func (c *GitPush) updateReference(repo *state.Repository, ref *core.PushedReference) {

	// When the reference needs to be deleted, remove from repo reference
	r := repo.References.Get(ref.Name)
	if ref.IsDeletable() && !r.IsNil() {
		delete(repo.References, ref.Name)
		return
	}

	// Set pusher as creator if reference is new
	if r.IsNil() {
		r.Creator = c.tx.PushNote.PushKeyID
	}

	// Set issue data for issue reference
	if plumbing.IsIssueReference(ref.Name) {
		if ref.Data.Close != nil {
			r.IssueData.Closed = *ref.Data.Close
		}

		// Process labels (new and negated labels)
		if ref.Data.Labels != nil {
			for _, label := range *ref.Data.Labels {
				if label[0] == '-' {
					r.IssueData.Labels = util.RemoveFromStringSlice(r.IssueData.Labels, label[1:])
					continue
				}
				if !funk.ContainsString(r.IssueData.Labels, label) {
					r.IssueData.Labels = append(r.IssueData.Labels, label)
				}
			}
		}

		// Process assignees (new and negated assignees)
		if ref.Data.Assignees != nil {
			for _, assignee := range *ref.Data.Assignees {
				if assignee[0] == '-' {
					r.IssueData.Assignees = util.RemoveFromStringSlice(r.IssueData.Assignees, assignee[1:])
					continue
				}
				if !funk.ContainsString(r.IssueData.Assignees, assignee) {
					r.IssueData.Assignees = append(r.IssueData.Assignees, assignee)
				}
			}
		}
	}

	r.Nonce = r.Nonce + 1
	r.Hash = util.MustFromHex(ref.NewHash)
	repo.References[ref.Name] = r
}

// Exec executes the contract
func (c *GitPush) Exec() error {

	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.PushNote.RepoName)

	// Register or update references
	for _, ref := range c.tx.PushNote.References {
		c.updateReference(repo, ref)
	}

	// Get the push key of the pusher
	pushKey := c.PushKeyKeeper().Get(crypto.BytesToPushKeyID(c.tx.PushNote.PushKeyID), c.chainHeight)

	// Get the account of the pusher
	acctKeeper := c.AccountKeeper()
	pusherAcct := acctKeeper.Get(pushKey.Address)

	// Update the repo
	repoKeeper.Update(c.tx.PushNote.RepoName, repo)

	// Deduct the pusher's fee
	common.DebitAccountObject(c, pushKey.Address, pusherAcct, c.tx.Fee.Decimal(), c.chainHeight)

	return c.GetRemoteServer().ExecTxPush(c.tx)
}
