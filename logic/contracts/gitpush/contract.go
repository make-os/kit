package gitpush

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/common"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
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

// Exec executes the contract
func (c *GitPush) Exec() error {

	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.PushNote.RepoName)

	// Register or update references
	for _, ref := range c.tx.PushNote.References {

		// When the reference needs to be deleted, remove from repo reference
		curRef := repo.References.Get(ref.Name)
		if ref.IsDeletable() && !curRef.IsNil() {
			delete(repo.References, ref.Name)
			continue
		}

		// Set pusher as creator if reference is new
		if curRef.IsNil() {
			curRef.Creator = c.tx.PushNote.PushKeyID
		}

		curRef.Closed = ref.Data.Close == 1
		curRef.Nonce = curRef.Nonce + 1
		curRef.Hash = util.MustFromHex(ref.NewHash)
		repo.References[ref.Name] = curRef
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

	return nil
}
