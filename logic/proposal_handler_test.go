package logic

import (
	"os"

	"github.com/golang/mock/gomock"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProposalHandler", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)
	var repo *types.Repository

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	BeforeEach(func() {
		repo = types.BareRepository()
		repo.AddOwner("addr1", &types.RepoOwner{})
		repo.AddOwner("addr2", &types.RepoOwner{})
		repo.AddOwner("addr3", &types.RepoOwner{})
		repo.AddOwner("addr4", &types.RepoOwner{})
		repo.AddOwner("addr5", &types.RepoOwner{})
		repo.AddOwner("addr6", &types.RepoOwner{})
		repo.AddOwner("addr7", &types.RepoOwner{})
		repo.AddOwner("addr8", &types.RepoOwner{})
		repo.AddOwner("addr9", &types.RepoOwner{})
		repo.AddOwner("addr10", &types.RepoOwner{})
	})

	Describe(".determineProposalOutcome", func() {

		When("repo proposee is ProposeeOwner and there are 10 owners", func() {
			var proposal *types.RepoProposal

			BeforeEach(func() {
				proposal = &types.RepoProposal{
					Config: types.MakeDefaultRepoConfig().Governace,
				}
				proposal.Config.ProposalProposee = types.ProposeeOwner
				proposal.Creator = key.Addr().String()
			})

			When("proposal quorum is 40% and total votes received is 3", func() {
				It("should return ProposalOutcomeQuorumNotMet", func() {
					proposal.Config.ProposalQuorum = 40
					proposal.Yes = 2
					proposal.No = 1
					res := determineProposalOutcome(logic, proposal, repo, 100)
					Expect(res).To(Equal(types.ProposalOutcomeQuorumNotMet))
				})
			})

			When("proposal quorum is 40% and total votes received is 4", func() {
				It("should return ProposalOutcomeTie", func() {
					proposal.Config.ProposalQuorum = 40
					proposal.Yes = 2
					proposal.No = 2
					res := determineProposalOutcome(logic, proposal, repo, 100)
					Expect(res).To(Equal(types.ProposalOutcomeTie))
				})
			})
		})

		Context("proposee max join height is set", func() {
			When("repo proposee is ProposeeOwner and there are 10 owners with 2 above proposal proposee max join height", func() {
				var proposal *types.RepoProposal

				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.ProposeeMaxJoinHeight = 100
					repo.Owners.Get("addr3").JoinedAt = 200
					repo.Owners.Get("addr4").JoinedAt = 210
				})

				When("proposal quorum is 40% and total votes received is 2", func() {
					It("should return ProposalOutcomeQuorumNotMet", func() {
						proposal.Config.ProposalQuorum = 40
						proposal.Yes = 1
						proposal.No = 1
						out := determineProposalOutcome(logic, proposal, repo, 100)
						Expect(out).To(Equal(types.ProposalOutcomeQuorumNotMet))
					})
				})

				When("proposal quorum is 40%, threshold is 51% and total votes received is 3", func() {
					It("should return ProposalOutcomeAccepted", func() {
						proposal.Config.ProposalQuorum = 40
						proposal.Config.ProposalThreshold = 51
						proposal.Yes = 2
						proposal.No = 1
						out := determineProposalOutcome(logic, proposal, repo, 100)
						Expect(out).To(Equal(types.ProposalOutcomeAccepted))
					})
				})
			})
		})
	})

	Describe(".maybeApplyProposal", func() {
		When("the proposal has already been finalized", func() {
			It("should return false", func() {
				proposal := &types.RepoProposal{}
				proposal.Outcome = types.ProposalOutcomeAccepted
				repo := types.BareRepository()
				applied, err := maybeApplyProposal(logic, proposal, repo, 0)
				Expect(err).To(BeNil())
				Expect(applied).To(BeFalse())
			})
		})

		When("the proposal type is ProposeeOwner and the sender is the only owner of the repo and creator of the proposal", func() {
			var proposal *types.RepoProposal
			var repo *types.Repository

			BeforeEach(func() {
				proposal = &types.RepoProposal{
					Config: types.MakeDefaultRepoConfig().Governace,
				}
				proposal.Config.ProposalProposee = types.ProposeeOwner
				proposal.Creator = key.Addr().String()
				proposal.Action = types.ProposalActionAddOwner
				proposal.ActionData = map[string]interface{}{
					"addresses": "addr",
					"veto":      false,
				}
				repo = types.BareRepository()
				repo.AddOwner(key.Addr().String(), &types.RepoOwner{})
			})

			It("should return true and proposal outcome = ProposalOutcomeAccepted", func() {
				applied, err := maybeApplyProposal(logic, proposal, repo, 0)
				Expect(err).To(BeNil())
				Expect(applied).To(BeTrue())
				Expect(proposal.Outcome).To(Equal(types.ProposalOutcomeAccepted))
			})
		})

		When("proposal's end height is a future height", func() {
			It("should return false", func() {
				proposal := types.BareRepoProposal()
				proposal.Config.ProposalProposee = types.ProposeeOwner
				proposal.Creator = key.Addr().String()
				proposal.EndAt = 100
				repo := types.BareRepository()
				applied, err := maybeApplyProposal(logic, proposal, repo, 0)
				Expect(err).To(BeNil())
				Expect(applied).To(BeFalse())
			})
		})

		Context("check if proposal fees were shared", func() {
			When("proposal fee is non-refundable", func() {
				var proposal *types.RepoProposal
				var repo *types.Repository
				var helmRepo = "helm-repo"

				BeforeEach(func() {
					err := logic.SysKeeper().SetHelmRepo(helmRepo)
					Expect(err).To(BeNil())

					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Action = types.ProposalActionAddOwner
					proposal.Fees = map[string]string{
						"addr":  "100",
						"addr2": "50",
					}
					proposal.ActionData = map[string]interface{}{
						"addresses": "addr",
						"veto":      false,
					}
					repo = types.BareRepository()
					repo.AddOwner(key.Addr().String(), &types.RepoOwner{})
					applied, err := maybeApplyProposal(logic, proposal, repo, 0)
					Expect(err).To(BeNil())
					Expect(applied).To(BeTrue())
				})

				Specify("that the proposal's repo has balance=90", func() {
					Expect(repo.Balance).To(Equal(util.String("90")))
				})

				Specify("that the helm repo has balance=60", func() {
					repo := logic.RepoKeeper().GetRepo(helmRepo)
					Expect(repo.Balance).To(Equal(util.String("60")))
				})
			})

			When("proposal fee is refundable", func() {
				var proposal *types.RepoProposal
				var repo *types.Repository
				var helmRepo = "helm-repo"

				BeforeEach(func() {
					err := logic.SysKeeper().SetHelmRepo(helmRepo)
					Expect(err).To(BeNil())

					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Action = types.ProposalActionAddOwner
					proposal.Config.ProposalFeeRefund = true
					proposal.Fees = map[string]string{
						"addr":  "100",
						"addr2": "50",
					}
					proposal.ActionData = map[string]interface{}{
						"addresses": "addr",
						"veto":      false,
					}
					repo = types.BareRepository()
					repo.AddOwner(key.Addr().String(), &types.RepoOwner{})
					applied, err := maybeApplyProposal(logic, proposal, repo, 0)
					Expect(err).To(BeNil())
					Expect(applied).To(BeTrue())
				})

				Specify("that the proposal's repo has balance=0", func() {
					Expect(repo.Balance).To(Equal(util.String("0")))
				})

				Specify("that the helm repo has balance=0", func() {
					repo := logic.RepoKeeper().GetRepo(helmRepo)
					Expect(repo.Balance).To(Equal(util.String("0")))
				})
			})
		})
	})

	Describe(".getProposalOutcome", func() {
		When("proposee type is ProposeeNetStakeholders", func() {
			var proposal *types.RepoProposal

			BeforeEach(func() {
				proposal = &types.RepoProposal{
					Config: types.MakeDefaultRepoConfig().Governace,
				}
				proposal.Config.ProposalProposee = types.ProposeeNetStakeholders
				proposal.Creator = key.Addr().String()
				proposal.Config.ProposalQuorum = 40
				proposal.Yes = 100
				proposal.No = 100
				proposal.NoWithVeto = 50

				mockTickMgr := mocks.NewMockTicketManager(ctrl)
				mockTickMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(1000), nil)
				logic.SetTicketManager(mockTickMgr)
			})

			It("should return outcome=ProposalOutcomeQuorumNotMet", func() {
				out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
				Expect(out).To(Equal(types.ProposalOutcomeQuorumNotMet))
			})
		})

		When("proposee type is ProposeeOwner", func() {
			When("quorum is not reached", func() {
				var proposal *types.RepoProposal

				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Config.ProposalQuorum = 40
					proposal.Yes = 1
					proposal.No = 1
					proposal.NoWithVeto = 1
				})

				It("should return outcome=ProposalOutcomeQuorumNotMet", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeQuorumNotMet))
				})
			})

			When("NoWithVeto quorum is reached", func() {
				var proposal *types.RepoProposal

				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Config.ProposalQuorum = 40
					proposal.Config.ProposalVetoQuorum = 10
					proposal.Yes = 5
					proposal.No = 4
					proposal.NoWithVeto = 1
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeRejectedWithVeto))
				})
			})

			When("NoWithVetoByOwners quorum is reached", func() {
				var proposal *types.RepoProposal

				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeNetStakeholdersAndVetoOwner
					proposal.Creator = key.Addr().String()
					proposal.Config.ProposalQuorum = 40
					proposal.Config.ProposalVetoQuorum = 10
					proposal.Yes = 700
					proposal.No = 4
					proposal.NoWithVeto = 1
					proposal.Config.ProposalVetoOwnersQuorum = 40
					proposal.NoWithVetoByOwners = 5

					mockTickMgr := mocks.NewMockTicketManager(ctrl)
					mockTickMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(1000), nil)
					logic.SetTicketManager(mockTickMgr)
				})

				It("should return outcome=ProposalOutcomeRejectedWithVetoByOwners", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeRejectedWithVetoByOwners))
				})
			})

			When("NoWithVeto quorum is unset but there is at least 1 NoWithVeto vote", func() {
				var proposal *types.RepoProposal

				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Config.ProposalQuorum = 40
					proposal.Config.ProposalVetoQuorum = 0
					proposal.Yes = 5
					proposal.No = 4
					proposal.NoWithVeto = 1
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeRejectedWithVeto))
				})
			})

			When("Yes threshold is reached", func() {
				var proposal *types.RepoProposal

				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Config.ProposalQuorum = 40
					proposal.Config.ProposalVetoQuorum = 10
					proposal.Config.ProposalThreshold = 51
					proposal.Yes = 6
					proposal.No = 4
					proposal.NoWithVeto = 0
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeAccepted))
				})
			})

			When("No threshold is reached", func() {
				var proposal *types.RepoProposal
				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Config.ProposalQuorum = 40
					proposal.Config.ProposalVetoQuorum = 10
					proposal.Config.ProposalThreshold = 51
					proposal.Yes = 4
					proposal.No = 6
					proposal.NoWithVeto = 0
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeRejected))
				})
			})

			When("some either Yes or No votes reached the threshold", func() {
				var proposal *types.RepoProposal

				BeforeEach(func() {
					proposal = &types.RepoProposal{
						Config: types.MakeDefaultRepoConfig().Governace,
					}
					proposal.Config.ProposalProposee = types.ProposeeOwner
					proposal.Creator = key.Addr().String()
					proposal.Config.ProposalQuorum = 40
					proposal.Config.ProposalVetoQuorum = 10
					proposal.Config.ProposalThreshold = 51
					proposal.Yes = 4
					proposal.No = 4
					proposal.NoWithVeto = 0
				})

				It("should return outcome=ProposalOutcomeTie", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeTie))
				})
			})
		})
	})
})
