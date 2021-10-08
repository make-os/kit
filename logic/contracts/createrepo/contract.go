package createrepo

import (
	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/logic/contracts/common"
	"github.com/make-os/kit/logic/contracts/registerpushkey"
	"github.com/make-os/kit/remote/policy"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
)

// Contract implements core.SystemContract. It is a system contract for creating a repository.
type Contract struct {
	core.Keepers
	tx          *txns.TxRepoCreate
	chainHeight uint64
}

// NewContract creates a new instance of CreateRepoContract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoCreate
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxRepoCreate)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	spk, _ := ed25519.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Create an empty repository
	newRepo := state.BareRepository()
	newRepo.Description = c.tx.Description
	newRepo.CreatedAt = util.UInt64(c.chainHeight + 1)

	// Add config
	newRepo.Config = state.MakeDefaultRepoConfig()
	if err := newRepo.Config.Merge(c.tx.Config.ToMap()); err != nil {
		return err
	}

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
	if pointer.GetBool(newRepo.Config.Gov.CreatorAsContributor) {

		// Register sender's public key as a push key
		if err := registerpushkey.NewContractWithNoSenderUpdate().Init(c.Keepers, &txns.TxRegisterPushKey{
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
	voterTypeIsNetStakersAndVetoOwners := *voterType == *state.VoterNetStakersAndVetoOwner.Ptr()
	if *voterType == *state.VoterOwner.Ptr() || voterTypeIsNetStakersAndVetoOwners {
		newRepo.AddOwner(spk.Addr().String(), &state.RepoOwner{
			Creator:  true,
			Veto:     voterTypeIsNetStakersAndVetoOwners,
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
