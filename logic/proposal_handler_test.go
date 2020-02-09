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
				It("should return ProposalOutcomeBelowThreshold", func() {
					proposal.Config.ProposalQuorum = 40
					proposal.Yes = 2
					proposal.No = 2
					res := determineProposalOutcome(logic, proposal, repo, 100)
					Expect(res).To(Equal(types.ProposalOutcomeBelowThreshold))
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

				Specify("that the proposal's repo has balance=120", func() {
					Expect(repo.Balance).To(Equal(util.String("120")))
				})

				Specify("that the helm repo has balance=60", func() {
					repo := logic.RepoKeeper().GetRepo(helmRepo)
					Expect(repo.Balance).To(Equal(util.String("30")))
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
					proposal.Fees = map[string]string{
						"addr":  "100",
						"addr2": "50",
					}
					proposal.ActionData = map[string]interface{}{
						"addresses": "addr",
						"veto":      false,
					}
					proposal.Config.ProposalFeeRefundType = types.ProposalFeeRefundOnAccept

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

				It("should return outcome=ProposalOutcomeBelowThreshold", func() {
					out := getProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(types.ProposalOutcomeBelowThreshold))
				})
			})
		})
	})

	Describe(".maybeProcessProposalFee", func() {
		var proposal *types.RepoProposal
		var addr = util.String("addr1")
		var addr2 = util.String("addr2")
		var repo *types.Repository
		var helmRepoName = "helm"

		BeforeEach(func() {
			logic.SysKeeper().SetHelmRepo(helmRepoName)
		})

		makeMaybeProcessProposalFeeTest := func(refundType types.ProposalFeeRefundType,
			outcome types.ProposalOutcome) error {
			repo = types.BareRepository()
			proposal = types.BareRepoProposal()
			proposal.Config.ProposalFeeRefundType = refundType
			proposal.Fees[addr.String()] = "100"
			proposal.Fees[addr2.String()] = "200"
			proposal.Outcome = outcome
			return maybeProcessProposalFee(proposal.Outcome, logic, proposal, repo)
		}

		When("proposal refund type is ProposalFeeRefundOnAccept", func() {
			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnAccept,
						types.ProposalOutcomeAccepted,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is not 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnAccept,
						types.ProposalOutcomeRejected,
					)
					Expect(err).To(BeNil())
				})

				It("should not add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("0"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("0"))
				})

				It("should distribute fees to target repo and helm", func() {
					Expect(repo.Balance.String()).To(Equal("240"))
					helmRepo := logic.RepoKeeper().GetRepo(helmRepoName)
					Expect(helmRepo.Balance.String()).To(Equal("60"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnAcceptReject", func() {
			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnAcceptReject,
						types.ProposalOutcomeAccepted,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnAcceptAllReject", func() {
			When("proposal outcome is 'rejected with veto'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnAcceptAllReject,
						types.ProposalOutcomeRejectedWithVeto,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected with veto by owners'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnAcceptAllReject,
						types.ProposalOutcomeRejectedWithVetoByOwners,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThreshold", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThreshold,
						types.ProposalOutcomeBelowThreshold,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThresholdAccept", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAccept,
						types.ProposalOutcomeBelowThreshold,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAccept,
						types.ProposalOutcomeAccepted,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThresholdAcceptReject", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptReject,
						types.ProposalOutcomeBelowThreshold,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptReject,
						types.ProposalOutcomeAccepted,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptReject,
						types.ProposalOutcomeRejected,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected with veto'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptReject,
						types.ProposalOutcomeRejectedWithVeto,
					)
					Expect(err).To(BeNil())
				})

				It("should not add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("0"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("0"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThresholdAcceptAllReject", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptAllReject,
						types.ProposalOutcomeBelowThreshold,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptAllReject,
						types.ProposalOutcomeAccepted,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptAllReject,
						types.ProposalOutcomeRejectedWithVetoByOwners,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected with veto'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(
						types.ProposalFeeRefundOnBelowThresholdAcceptAllReject,
						types.ProposalOutcomeRejectedWithVeto,
					)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().GetAccount(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().GetAccount(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})
	})
})
