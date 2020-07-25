package registerrepopushkeys

import (
	"github.com/pkg/errors"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/logic/contracts/common"
	"github.com/themakeos/lobe/logic/proposals"
	"github.com/themakeos/lobe/types"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
	crypto2 "github.com/themakeos/lobe/util/crypto"
)

// RegisterRepoPushKeysContract is a system contract that creates a proposal to register
// push keys to a repo. RegisterRepoPushKeysContract implements ProposalContract.
type RegisterRepoPushKeysContract struct {
	core.Logic
	tx          *txns.TxRepoProposalRegisterPushKey
	chainHeight uint64
	contracts   *[]core.SystemContract
}

// NewContract creates a new instance of RegisterRepoPushKeysContract
func NewContract(contracts *[]core.SystemContract) *RegisterRepoPushKeysContract {
	return &RegisterRepoPushKeysContract{contracts: contracts}
}

func (c *RegisterRepoPushKeysContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoProposalRegisterPushKey
}

// Init initialize the contract
func (c *RegisterRepoPushKeysContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxRepoProposalRegisterPushKey)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *RegisterRepoPushKeysContract) Exec() error {

	// Get the repo
	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.RepoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())
	proposal := proposals.MakeProposal(spk.Addr().String(), repo, c.tx.ID, c.tx.Value, c.chainHeight)
	proposal.Action = txns.TxTypeRepoProposalRegisterPushKey
	proposal.ActionData = map[string]util.Bytes{
		constants.ActionDataKeyIDs:      util.ToBytes(c.tx.PushKeys),
		constants.ActionDataKeyPolicies: util.ToBytes(c.tx.Policies),
		constants.ActionDataKeyFeeMode:  util.ToBytes(c.tx.FeeMode),
		constants.ActionDataKeyFeeCap:   util.ToBytes(c.tx.FeeCap.String()),
	}
	if c.tx.Namespace != "" {
		proposal.ActionData[constants.ActionDataKeyNamespace] = util.ToBytes(c.tx.Namespace)
	}
	if c.tx.NamespaceOnly != "" {
		proposal.ActionData[constants.ActionDataKeyNamespaceOnly] = util.ToBytes(c.tx.NamespaceOnly)
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

	// Index the proposal against its end height so it can be tracked and
	// finalized at that height.
	if err = repoKeeper.IndexProposalEnd(c.tx.RepoName, proposal.ID, proposal.EndAt.UInt64()); err != nil {
		return errors.Wrap(err, common.ErrFailedToIndexProposal)
	}

update:
	repoKeeper.Update(c.tx.RepoName, repo)
	return nil
}

// Apply applies the proposal
func (c *RegisterRepoPushKeysContract) Apply(args *core.ProposalApplyArgs) error {
	ad := args.Proposal.GetActionData()

	// Extract the policies.
	var policies []*state.ContributorPolicy
	_ = util.ToObject(ad[constants.ActionDataKeyPolicies], &policies)

	// Extract the push key IDs.
	var pushKeyIDs []string
	_ = util.ToObject(ad[constants.ActionDataKeyIDs], &pushKeyIDs)

	// Extract fee mode and fee cap
	var feeMode state.FeeMode
	_ = util.ToObject(ad[constants.ActionDataKeyFeeMode], &feeMode)
	var feeCap = util.String("0")
	if feeMode == state.FeeModeRepoPaysCapped {
		_ = util.ToObject(ad[constants.ActionDataKeyFeeCap], &feeCap)
	}

	// Get any target namespace.
	var namespace, namespaceOnly, targetNS string
	var ns *state.Namespace
	if _, ok := ad[constants.ActionDataKeyNamespace]; ok {
		util.ToObject(ad[constants.ActionDataKeyNamespace], &namespace)
		targetNS = namespace
	}
	if _, ok := ad[constants.ActionDataKeyNamespaceOnly]; ok {
		util.ToObject(ad[constants.ActionDataKeyNamespaceOnly], &namespaceOnly)
		targetNS = namespaceOnly
	}
	if targetNS != "" {
		ns = args.Keepers.NamespaceKeeper().Get(crypto2.HashNamespace(targetNS))
		if ns.IsNil() {
			panic("namespace must exist")
		}
	}

	// For each push key ID, add a contributor.
	// This will replace any existing contributor with matching push key ID.
	for _, pushKeyID := range pushKeyIDs {

		contributor := &state.BaseContributor{FeeCap: feeCap, FeeUsed: "0", Policies: policies}

		// If namespace is set, add the contributor to the the namespace and
		// then if namespaceOnly is set, continue  to the next push key
		// id after adding a contributor to the namespace
		if namespace != "" || namespaceOnly != "" {
			ns.Contributors[pushKeyID] = contributor
			if namespaceOnly != "" {
				continue
			}
		}

		// Register contributor to the repo
		args.Repo.Contributors[pushKeyID] = &state.RepoContributor{
			FeeMode:  feeMode,
			FeeCap:   contributor.FeeCap,
			FeeUsed:  contributor.FeeUsed,
			Policies: contributor.Policies,
		}
	}

	if ns != nil {
		args.Keepers.NamespaceKeeper().Update(crypto2.HashNamespace(targetNS), ns)
	}

	return nil
}
