package gitpush

import (
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	types3 "gitlab.com/makeos/mosdef/remote/pushpool/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
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

func (c *GitPush) updateReference(repo *state.Repository, ref *types3.PushedReference) {

	// When the reference needs to be deleted, remove from repo reference
	r := repo.References.Get(ref.Name)
	if ref.IsDeletable() && !r.IsNil() {
		delete(repo.References, ref.Name)
		return
	}

	// Set pusher as creator if reference is new
	if r.IsNil() {
		r.Creator = c.tx.PushNote.GetPusherKeyID()
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

	r.Nonce = r.Nonce + 1
	r.Hash = util.MustFromHex(ref.NewHash)
	repo.References[ref.Name] = r
}

// Exec executes the contract
func (c *GitPush) Exec() error {

	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.PushNote.GetRepoName())

	// Register or update references
	for _, ref := range c.tx.PushNote.GetPushedReferences() {
		c.updateReference(repo, ref)
	}

	// Get the push key of the pusher
	pushKey := c.PushKeyKeeper().Get(crypto.BytesToPushKeyID(c.tx.PushNote.GetPusherKeyID()), c.chainHeight)

	// Get the account of the pusher
	acctKeeper := c.AccountKeeper()
	pusherAcct := acctKeeper.Get(pushKey.Address)

	// Update the repo
	repoKeeper.Update(c.tx.PushNote.GetRepoName(), repo)

	// Deduct the pusher's fee
	common.DebitAccountObject(c, pushKey.Address, pusherAcct, c.tx.Fee.Decimal(), c.chainHeight)

	return c.GetRemoteServer().ExecTxPush(c.tx)
}
