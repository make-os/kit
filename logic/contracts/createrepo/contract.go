package createrepo

import (
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/logic/contracts/common"
	"github.com/themakeos/lobe/logic/contracts/registerpushkey"
	"github.com/themakeos/lobe/remote/policy"
	"github.com/themakeos/lobe/types"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
)

// CreateRepoContract is a system contract for creating a repository.
// CreateRepoContract implements SystemContract.
type CreateRepoContract struct {
	core.Logic
	tx          *txns.TxRepoCreate
	chainHeight uint64
}

// NewContract creates a new instance of CreateRepoContract
func NewContract() *CreateRepoContract {
	return &CreateRepoContract{}
}

func (c *CreateRepoContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoCreate
}

// Init initialize the contract
func (c *CreateRepoContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxRepoCreate)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *CreateRepoContract) Exec() error {

	spk, _ := crypto.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Create an empty repository
	newRepo := state.BareRepository()

	// Add config
	newRepo.Config = state.NewDefaultRepoConfigFromMap(c.tx.Config)

	// Apply default policies when none is set
	if len(newRepo.Config.Policies) == 0 {
		policy.AddDefaultPolicies(newRepo.Config)
	}

	// Add transaction value to repo balance
	if !c.tx.Value.IsZero() {
		newRepoBal := newRepo.Balance.Decimal().Add(c.tx.Value.Decimal())
		newRepo.Balance = util.String(newRepoBal.String())
	}

	// Add the creator as a contributor if allowed in config.
	if newRepo.Config.Gov.CreatorAsContributor {

		// Register sender's public key as a push key
		if err := registerpushkey.NewContractWithNoSenderUpdate().Init(c.Logic, &txns.TxRegisterPushKey{
			TxCommon:  &txns.TxCommon{SenderPubKey: c.tx.SenderPubKey},
			PublicKey: spk.ToPublicKey(),
			FeeCap:    "0",
		}, c.chainHeight).Exec(); err != nil {
			return err
		}

		newRepo.Contributors[spk.PushAddr().String()] = &state.RepoContributor{
			FeeMode: state.FeeModePusherPays,
			FeeCap:  "0",
			FeeUsed: "0",
		}
	}

	// Register sender as owner only if voter type is VoterOwner or VoterNetStakersAndVetoOwner.
	// If voter type is VoterNetStakersAndVetoOwner, give veto right to sender.
	voterType := newRepo.Config.Gov.Voter
	if voterType == state.VoterOwner || voterType == state.VoterNetStakersAndVetoOwner {
		newRepo.AddOwner(spk.Addr().String(), &state.RepoOwner{
			Creator:  true,
			Veto:     voterType == state.VoterNetStakersAndVetoOwner,
			JoinedAt: util.UInt64(c.chainHeight) + 1,
		})
	}

	// Store the new repo
	c.RepoKeeper().Update(c.tx.Name, newRepo)

	// Deduct fee+value from sender
	deductible := c.tx.Value.Decimal().Add(c.tx.Fee.Decimal())
	common.DebitAccount(c, spk, deductible, c.chainHeight)

	return nil
}
