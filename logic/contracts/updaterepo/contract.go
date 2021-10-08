package updaterepo

import (
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/logic/contracts/common"
	"github.com/make-os/kit/logic/proposals"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/pkg/errors"
)

// Contract implements core.ProposalContract. It is a system contract that
// creates a proposal to update a repository.
type Contract struct {
	core.Keepers
	tx          *txns.TxRepoProposalUpdate
	chainHeight uint64
	contracts   *[]core.SystemContract
}

// NewContract creates a new instance of Contract
func NewContract(contracts *[]core.SystemContract) *Contract {
	return &Contract{contracts: contracts}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoProposalUpdate
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxRepoProposalUpdate)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	// Get the repo
	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.RepoName)

	// Create a proposal
	spk, _ := ed25519.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	proposal := proposals.MakeProposal(spk.Addr().String(), repo, c.tx.ID, c.tx.Value, c.chainHeight)
	proposal.Action = txns.TxTypeRepoProposalUpdate
	if len(c.tx.Config) > 0 {
		proposal.ActionData[constants.ActionDataKeyCFG] = util.ToBytes(c.tx.Config)
	}
	if c.tx != nil && c.tx.Description != "" {
		proposal.ActionData[constants.ActionDataKeyDescription] = util.ToBytes(c.tx.Description)
	}

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

	// Index the proposal against its end height so that it
	// can be tracked and finalized at that height.
	if err = repoKeeper.IndexProposalEnd(c.tx.RepoName, proposal.ID, proposal.EndAt.UInt64()); err != nil {
		return errors.Wrap(err, common.ErrFailedToIndexProposal)
	}

update:
	repoKeeper.Update(c.tx.RepoName, repo)
	return nil
}

// Apply applies the proposal action
func (c *Contract) Apply(args *core.ProposalApplyArgs) error {
	var cfgUpd map[string]interface{}

	// Update config if an update exists
	actDataCfg := args.Proposal.GetActionData()[constants.ActionDataKeyCFG]
	if len(actDataCfg) > 0 {
		if err := util.ToObject(actDataCfg, &cfgUpd); err != nil {
			return err
		}
		if err := args.Repo.Config.Merge(cfgUpd); err != nil {
			return err
		}
	}

	// Update description if update exist
	actDataDesc := args.Proposal.GetActionData()[constants.ActionDataKeyDescription]
	if len(actDataDesc) > 0 {
		if err := util.ToObject(actDataDesc, &args.Repo.Description); err != nil {
			return err
		}
	}

	return nil
}
