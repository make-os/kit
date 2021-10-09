package updaterepo_test

import (
	"os"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts"
	"github.com/make-os/kit/logic/contracts/updaterepo"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestUpdateRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UpdateRepo Suite")
}

var _ = Describe("Contract", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = ed25519.NewKeyFromIntSeed(1)
	var key2 = ed25519.NewKeyFromIntSeed(2)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB()
		logic = logic2.New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&state.BlockInfo{Height: 1})
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
			repoUpd.Config.Gov.Voter = state.VoterOwner.Ptr()
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				config := &state.RepoConfig{
					Gov: &state.RepoConfigGovernance{PropDuration: pointer.ToString("1000")},
				}

				err = updaterepo.NewContract(&contracts.SystemContracts).Init(logic, &txns.TxRepoProposalUpdate{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{ID: propID, Value: proposalFee, RepoName: repoName},
					TxDescription:    &txns.TxDescription{Description: "hello world"},
					Config:           config,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal was immediately finalized", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(1)))
			})

			Specify("that config was updated", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Config).ToNot(Equal(repoUpd.Config))
				Expect(*repo.Config.Gov.PropDuration).To(Equal("1000"))
			})

			Specify("that the description was updated", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Description).To(Equal("hello world"))
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
					Gov: &state.RepoConfigGovernance{PropDuration: pointer.ToString("1000")},
				}
				err = updaterepo.NewContract(&contracts.SystemContracts).Init(logic, &txns.TxRepoProposalUpdate{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{ID: propID, Value: proposalFee, RepoName: repoName},
					TxDescription:    &txns.TxDescription{Description: "hello world"},
					Config:           config,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal was not immediately finalized", func() {
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
				res := logic.RepoKeeper().GetProposalsEndingAt(util.PtrStrToUInt64(repoUpd.Config.Gov.PropDuration) + curHeight + 1)
				Expect(res).To(HaveLen(1))
			})
		})

		When("repo config has proposal deposit fee duration set to a non-zero number", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			propID := "1"

			BeforeEach(func() {
				repoUpd.Config.Gov.PropDuration = pointer.ToString("1000")
				repoUpd.Config.Gov.PropFeeDepositDur = pointer.ToString("100")
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				config := &state.RepoConfig{
					Gov: &state.RepoConfigGovernance{PropDuration: pointer.ToString("2000")},
				}
				err = updaterepo.NewContract(&contracts.SystemContracts).Init(logic, &txns.TxRepoProposalUpdate{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{ID: propID, Value: proposalFee, RepoName: repoName},
					TxDescription:    &txns.TxDescription{Description: "hello world"},
					Config:           config,
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

		When("action data for config is empty", func() {
			It("should not change the config", func() {
				proposal := &state.RepoProposal{
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyCFG: util.ToBytes(&state.RepoConfig{}),
					},
				}
				err = updaterepo.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repo,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
				Expect(repo.Config).To(Equal(state.DefaultRepoConfig))

				// No Action data
				proposal = &state.RepoProposal{
					ActionData: map[string]util.Bytes{},
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

		When("action data for config object is not empty", func() {
			It("should change the config", func() {
				cfg := &state.RepoConfig{Gov: &state.RepoConfigGovernance{
					PropQuorum:   pointer.ToString("120"),
					PropDuration: pointer.ToString("100"),
				}}
				proposal := &state.RepoProposal{
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyCFG: util.ToBytes(cfg),
					},
				}
				err = updaterepo.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repo,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
				Expect(*repo.Config.Gov.PropQuorum).To(Equal("120"))
				Expect(*repo.Config.Gov.PropDuration).To(Equal("100"))
			})
		})

		When("action data for description is empty", func() {
			It("should not change the description", func() {
				proposal := &state.RepoProposal{
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyDescription: util.ToBytes(""),
					},
				}
				err = updaterepo.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repo,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
				Expect(repo.Description).To(BeEmpty())

				// No Action data
				proposal = &state.RepoProposal{
					ActionData: map[string]util.Bytes{},
				}
				err = updaterepo.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repo,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
				Expect(repo.Description).To(BeEmpty())
			})
		})

		When("action data for description is not empty", func() {
			It("should change the description", func() {
				proposal := &state.RepoProposal{
					ActionData: map[string]util.Bytes{
						constants.ActionDataKeyDescription: util.ToBytes("hello world"),
					},
				}
				err = updaterepo.NewContract(nil).Apply(&core.ProposalApplyArgs{
					Proposal:    proposal,
					Repo:        repo,
					ChainHeight: 0,
				})
				Expect(err).To(BeNil())
				Expect(repo.Description).To(Equal("hello world"))
			})
		})
	})
})
