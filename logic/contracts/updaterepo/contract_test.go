package updaterepo_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts"
	"gitlab.com/makeos/mosdef/logic/contracts/updaterepo"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("UpdateRepoContract", func() {
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
			ct := updaterepo.NewContract(nil)
			Expect(ct.CanExec(txns.TxTypeRepoProposalUpdate)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeHostTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", DelegatorCommission: 10})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Gov.Voter = state.VoterOwner
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				config := &state.RepoConfig{
					Gov: &state.RepoConfigGovernance{PropDuration: 1000},
				}

				err = updaterepo.NewContract(&contracts.SystemContracts).Init(logic, &txns.TxRepoProposalUpdate{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{ID: propID, Value: proposalFee, RepoName: repoName},
					Config:           config.ToBasicMap(),
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
				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(1)))
			})

			Specify("that config is updated", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Config).ToNot(Equal(repoUpd.Config))
				Expect(repo.Config.Gov.PropDuration.UInt64()).To(Equal(uint64(1000)))
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

		When("sender is not the only owner", func() {
			repoName := "repo"
			curHeight := uint64(0)
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				config := &state.RepoConfig{
					Gov: &state.RepoConfigGovernance{PropDuration: 1000},
				}
				err = updaterepo.NewContract(&contracts.SystemContracts).Init(logic, &txns.TxRepoProposalUpdate{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{ID: propID, Value: proposalFee, RepoName: repoName},
					Config:           config.ToBasicMap(),
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
				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeFalse())
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(0)))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), curHeight)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the proposal fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get(propID).Fees[sender.Addr().String()]).To(Equal("1"))
			})

			Specify("that the proposal was indexed against its end height", func() {
				res := logic.RepoKeeper().GetProposalsEndingAt(repoUpd.Config.Gov.PropDuration.UInt64() + curHeight + 1)
				Expect(res).To(HaveLen(1))
			})
		})

		When("repo config has proposal deposit fee duration set to a non-zero number", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.Config.Gov.PropDuration = 1000
				repoUpd.Config.Gov.PropFeeDepositDur = 100
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				config := &state.RepoConfig{
					Gov: &state.RepoConfigGovernance{PropDuration: 2000},
				}
				err = updaterepo.NewContract(&contracts.SystemContracts).Init(logic, &txns.TxRepoProposalUpdate{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{ID: propID, Value: proposalFee, RepoName: repoName},
					Config:           config.ToBasicMap(),
				}, 200).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal with expected `endAt` and `feeDepEndAt` values", func() {
				repo := logic.RepoKeeper().GetNoPopulate(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).FeeDepositEndAt.UInt64()).To(Equal(uint64(301)))
				Expect(repo.Proposals.Get(propID).EndAt.UInt64()).To(Equal(uint64(1301)))
			})
		})
	})

	Describe(".Apply", func() {
		var err error
		var repo *state.Repository

		BeforeEach(func() {
			repo = state.BareRepository()
			repo.Config = state.DefaultRepoConfig
		})

		When("update config object is empty", func() {
			It("should not change the config", func() {
				proposal := &state.RepoProposal{
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyCFG: util.ToBytes((&state.RepoConfig{}).ToBasicMap()),
					},
				}
				err = updaterepo.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repo,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
				Expect(repo.Config).To(Equal(state.DefaultRepoConfig))
			})
		})

		When("update config object is not empty", func() {
			It("should change the config", func() {
				cfg := &state.RepoConfig{Gov: &state.RepoConfigGovernance{PropQuorum: 120, PropDuration: 100}}
				proposal := &state.RepoProposal{
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyCFG: util.ToBytes(cfg.ToBasicMap()),
					},
				}
				err = updaterepo.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repo,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
				Expect(repo.Config.Gov.PropQuorum).To(Equal(float64(120)))
				Expect(repo.Config.Gov.PropDuration.UInt64()).To(Equal(uint64(100)))
			})
		})
	})
})
