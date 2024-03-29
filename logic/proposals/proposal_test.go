package proposals_test

import (
	"os"

	"github.com/AlekSi/pointer"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/proposals"
	"github.com/make-os/kit/mocks"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("ProposalHandler", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var key = ed25519.NewKeyFromIntSeed(1)
	var repo *state.Repository

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

	BeforeEach(func() {
		repo = state.BareRepository()
		repo.AddOwner("addr1", &state.RepoOwner{})
		repo.AddOwner("addr2", &state.RepoOwner{})
		repo.AddOwner("addr3", &state.RepoOwner{})
		repo.AddOwner("addr4", &state.RepoOwner{})
		repo.AddOwner("addr5", &state.RepoOwner{})
		repo.AddOwner("addr6", &state.RepoOwner{})
		repo.AddOwner("addr7", &state.RepoOwner{})
		repo.AddOwner("addr8", &state.RepoOwner{})
		repo.AddOwner("addr9", &state.RepoOwner{})
		repo.AddOwner("addr10", &state.RepoOwner{})
	})

	Describe(".DetermineProposalOutcome", func() {

		When("repo proposer is ProposerOwner and there are 10 owners", func() {
			var proposal *state.RepoProposal

			BeforeEach(func() {
				proposal = &state.RepoProposal{
					Config: state.MakeDefaultRepoConfig().Gov,
				}
				proposal.Config.Voter = state.VoterOwner.Ptr()
				proposal.Creator = key.Addr().String()
			})

			When("proposal quorum is 40% and total votes received is 3", func() {
				It("should return ProposalOutcomeQuorumNotMet", func() {
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Yes = 2
					proposal.No = 1
					res := proposals.DetermineProposalOutcome(logic, proposal, repo, 100)
					Expect(res).To(Equal(state.ProposalOutcomeQuorumNotMet))
				})
			})

			When("proposal quorum is 40% and total votes received is 4", func() {
				It("should return ProposalOutcomeBelowThreshold", func() {
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Yes = 2
					proposal.No = 2
					res := proposals.DetermineProposalOutcome(logic, proposal, repo, 100)
					Expect(res).To(Equal(state.ProposalOutcomeBelowThreshold))
				})
			})
		})

		Context("proposer max join height is set", func() {
			When("repo proposer is ProposerOwner and there are 10 owners with 2 above proposal proposer max join height", func() {
				var proposal *state.RepoProposal

				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.PowerAge = 100
					repo.Owners.Get("addr3").JoinedAt = 200
					repo.Owners.Get("addr4").JoinedAt = 210
				})

				When("proposal quorum is 40% and total votes received is 2", func() {
					It("should return ProposalOutcomeQuorumNotMet", func() {
						proposal.Config.PropQuorum = pointer.ToString("40")
						proposal.Yes = 1
						proposal.No = 1
						out := proposals.DetermineProposalOutcome(logic, proposal, repo, 100)
						Expect(out).To(Equal(state.ProposalOutcomeQuorumNotMet))
					})
				})

				When("proposal quorum is 40%, threshold is 51% and total votes received is 3", func() {
					It("should return ProposalOutcomeAccepted", func() {
						proposal.Config.PropQuorum = pointer.ToString("40")
						proposal.Config.PropThreshold = pointer.ToString("51")
						proposal.Yes = 2
						proposal.No = 1
						out := proposals.DetermineProposalOutcome(logic, proposal, repo, 100)
						Expect(out).To(Equal(state.ProposalOutcomeAccepted))
					})
				})
			})
		})
	})

	Describe(".common.MaybeApplyProposal", func() {
		When("the proposal has already been finalized", func() {
			It("should return false", func() {
				proposal := &state.RepoProposal{}
				proposal.Outcome = state.ProposalOutcomeAccepted
				repo := state.BareRepository()
				args := &proposals.ApplyProposalArgs{Keepers: logic, Proposal: proposal, Repo: repo, ChainHeight: 0}
				applied, err := proposals.MaybeApplyProposal(args)
				Expect(err).To(BeNil())
				Expect(applied).To(BeFalse())
			})
		})

		When("proposal fee deposit is enabled but not enough fees where deposited", func() {
			var proposal *state.RepoProposal
			var repo *state.Repository

			BeforeEach(func() {
				govCfg := state.MakeDefaultRepoConfig().Gov
				govCfg.PropFee = pointer.ToString("1")
				proposal = &state.RepoProposal{
					Config:          govCfg,
					FeeDepositEndAt: 100,
					Fees:            map[string]string{},
				}
				repo = state.BareRepository()
				repo.AddOwner(key.Addr().String(), &state.RepoOwner{})
			})

			It("should return true and proposal outcome = ProposalOutcomeInsufficientDeposit", func() {
				args := &proposals.ApplyProposalArgs{Keepers: logic, Proposal: proposal, Repo: repo, ChainHeight: 101}
				applied, err := proposals.MaybeApplyProposal(args)
				Expect(err).To(BeNil())
				Expect(applied).To(BeFalse())
				Expect(proposal.Outcome).To(Equal(state.ProposalOutcomeInsufficientDeposit))
			})
		})

		When("the proposal type is ProposerOwner and the sender is the only owner of the repo and creator of the proposal", func() {
			var proposal *state.RepoProposal
			var repo *state.Repository

			BeforeEach(func() {
				proposal = &state.RepoProposal{
					Config: state.MakeDefaultRepoConfig().Gov,
				}
				proposal.Config.Voter = state.VoterOwner.Ptr()
				proposal.Creator = key.Addr().String()
				proposal.Action = txns.TxTypeRepoProposalUpsertOwner
				proposal.ActionData = map[string]util.Bytes{
					"addresses": util.ToBytes("addr"),
					"veto":      util.ToBytes(false),
				}
				repo = state.BareRepository()
				repo.AddOwner(key.Addr().String(), &state.RepoOwner{})
			})

			It("should return true and proposal outcome = ProposalOutcomeAccepted", func() {
				args := &proposals.ApplyProposalArgs{Keepers: logic, Proposal: proposal, Repo: repo, ChainHeight: 0}
				applied, err := proposals.MaybeApplyProposal(args)
				Expect(err).To(BeNil())
				Expect(applied).To(BeTrue())
				Expect(proposal.Outcome).To(Equal(state.ProposalOutcomeAccepted))
			})
		})

		When("proposal's end height is a future height", func() {
			It("should return false", func() {
				proposal := state.BareRepoProposal()
				proposal.Config.Voter = state.VoterOwner.Ptr()
				proposal.Creator = key.Addr().String()
				proposal.EndAt = 100
				repo := state.BareRepository()
				args := &proposals.ApplyProposalArgs{Keepers: logic, Proposal: proposal, Repo: repo, ChainHeight: 0}
				applied, err := proposals.MaybeApplyProposal(args)
				Expect(err).To(BeNil())
				Expect(applied).To(BeFalse())
			})
		})

		Context("check if proposal fees were shared", func() {
			When("proposal fee is non-refundable", func() {
				var proposal *state.RepoProposal
				var repo *state.Repository
				var helmRepo = "helm-repo"

				BeforeEach(func() {
					err := logic.SysKeeper().SetHelmRepo(helmRepo)
					Expect(err).To(BeNil())

					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Action = txns.TxTypeRepoProposalUpsertOwner
					proposal.Fees = map[string]string{
						"addr":  "100",
						"addr2": "50",
					}
					proposal.ActionData = map[string]util.Bytes{
						"addresses": util.ToBytes("addr"),
						"veto":      util.ToBytes(false),
					}
					repo = state.BareRepository()
					repo.AddOwner(key.Addr().String(), &state.RepoOwner{})
					args := &proposals.ApplyProposalArgs{Keepers: logic, Proposal: proposal, Repo: repo, ChainHeight: 0}
					applied, err := proposals.MaybeApplyProposal(args)
					Expect(err).To(BeNil())
					Expect(applied).To(BeTrue())
				})

				Specify("that the proposal's repo has balance=120", func() {
					Expect(repo.Balance).To(Equal(util.String("120")))
				})

				Specify("that the helm repo has balance=60", func() {
					repo := logic.RepoKeeper().Get(helmRepo)
					Expect(repo.Balance).To(Equal(util.String("30")))
				})
			})

			When("proposal fee is refundable", func() {
				var proposal *state.RepoProposal
				var repo *state.Repository
				var helmRepo = "helm-repo"

				BeforeEach(func() {
					err := logic.SysKeeper().SetHelmRepo(helmRepo)
					Expect(err).To(BeNil())

					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Action = txns.TxTypeRepoProposalUpsertOwner
					proposal.Fees = map[string]string{
						"addr":  "100",
						"addr2": "50",
					}
					proposal.ActionData = map[string]util.Bytes{
						"addresses": util.ToBytes("addr"),
						"veto":      util.ToBytes(false),
					}
					proposal.Config.PropFeeRefundType = state.ProposalFeeRefundOnAccept.Ptr()

					repo = state.BareRepository()
					repo.AddOwner(key.Addr().String(), &state.RepoOwner{})
					args := &proposals.ApplyProposalArgs{Keepers: logic, Proposal: proposal, Repo: repo, ChainHeight: 0}
					applied, err := proposals.MaybeApplyProposal(args)
					Expect(err).To(BeNil())
					Expect(applied).To(BeTrue())
				})

				Specify("that the proposal's repo has balance=0", func() {
					Expect(repo.Balance).To(Equal(util.String("0")))
				})

				Specify("that the helm repo has balance=0", func() {
					repo := logic.RepoKeeper().Get(helmRepo)
					Expect(repo.Balance).To(Equal(util.String("0")))
				})
			})
		})
	})

	Describe(".GetProposalOutcome", func() {
		When("proposer type is ProposerNetStakeholders", func() {
			var proposal *state.RepoProposal

			BeforeEach(func() {
				proposal = &state.RepoProposal{
					Config: state.MakeDefaultRepoConfig().Gov,
				}
				proposal.Config.Voter = state.VoterNetStakers.Ptr()
				proposal.Creator = key.Addr().String()
				proposal.Config.PropQuorum = pointer.ToString("40")
				proposal.Yes = 100
				proposal.No = 100
				proposal.NoWithVeto = 50

				mockTickMgr := mocks.NewMockTicketManager(ctrl)
				mockTickMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(1000), nil)
				logic.SetTicketManager(mockTickMgr)
			})

			It("should return outcome=ProposalOutcomeQuorumNotMet", func() {
				out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
				Expect(out).To(Equal(state.ProposalOutcomeQuorumNotMet))
			})
		})

		When("proposer type is ProposerOwner", func() {
			When("quorum is not reached", func() {
				var proposal *state.RepoProposal

				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Yes = 1
					proposal.No = 1
					proposal.NoWithVeto = 1
				})

				It("should return outcome=ProposalOutcomeQuorumNotMet", func() {
					out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(state.ProposalOutcomeQuorumNotMet))
				})
			})

			When("NoWithVeto quorum is reached", func() {
				var proposal *state.RepoProposal

				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Config.PropVetoQuorum = pointer.ToString("10")
					proposal.Yes = 5
					proposal.No = 4
					proposal.NoWithVeto = 1
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(state.ProposalOutcomeRejectedWithVeto))
				})
			})

			When("NoWithVetoByOwners quorum is reached", func() {
				var proposal *state.RepoProposal

				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterNetStakersAndVetoOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Config.PropVetoQuorum = pointer.ToString("10")
					proposal.Yes = 700
					proposal.No = 4
					proposal.NoWithVeto = 1
					proposal.Config.PropVetoOwnersQuorum = pointer.ToString("40")
					proposal.NoWithVetoByOwners = 5

					mockTickMgr := mocks.NewMockTicketManager(ctrl)
					mockTickMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(1000), nil)
					logic.SetTicketManager(mockTickMgr)
				})

				It("should return outcome=ProposalOutcomeRejectedWithVetoByOwners", func() {
					out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(state.ProposalOutcomeRejectedWithVetoByOwners))
				})
			})

			When("NoWithVeto quorum is unset but there is at least 1 NoWithVeto vote", func() {
				var proposal *state.RepoProposal

				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Config.PropVetoQuorum = pointer.ToString("0")
					proposal.Yes = 5
					proposal.No = 4
					proposal.NoWithVeto = 1
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(state.ProposalOutcomeRejectedWithVeto))
				})
			})

			When("Yes threshold is reached", func() {
				var proposal *state.RepoProposal

				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Config.PropVetoQuorum = pointer.ToString("10")
					proposal.Config.PropThreshold = pointer.ToString("51")
					proposal.Yes = 6
					proposal.No = 4
					proposal.NoWithVeto = 0
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(state.ProposalOutcomeAccepted))
				})
			})

			When("No threshold is reached", func() {
				var proposal *state.RepoProposal
				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Config.PropVetoQuorum = pointer.ToString("10")
					proposal.Config.PropThreshold = pointer.ToString("51")
					proposal.Yes = 4
					proposal.No = 6
					proposal.NoWithVeto = 0
				})

				It("should return outcome=ProposalOutcomeRejectedWithVeto", func() {
					out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(state.ProposalOutcomeRejected))
				})
			})

			When("some either Yes or No votes reached the threshold", func() {
				var proposal *state.RepoProposal

				BeforeEach(func() {
					proposal = &state.RepoProposal{Config: state.MakeDefaultRepoConfig().Gov}
					proposal.Config.Voter = state.VoterOwner.Ptr()
					proposal.Creator = key.Addr().String()
					proposal.Config.PropQuorum = pointer.ToString("40")
					proposal.Config.PropVetoQuorum = pointer.ToString("10")
					proposal.Config.PropThreshold = pointer.ToString("51")
					proposal.Yes = 4
					proposal.No = 4
					proposal.NoWithVeto = 0
				})

				It("should return outcome=ProposalOutcomeBelowThreshold", func() {
					out := proposals.GetProposalOutcome(logic.GetTicketManager(), proposal, repo)
					Expect(out).To(Equal(state.ProposalOutcomeBelowThreshold))
				})
			})
		})
	})

	Describe(".MaybeProcessProposalFee", func() {
		var proposal *state.RepoProposal
		var addr = identifier.Address("addr1")
		var addr2 = identifier.Address("addr2")
		var repo *state.Repository
		var helmRepoName = "helm"

		BeforeEach(func() {
			logic.SysKeeper().SetHelmRepo(helmRepoName)
		})

		makeMaybeProcessProposalFeeTest := func(refundType state.PropFeeRefundType,
			outcome state.ProposalOutcome) error {
			repo = state.BareRepository()
			proposal = state.BareRepoProposal()
			proposal.Config.PropFeeRefundType = refundType.Ptr()
			proposal.Fees[addr.String()] = "100"
			proposal.Fees[addr2.String()] = "200"
			proposal.Outcome = outcome
			return proposals.MaybeProcessProposalFee(proposal.Outcome, logic, proposal, repo)
		}

		When("proposal refund type is ProposalFeeRefundOnAccept", func() {
			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnAccept, state.ProposalOutcomeAccepted)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is not 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnAccept, state.ProposalOutcomeRejected)
					Expect(err).To(BeNil())
				})

				It("should not add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("0"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("0"))
				})

				It("should distribute fees to target repo and helm", func() {
					Expect(repo.Balance.String()).To(Equal("240"))
					helmRepo := logic.RepoKeeper().Get(helmRepoName)
					Expect(helmRepo.Balance.String()).To(Equal("60"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnAcceptReject", func() {
			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnAcceptReject, state.ProposalOutcomeAccepted)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnAcceptAllReject", func() {
			When("proposal outcome is 'rejected with veto'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnAcceptAllReject, state.ProposalOutcomeRejectedWithVeto)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected with veto by owners'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnAcceptAllReject, state.ProposalOutcomeRejectedWithVetoByOwners)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThreshold", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThreshold, state.ProposalOutcomeBelowThreshold)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThresholdAccept", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAccept, state.ProposalOutcomeBelowThreshold)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAccept, state.ProposalOutcomeAccepted)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThresholdAcceptReject", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptReject, state.ProposalOutcomeBelowThreshold)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptReject, state.ProposalOutcomeAccepted)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptReject, state.ProposalOutcomeRejected)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected with veto'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptReject, state.ProposalOutcomeRejectedWithVeto)
					Expect(err).To(BeNil())
				})

				It("should not add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("0"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("0"))
				})
			})
		})

		When("proposal refund type is ProposalFeeRefundOnBelowThresholdAcceptAllReject", func() {
			When("proposal outcome is a 'tie'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptAllReject, state.ProposalOutcomeBelowThreshold)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'accepted'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptAllReject, state.ProposalOutcomeAccepted)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptAllReject, state.ProposalOutcomeRejectedWithVetoByOwners)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})

			When("proposal outcome is 'rejected with veto'", func() {
				BeforeEach(func() {
					err = makeMaybeProcessProposalFeeTest(state.ProposalFeeRefundOnBelowThresholdAcceptAllReject, state.ProposalOutcomeRejectedWithVeto)
					Expect(err).To(BeNil())
				})

				It("should add fees back to senders accounts", func() {
					Expect(logic.AccountKeeper().Get(addr).Balance.String()).To(Equal("100"))
					Expect(logic.AccountKeeper().Get(addr2).Balance.String()).To(Equal("200"))
				})
			})
		})
	})
})
