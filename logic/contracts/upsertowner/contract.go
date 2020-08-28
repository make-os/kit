package upsertowner

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/logic/contracts/common"
	common2 "github.com/make-os/lobe/logic/proposals"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/pkg/errors"
)

// Contract implements core.ProposalContract. It is a system contract that
// creates a proposal to update or insert a repo owner.
type Contract struct {
	core.Keepers
	tx          *txns.TxRepoProposalUpsertOwner
	chainHeight uint64
	contracts   *[]core.SystemContract
}

// NewContract creates a new instance of Contract
func NewContract(contracts *[]core.SystemContract) *Contract {
	return &Contract{contracts: contracts}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoProposalUpsertOwner
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxRepoProposalUpsertOwner)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	// Get the repo
	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.RepoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	proposal := common2.MakeProposal(spk.Addr().String(), repo, c.tx.ID, c.tx.Value, c.chainHeight)
	proposal.Action = txns.TxTypeRepoProposalUpsertOwner
	proposal.ActionData = map[string]util.Bytes{
		constants.ActionDataKeyAddrs: util.ToBytes(c.tx.Addresses),
		constants.ActionDataKeyVeto:  util.ToBytes(c.tx.Veto),
	}

	// Deduct network fee + proposal fee from sender
	totalFee := c.tx.Fee.Decimal().Add(c.tx.Value.Decimal())
	common.DebitAccount(c, spk, totalFee, c.chainHeight)

	// Attempt to apply the proposal action
	applied, err := common2.MaybeApplyProposal(&common2.ApplyProposalArgs{
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

	// Index the proposal against its end height so it can be tracked
	// and finalized at that height.
	if err = repoKeeper.IndexProposalEnd(c.tx.RepoName, proposal.ID, proposal.EndAt.UInt64()); err != nil {
		return errors.Wrap(err, common.ErrFailedToIndexProposal)
	}

update:
	repoKeeper.Update(c.tx.RepoName, repo)
	return nil
}

// Apply applies the proposal
func (c *Contract) Apply(args *core.ProposalApplyArgs) error {

	// Get the action data
	ad := args.Proposal.GetActionData()
	var targetAddrs []string
	util.ToObject(ad[constants.ActionDataKeyAddrs], &targetAddrs)
	var veto bool
	util.ToObject(ad[constants.ActionDataKeyVeto], &veto)

	// Register new repo owner iif the target address does not
	// already exist as an owner. If it exists, just update select fields.
	for _, address := range targetAddrs {
		existingOwner := args.Repo.Owners.Get(address)
		if existingOwner != nil {
			existingOwner.Veto = veto
			continue
		}

		args.Repo.AddOwner(address, &state.RepoOwner{
			Creator:  false,
			JoinedAt: util.UInt64(args.ChainHeight) + 1,
			Veto:     veto,
		})
	}

	return nil
}
