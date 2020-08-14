package voteproposal_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	logic2 "github.com/themakeos/lobe/logic"
	"github.com/themakeos/lobe/logic/contracts/voteproposal"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/storage"
	"github.com/themakeos/lobe/testutil"
	tickettypes "github.com/themakeos/lobe/ticket/types"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
)

var _ = Describe("ProposalVoteContract", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var mockTickMgr *mocks.MockTicketManager
	var sender = crypto.NewKeyFromIntSeed(1)
	var key2 = crypto.NewKeyFromIntSeed(2)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = logic2.New(appDB, stateTreeDB, cfg)
		mockTickMgr = mocks.NewMockTicketManager(ctrl)
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
			ct := voteproposal.NewContract()
			Expect(ct.CanExec(txns.TxTypeRepoProposalVote)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeRegisterPushKey)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repoUpd *state.Repository
		var propID = "proposer_id"
		var repoName = "repo"

		BeforeEach(func() {
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             "10",
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("proposal tally method is ProposalTallyMethodIdentity", func() {
			BeforeEach(func() {
				repoUpd.Config.Gov.Voter = state.VoterOwner
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodIdentity
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{Config: repoUpd.Config.Gov, Yes: 1}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 1", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(2)))
			})

			Context("with two votes; vote 1 = NoWithVeto, vote 2=Yes", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodIdentity
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{Config: repoUpd.Config.Gov}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					// Vote 1
					err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
						TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
						RepoName:   repoName,
						ProposalID: propID,
						Vote:       state.ProposalVoteNoWithVeto,
					}, 0).Exec()
					Expect(err).To(BeNil())

					// Vote 2
					err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
						TxCommon:   &txns.TxCommon{SenderPubKey: key2.PubKey().ToPublicKey(), Fee: "1.5"},
						RepoName:   repoName,
						ProposalID: propID,
						Vote:       state.ProposalVoteYes,
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				It("should increment proposal.Yes by 1 and proposal.NoWithVeto by 1", func() {
					repo := logic.RepoKeeper().Get(repoName)
					Expect(repo.Proposals.Get(propID).NoWithVeto).To(Equal(float64(1)))
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(1)))
				})
			})
		})

		When("proposal tally method is ProposalTallyMethodCoinWeighted", func() {
			BeforeEach(func() {
				repoUpd.Config.Gov.Voter = state.VoterOwner
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodCoinWeighted
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    1,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 10", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(11)))
			})
		})

		When("proposal tally method is ProposalTallyMethodNetStakeOfProposer and the voter's non-delegated ticket value=100", func() {
			BeforeEach(func() {
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStakeNonDelegated
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				mockTickMgr.EXPECT().ValueOfNonDelegatedTickets(sender.PubKey().MustBytes32(), uint64(0)).Return(float64(100), nil)
				logic.SetTicketManager(mockTickMgr)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 100", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(100)))
			})
		})

		When("proposal tally method is ProposalTallyMethodNetStakeOfDelegators and the voter's non-delegated ticket value=100", func() {
			BeforeEach(func() {
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStakeOfDelegators
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				mockTickMgr.EXPECT().ValueOfDelegatedTickets(sender.PubKey().MustBytes32(), uint64(0)).Return(float64(100), nil)
				logic.SetTicketManager(mockTickMgr)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 100", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(100)))
			})
		})

		When("proposal tally method is ProposalTallyMethodNetStake", func() {
			When("ticketA and ticketB are not delegated, with value 10, 20 respectively", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStake
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{
						Config: repoUpd.Config.Gov,
						Yes:    0,
					}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					ticketA := &tickettypes.Ticket{Value: "10"}
					ticketB := &tickettypes.Ticket{Value: "20"}
					tickets := []*tickettypes.Ticket{ticketA, ticketB}

					mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().MustBytes32(), uint64(0)).Return(tickets, nil)
					logic.SetTicketManager(mockTickMgr)
				})

				When("vote is ProposalVoteYes", func() {
					It("should increment proposal.Yes by 30", func() {
						err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
							TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
							RepoName:   repoName,
							ProposalID: propID,
							Vote:       state.ProposalVoteYes,
						}, 0).Exec()
						repo := logic.RepoKeeper().Get(repoName)
						Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
					})
				})

				When("vote is ProposalVoteNo", func() {
					It("should increment proposal.Yes by 30", func() {
						err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
							TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
							RepoName:   repoName,
							ProposalID: propID,
							Vote:       state.ProposalVoteNo,
						}, 0).Exec()
						Expect(err).To(BeNil())
						repo := logic.RepoKeeper().Get(repoName)
						Expect(repo.Proposals.Get(propID).No).To(Equal(float64(30)))
					})
				})

				When("vote is Abstain", func() {
					It("should increment proposal.Yes by 30", func() {
						err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
							TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
							RepoName:   repoName,
							ProposalID: propID,
							Vote:       state.ProposalVoteAbstain,
						}, 0).Exec()
						Expect(err).To(BeNil())
						repo := logic.RepoKeeper().Get(repoName)
						Expect(repo.Proposals.Get(propID).Abstain).To(Equal(float64(30)))
					})
				})

				When("vote is NoWithVeto", func() {
					It("should increment proposal.Yes by 30", func() {
						err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
							TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
							RepoName:   repoName,
							ProposalID: propID,
							Vote:       state.ProposalVoteNoWithVeto,
						}, 0).Exec()
						Expect(err).To(BeNil())
						repo := logic.RepoKeeper().Get(repoName)
						Expect(repo.Proposals.Get(propID).NoWithVeto).To(Equal(float64(30)))
					})
				})
			})
		})

		When("ticketA and ticketB exist, with value 10, 20 respectively. voter is delegator and proposer of ticketB", func() {
			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStake
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				ticketA := &tickettypes.Ticket{Value: "10"}
				ticketB := &tickettypes.Ticket{
					Value:          "20",
					ProposerPubKey: sender.PubKey().MustBytes32(),
					Delegator:      sender.Addr().String(),
				}
				tickets := []*tickettypes.Ticket{ticketA, ticketB}

				mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(tickets, nil)
				logic.SetTicketManager(mockTickMgr)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 30", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
			})
		})

		When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
			"proposer of ticketB but someone else is delegator and they have not "+
			"voted on the proposal", func() {
			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStake
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				ticketA := &tickettypes.Ticket{Value: "10"}
				ticketB := &tickettypes.Ticket{
					Value:          "20",
					ProposerPubKey: sender.PubKey().MustBytes32(),
					Delegator:      key2.Addr().String(),
				}
				tickets := []*tickettypes.Ticket{ticketA, ticketB}

				mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(tickets, nil)
				logic.SetTicketManager(mockTickMgr)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 30", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
			})
		})

		When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
			"proposer of ticketB but someone else is delegator and they have "+
			"voted on the proposal", func() {
			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStake
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				ticketA := &tickettypes.Ticket{Value: "10"}
				ticketB := &tickettypes.Ticket{
					Value:          "20",
					ProposerPubKey: sender.PubKey().MustBytes32(),
					Delegator:      key2.Addr().String(),
				}
				tickets := []*tickettypes.Ticket{ticketA, ticketB}

				mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(tickets, nil)
				logic.SetTicketManager(mockTickMgr)

				logic.RepoKeeper().IndexProposalVote(repoName, propID,
					key2.Addr().String(), state.ProposalVoteYes)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 10", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(10)))
			})
		})

		When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
			"delegator of ticketB but someone else is proposer and they have not "+
			"voted on the proposal", func() {
			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStake
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				ticketA := &tickettypes.Ticket{Value: "10"}
				ticketB := &tickettypes.Ticket{
					Value:          "20",
					ProposerPubKey: key2.PubKey().MustBytes32(),
					Delegator:      sender.Addr().String(),
				}
				tickets := []*tickettypes.Ticket{ticketA, ticketB}

				mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(tickets, nil)
				logic.SetTicketManager(mockTickMgr)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteYes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 30", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
			})
		})

		When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
			"delegator of ticketB but someone else is proposer and they have "+
			"voted 'Yes' on the proposal", func() {
			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStake
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Yes: 100,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				ticketA := &tickettypes.Ticket{Value: "10"}
				ticketB := &tickettypes.Ticket{
					Value:          "20",
					ProposerPubKey: key2.PubKey().MustBytes32(),
					Delegator:      sender.Addr().String(),
				}
				tickets := []*tickettypes.Ticket{ticketA, ticketB}

				mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(tickets, nil)
				logic.SetTicketManager(mockTickMgr)

				logic.RepoKeeper().IndexProposalVote(repoName, propID,
					key2.Addr().String(), state.ProposalVoteYes)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteNo,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should increment proposal.No by 30", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).No).To(Equal(float64(30)))
			})

			Specify("that proposal.Yes is now 80", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(80)))
			})
		})

		When("proposal tally method is ProposalTallyMethodNetStake and "+
			"proposer type is ProposerNetStakeholdersAndVetoOwner and "+
			"voter is a veto owner", func() {
			BeforeEach(func() {
				repoUpd.Config.Gov.PropTallyMethod = state.ProposalTallyMethodNetStake
				repoUpd.Config.Gov.Voter = state.VoterNetStakersAndVetoOwner
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{Veto: true})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Gov,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				ticketA := &tickettypes.Ticket{Value: "10"}
				ticketB := &tickettypes.Ticket{Value: "20"}
				tickets := []*tickettypes.Ticket{ticketA, ticketB}

				mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(tickets, nil)
				logic.SetTicketManager(mockTickMgr)

				err = voteproposal.NewContract().Init(logic, &txns.TxRepoProposalVote{
					TxCommon:   &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					RepoName:   repoName,
					ProposalID: propID,
					Vote:       state.ProposalVoteNoWithVeto,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			When("vote is NoWithVeto", func() {
				It("should increment proposal.NoWithVeto by 30", func() {
					repo := logic.RepoKeeper().Get(repoName)
					Expect(repo.Proposals.Get(propID).NoWithVeto).To(Equal(float64(30)))
				})

				It("should increment NoWithVetoByOwners by 1", func() {
					repo := logic.RepoKeeper().Get(repoName)
					Expect(repo.Proposals.Get(propID).NoWithVetoByOwners).To(Equal(float64(1)))
				})
			})
		})
	})
})
