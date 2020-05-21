package mergerequest

// MergeRequestContract is a system contract that creates a merge request proposal.
// MergeRequestContract implements SystemContract.
// type MergeRequestContract struct {
// 	core.Logic
// 	tx          *core.TxRepoProposalMergeRequest
// 	chainHeight uint64
// }
//
// // NewContract creates a new instance of MergeRequestContract
// func NewContract() *MergeRequestContract {
// 	return &MergeRequestContract{}
// }
//
// func (c *MergeRequestContract) CanExec(typ types.TxCode) bool {
// 	return typ == core.TxTypeRepoProposalMergeRequest
// }
//
// // Init initialize the contract
// func (c *MergeRequestContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
// 	c.Logic = logic
// 	c.tx = tx.(*core.TxRepoProposalMergeRequest)
// 	c.chainHeight = curChainHeight
// 	return c
// }
//
// // Exec executes the contract
// func (c *MergeRequestContract) Exec() error {
//
// 	// Get the repo
// 	repoKeeper := c.RepoKeeper()
// 	repo := repoKeeper.Get(c.tx.RepoName)
//
// 	// Create a proposal
// 	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
// 	proposal := proposals.MakeProposal(spk, repo, c.tx.ProposalID, c.tx.Value, c.chainHeight)
// 	proposal.Action = core.TxTypeRepoProposalMergeRequest
// 	proposal.ActionData = map[string][]byte{
// 		constants.ActionDataKeyBaseBranch:   util.ToBytes(c.tx.BaseBranch),
// 		constants.ActionDataKeyBaseHash:     util.ToBytes(c.tx.BaseBranchHash),
// 		constants.ActionDataKeyTargetBranch: util.ToBytes(c.tx.TargetBranch),
// 		constants.ActionDataKeyTargetHash:   util.ToBytes(c.tx.TargetBranchHash),
// 	}
//
// 	// Deduct network fee + proposal fee from sender
// 	totalFee := c.tx.Fee.Decimal().Add(c.tx.Value.Decimal())
// 	common.DebitAccount(c, spk, totalFee, c.chainHeight)
//
// 	// Attempt to apply the proposal action
// 	applied, err := proposals.MaybeApplyProposal(&proposals.ApplyProposalArgs{
// 		Keepers:     c,
// 		Proposal:    proposal,
// 		Repo:        repo,
// 		ChainHeight: c.chainHeight,
// 	})
// 	if err != nil {
// 		return errors.Wrap(err, "failed to apply proposal")
// 	} else if applied {
// 		goto update
// 	}
//
// 	// Index the proposal against its end height so it can be tracked and
// 	// finalized at that height.
// 	if err = repoKeeper.IndexProposalEnd(c.tx.RepoName, proposal.ID, proposal.EndAt); err != nil {
// 		return errors.Wrap(err, "failed to index proposal against end height")
// 	}
//
// update:
// 	repoKeeper.Update(c.tx.RepoName, repo)
// 	return nil
// }
