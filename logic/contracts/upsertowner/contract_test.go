package upsertowner_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts"
	"gitlab.com/makeos/mosdef/logic/contracts/upsertowner"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("UpsertOwnerContract", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)
	var key2 = crypto.NewKeyFromIntSeed(2)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = logic2.New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CanExec", func() {
		It("should return true when able to execute tx type", func() {
			ct := upsertowner.NewContract(nil)
			Expect(ct.CanExec(core.TxTypeRepoProposalUpsertOwner)).To(BeTrue())
			Expect(ct.CanExec(core.TxTypeHostTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             "10",
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Governance.Voter = state.VoterOwner
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			propID := "1"
			address := []string{"owner_address"}
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)
				err = upsertowner.NewContract(contracts.SystemContracts).Init(logic, &core.TxRepoProposalUpsertOwner{
					TxCommon:         &core.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &core.TxProposalCommon{ProposalID: propID, Value: proposalFee, RepoName: repoName},
					Addresses:        address,
					Veto:             false,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Has(propID)).To(BeTrue())
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(1)))
			})

			Specify("that new owner was added", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Owners).To(HaveLen(2))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the proposal fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get(propID).Fees[sender.Addr().String()]).To(Equal("1"))
			})
		})

		When("sender is the only owner and there are multiple addresses", func() {
			repoName := "repo"
			addresses := []string{"owner_address", "owner_address2"}
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = upsertowner.NewContract(contracts.SystemContracts).Init(logic, &core.TxRepoProposalUpsertOwner{
					TxCommon:         &core.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &core.TxProposalCommon{ProposalID: propID, Value: proposalFee, RepoName: repoName},
					Addresses:        addresses,
					Veto:             false,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(1)))
				Expect(repo.Proposals.Get("1").Outcome).To(Equal(state.ProposalOutcomeAccepted))
			})

			Specify("that three owners were added", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Owners).To(HaveLen(3))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the proposal fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})
		})

		When("sender is not the only owner", func() {
			repoName := "repo"
			addresses := []string{"owner_address"}
			curHeight := uint64(0)
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = upsertowner.NewContract(contracts.SystemContracts).Init(logic, &core.TxRepoProposalUpsertOwner{
					TxCommon:         &core.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &core.TxProposalCommon{ProposalID: propID, Value: proposalFee, RepoName: repoName},
					Addresses:        addresses,
					Veto:             false,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is not finalized or self accepted", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeFalse())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(0)))
			})

			Specify("that no new owner was added", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Owners).To(HaveLen(2))
				Expect(repo.Owners.Has(sender.Addr().String())).To(BeTrue())
				Expect(repo.Owners.Has(key2.Addr().String())).To(BeTrue())
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), curHeight)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the proposal fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})

			Specify("that the proposal was indexed against its end height", func() {
				res := logic.RepoKeeper().GetProposalsEndingAt(repoUpd.Config.Governance.DurOfProposal + curHeight + 1)
				Expect(res).To(HaveLen(1))
			})
		})

		When("repo config has proposal deposit fee duration set to a non-zero number", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			addresses := []string{"owner_address"}
			currentHeight := uint64(200)
			propID := "1"

			BeforeEach(func() {
				repoUpd.Config.Governance.DurOfProposal = 1000
				repoUpd.Config.Governance.FeeDepositDurOfProposal = 100
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = upsertowner.NewContract(contracts.SystemContracts).Init(logic, &core.TxRepoProposalUpsertOwner{
					TxCommon:         &core.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &core.TxProposalCommon{ProposalID: propID, Value: proposalFee, RepoName: repoName},
					Addresses:        addresses,
					Veto:             false,
				}, currentHeight).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal with expected `endAt` and `feeDepEndAt` values", func() {
				repo := logic.RepoKeeper().GetNoPopulate(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").FeeDepositEndAt).To(Equal(uint64(301)))
				Expect(repo.Proposals.Get("1").EndAt).To(Equal(uint64(1301)))
			})
		})
	})

	Describe(".Apply", func() {
		var repoUpd *state.Repository

		BeforeEach(func() {
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
		})

		When("proposal includes 2 addresses", func() {
			BeforeEach(func() {
				proposal := &state.RepoProposal{ActionData: map[string][]byte{
					constants.ActionDataKeyAddrs: util.ToBytes([]string{"addr1", "addr2"}),
				}}
				err = upsertowner.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repoUpd,
					ChainHeight: 0,
				})
			})

			It("should return no err", func() {
				Expect(err).To(BeNil())
			})

			It("should add 2 owners", func() {
				Expect(repoUpd.Owners).To(HaveLen(2))
			})
		})

		When("address (addr1) already exist", func() {
			var existing = &state.RepoOwner{Veto: false, JoinedAt: 100}
			BeforeEach(func() {
				repoUpd.AddOwner("addr1", existing)
				proposal := &state.RepoProposal{ActionData: map[string][]byte{
					constants.ActionDataKeyAddrs: util.ToBytes([]string{"addr1", "addr2"}),
					constants.ActionDataKeyVeto:  util.ToBytes(true),
				}}
				err = upsertowner.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repoUpd,
					ChainHeight: 0,
				})
			})

			It("should return no err", func() {
				Expect(err).To(BeNil())
			})

			It("should add 2 owners", func() {
				Expect(repoUpd.Owners).To(HaveLen(2))
			})

			Specify("that addr1 was updated", func() {
				owner := repoUpd.Owners.Get("addr1")
				Expect(owner.Veto).To(BeTrue())
			})
		})
	})
})
