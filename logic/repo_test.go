package logic

import (
	"os"

	types3 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/golang/mock/gomock"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("Repo", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var txLogic *Transaction
	var ctrl *gomock.Controller
	var mockTickMgr *mocks.MockTicketManager
	var sender = crypto.NewKeyFromIntSeed(1)
	var key2 = crypto.NewKeyFromIntSeed(2)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
		mockTickMgr = mocks.NewMockTicketManager(ctrl)
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		state.DefaultRepoConfig = state.MakeDefaultRepoConfig()
		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".execRepoCreate", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var spk util.Bytes32
		var repoCfg *state.RepoConfig

		BeforeEach(func() {
			repoCfg = state.MakeDefaultRepoConfig()
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             util.String("10"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("successful", func() {
			BeforeEach(func() {
				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoCreate(spk, "repo", repoCfg.ToMap(), "1.5", 0)
				Expect(err).To(BeNil())
			})

			Specify("that repo config is the default", func() {
				repo := txLogic.logic.RepoKeeper().GetRepo("repo")
				Expect(repo.Config).To(Equal(state.DefaultRepoConfig))
			})

			Specify("that fee is deducted from sender account", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("8.5")))
			})

			Specify("that sender account nonce increased", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.Nonce).To(Equal(uint64(1)))
			})

			When("proposee is ProposalOwner", func() {
				BeforeEach(func() {
					repoCfg.Governance.ProposalProposee = state.ProposeeOwner
					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoCreate(spk, "repo", repoCfg.ToMap(), "1.5", 0)
					Expect(err).To(BeNil())
				})

				Specify("that the repo was added to the tree", func() {
					repo := txLogic.logic.RepoKeeper().GetRepo("repo")
					Expect(repo.IsNil()).To(BeFalse())
					Expect(repo.Owners).To(HaveKey(sender.Addr().String()))
				})
			})

			When("proposee is not ProposalOwner", func() {
				BeforeEach(func() {
					repoCfg.Governance.ProposalProposee = state.ProposeeNetStakeholders
					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoCreate(spk, "repo", repoCfg.ToMap(), "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should not add the sender as an owner", func() {
					repo := txLogic.logic.RepoKeeper().GetRepo("repo")
					Expect(repo.Owners).To(BeEmpty())
				})
			})

			When("non-nil repo config is provided", func() {
				repoCfg2 := &state.RepoConfig{Governance: &state.RepoConfigGovernance{ProposalDur: 1000}}
				BeforeEach(func() {
					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoCreate(spk, "repo", repoCfg2.ToMap(), "1.5", 0)
					Expect(err).To(BeNil())
				})

				Specify("that repo config is not the default", func() {
					repo := txLogic.logic.RepoKeeper().GetRepo("repo")
					Expect(repo.Config).ToNot(Equal(state.DefaultRepoConfig))
					Expect(repo.Config.Governance.ProposalDur).To(Equal(uint64(1000)))
				})
			})
		})
	})

	Describe(".execRepoProposalVote", func() {
		var err error
		var spk util.Bytes32

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             util.String("10"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("proposal tally method is ProposalTallyMethodIdentity", func() {
			var propID = "proposer_id"
			var repoName = "repo"

			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Governance.ProposalProposee = state.ProposeeOwner
				repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodIdentity
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Governance,
					Yes:    1,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 1", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(2)))
			})

			Context("with two votes; vote 1 = NoWithVeto, vote 2=Yes", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodIdentity
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{Config: repoUpd.Config.Governance}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					// Vote 1
					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteNoWithVeto, "1.5", 0)
					Expect(err).To(BeNil())

					// Vote 2
					spk = key2.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should increment proposal.Yes by 1 and proposal.NoWithVeto by 1", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).NoWithVeto).To(Equal(float64(1)))
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(1)))
				})
			})
		})

		When("proposal tally method is ProposalTallyMethodCoinWeighted", func() {
			var propID = "proposer_id"
			var repoName = "repo"

			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Governance.ProposalProposee = state.ProposeeOwner
				repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodCoinWeighted
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Governance,
					Yes:    1,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 10", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(11)))
			})
		})

		When("proposal tally method is ProposalTallyMethodNetStakeOfProposer and the voter's non-delegated ticket value=100", func() {
			var propID = "proposer_id"
			var repoName = "repo"

			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStakeOfProposer
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Governance,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				mockTickMgr.EXPECT().ValueOfNonDelegatedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(float64(100), nil)
				txLogic.logic.SetTicketManager(mockTickMgr)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 100", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(100)))
			})
		})

		When("proposal tally method is ProposalTallyMethodNetStakeOfDelegators and the voter's non-delegated ticket value=100", func() {
			var propID = "proposer_id"
			var repoName = "repo"

			BeforeEach(func() {
				repoUpd := state.BareRepository()
				repoUpd.Config = state.DefaultRepoConfig
				repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStakeOfDelegators
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				proposal := &state.RepoProposal{
					Config: repoUpd.Config.Governance,
					Yes:    0,
				}
				repoUpd.Proposals.Add(propID, proposal)
				logic.RepoKeeper().Update(repoName, repoUpd)

				mockTickMgr.EXPECT().ValueOfDelegatedTickets(sender.PubKey().
					MustBytes32(), uint64(0)).Return(float64(100), nil)
				txLogic.logic.SetTicketManager(mockTickMgr)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should increment proposal.Yes by 100", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(100)))
			})
		})

		When("proposal tally method is ProposalTallyMethodNetStake", func() {
			var propID = "proposer_id"
			var repoName = "repo"

			When("ticketA and ticketB are not delegated, with value 10, 20 respectively", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStake
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{
						Config: repoUpd.Config.Governance,
						Yes:    0,
					}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					ticketA := &types3.Ticket{Value: "10"}
					ticketB := &types3.Ticket{Value: "20"}
					tickets := []*types3.Ticket{ticketA, ticketB}

					mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
						MustBytes32(), uint64(0)).Return(tickets, nil)
					txLogic.logic.SetTicketManager(mockTickMgr)

					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should increment proposal.Yes by 30", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
				})
			})

			When("ticketA and ticketB exist, with value 10, 20 respectively. voter is delegator and proposer of ticketB", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStake
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{
						Config: repoUpd.Config.Governance,
						Yes:    0,
					}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					ticketA := &types3.Ticket{Value: "10"}
					ticketB := &types3.Ticket{
						Value:          "20",
						ProposerPubKey: sender.PubKey().MustBytes32(),
						Delegator:      sender.Addr().String(),
					}
					tickets := []*types3.Ticket{ticketA, ticketB}

					mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
						MustBytes32(), uint64(0)).Return(tickets, nil)
					txLogic.logic.SetTicketManager(mockTickMgr)

					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should increment proposal.Yes by 30", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
				})
			})

			When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
				"proposer of ticketB but someone else is delegator and they have not "+
				"voted on the proposal", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStake
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{
						Config: repoUpd.Config.Governance,
						Yes:    0,
					}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					ticketA := &types3.Ticket{Value: "10"}
					ticketB := &types3.Ticket{
						Value:          "20",
						ProposerPubKey: sender.PubKey().MustBytes32(),
						Delegator:      key2.Addr().String(),
					}
					tickets := []*types3.Ticket{ticketA, ticketB}

					mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
						MustBytes32(), uint64(0)).Return(tickets, nil)
					txLogic.logic.SetTicketManager(mockTickMgr)

					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should increment proposal.Yes by 30", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
				})
			})

			When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
				"proposer of ticketB but someone else is delegator and they have "+
				"voted on the proposal", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStake
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{
						Config: repoUpd.Config.Governance,
						Yes:    0,
					}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					ticketA := &types3.Ticket{Value: "10"}
					ticketB := &types3.Ticket{
						Value:          "20",
						ProposerPubKey: sender.PubKey().MustBytes32(),
						Delegator:      key2.Addr().String(),
					}
					tickets := []*types3.Ticket{ticketA, ticketB}

					mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
						MustBytes32(), uint64(0)).Return(tickets, nil)
					txLogic.logic.SetTicketManager(mockTickMgr)

					logic.RepoKeeper().IndexProposalVote(repoName, propID,
						key2.Addr().String(), state.ProposalVoteYes)

					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should increment proposal.Yes by 10", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(10)))
				})
			})

			When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
				"delegator of ticketB but someone else is proposer and they have not "+
				"voted on the proposal", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStake
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{
						Config: repoUpd.Config.Governance,
						Yes:    0,
					}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					ticketA := &types3.Ticket{Value: "10"}
					ticketB := &types3.Ticket{
						Value:          "20",
						ProposerPubKey: key2.PubKey().MustBytes32(),
						Delegator:      sender.Addr().String(),
					}
					tickets := []*types3.Ticket{ticketA, ticketB}

					mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
						MustBytes32(), uint64(0)).Return(tickets, nil)
					txLogic.logic.SetTicketManager(mockTickMgr)

					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteYes, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should increment proposal.Yes by 30", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(30)))
				})
			})

			When("ticketA and ticketB exist, with value 10, 20 respectively. voter is "+
				"delegator of ticketB but someone else is proposer and they have "+
				"voted 'Yes' on the proposal", func() {
				BeforeEach(func() {
					repoUpd := state.BareRepository()
					repoUpd.Config = state.DefaultRepoConfig
					repoUpd.Config.Governance.ProposalTallyMethod = state.ProposalTallyMethodNetStake
					repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
					proposal := &state.RepoProposal{
						Yes: 100,
					}
					repoUpd.Proposals.Add(propID, proposal)
					logic.RepoKeeper().Update(repoName, repoUpd)

					ticketA := &types3.Ticket{Value: "10"}
					ticketB := &types3.Ticket{
						Value:          "20",
						ProposerPubKey: key2.PubKey().MustBytes32(),
						Delegator:      sender.Addr().String(),
					}
					tickets := []*types3.Ticket{ticketA, ticketB}

					mockTickMgr.EXPECT().GetNonDecayedTickets(sender.PubKey().
						MustBytes32(), uint64(0)).Return(tickets, nil)
					txLogic.logic.SetTicketManager(mockTickMgr)

					logic.RepoKeeper().IndexProposalVote(repoName, propID,
						key2.Addr().String(), state.ProposalVoteYes)

					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalVote(spk, repoName, propID, state.ProposalVoteNo, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should increment proposal.No by 30", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).No).To(Equal(float64(30)))
				})

				Specify("that proposal.Yes is now 80", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(80)))
				})
			})
		})
	})

	Describe(".execRepoUpsertOwner", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var key2 = crypto.NewKeyFromIntSeed(2)
		var spk util.Bytes32
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             util.String("10"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Governance.ProposalProposee = state.ProposeeOwner
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			address := "owner_address"
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoUpsertOwner(spk, repoName, "1", address, false, proposalFee, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(1)))
			})

			Specify("that new owner was added", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Owners).To(HaveLen(2))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})
		})

		When("sender is the only owner and there are multiple addresses", func() {
			repoName := "repo"
			addresses := "owner_address,owner_address2"
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoUpsertOwner(spk, repoName, "1", addresses, false, proposalFee, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(1)))
				Expect(repo.Proposals.Get("1").Outcome).To(Equal(state.ProposalOutcomeAccepted))
			})

			Specify("that three owners were added", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Owners).To(HaveLen(3))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})
		})

		When("sender is not the only owner", func() {
			repoName := "repo"
			address := "owner_address"
			curHeight := uint64(0)
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoUpsertOwner(spk, repoName, "1", address, false,
					proposalFee, "1.5", curHeight)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is not finalized or self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeFalse())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(0)))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), curHeight)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})

			Specify("that the proposal was indexed against its end height", func() {
				res := logic.RepoKeeper().GetProposalsEndingAt(repoUpd.Config.Governance.ProposalDur + curHeight + 1)
				Expect(res).To(HaveLen(1))
			})
		})

		When("repo config has proposal deposit fee duration set to a non-zero number", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			address := "owner_address"
			currentHeight := uint64(200)

			BeforeEach(func() {
				repoUpd.Config.Governance.ProposalDur = 1000
				repoUpd.Config.Governance.ProposalFeeDepDur = 100
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoUpsertOwner(spk, repoName, "1", address, false, proposalFee, "1.5", currentHeight)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal with expected `endAt` and `feeDepEndAt` values", func() {
				repo := logic.RepoKeeper().GetRepoOnly(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").FeeDepositEndAt).To(Equal(uint64(301)))
				Expect(repo.Proposals.Get("1").EndAt).To(Equal(uint64(1301)))
			})
		})
	})

	Describe(".execRepoProposalSendFee", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var spk util.Bytes32
		var key2 = crypto.NewKeyFromIntSeed(2)
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             util.String("20"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 0,
			})
			logic.AccountKeeper().Update(key2.Addr(), &state.Account{
				Balance:             util.String("20"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 0,
			})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Governance.ProposalProposee = state.ProposeeOwner
		})

		When("sender has not previously deposited", func() {
			repoName := "repo"
			proposalFee := util.String("10")

			BeforeEach(func() {
				repoUpd.Proposals.Add("1", &state.RepoProposal{
					Fees: map[string]string{},
				})
				logic.RepoKeeper().Update(repoName, repoUpd)
				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalSendFee(spk, repoName, "1", proposalFee, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add fee to proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees.Get(sender.Addr().String())).To(Equal(proposalFee))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("8.5"))
			})

			When("sender already deposit", func() {
				proposalFee := util.String("2")

				BeforeEach(func() {
					spk = sender.PubKey().MustBytes32()
					err = txLogic.execRepoProposalSendFee(spk, repoName, "1", proposalFee, "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should add fee to existing senders deposited proposal fee", func() {
					repo := logic.RepoKeeper().GetRepo(repoName)
					Expect(repo.Proposals).To(HaveLen(1))
					Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
					Expect(repo.Proposals.Get("1").Fees.Get(sender.Addr().String())).To(Equal(util.String("12")))
				})

				Specify("that network fee + proposal fee was deducted", func() {
					acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
					Expect(acct.Balance.String()).To(Equal("5"))
				})
			})
		})

		When("two different senders deposit proposal fees", func() {
			repoName := "repo"
			proposalFee := util.String("10")

			BeforeEach(func() {
				repoUpd.Proposals.Add("1", &state.RepoProposal{
					Fees: map[string]string{},
				})
				logic.RepoKeeper().Update(repoName, repoUpd)
				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalSendFee(spk, repoName, "1", proposalFee, "1.5", 0)
				Expect(err).To(BeNil())

				spk = key2.PubKey().MustBytes32()
				err = txLogic.execRepoProposalSendFee(spk, repoName, "1", proposalFee, "1.5", 0)
				Expect(err).To(BeNil())
			})

			Specify("that the proposal has 2 entries from both depositors", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(2))
				Expect(repo.Proposals.Get("1").Fees.Get(sender.Addr().String())).To(Equal(proposalFee))
				Expect(repo.Proposals.Get("1").Fees.Get(key2.Addr().String())).To(Equal(proposalFee))
				Expect(repo.Proposals.Get("1").Fees.Total().String()).To(Equal("20"))
			})
		})
	})

	Describe(".execRepoProposalUpdate", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var spk util.Bytes32
		var key2 = crypto.NewKeyFromIntSeed(2)
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             util.String("10"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Governance.ProposalProposee = state.ProposeeOwner
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				config := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{ProposalDur: 1000},
				}
				err = txLogic.execRepoProposalUpdate(spk, repoName, "1", config.ToMap(),
					proposalFee, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(1)))
			})

			Specify("that config is updated", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Config).ToNot(Equal(repoUpd.Config))
				Expect(repo.Config.Governance.ProposalDur).To(Equal(uint64(1000)))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})
		})

		When("sender is not the only owner", func() {
			repoName := "repo"
			curHeight := uint64(0)
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				config := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{ProposalDur: 1000},
				}

				err = txLogic.execRepoProposalUpdate(spk, repoName, "1", config.ToMap(), proposalFee, "1.5", curHeight)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is not finalized or self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeFalse())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(0)))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), curHeight)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})

			Specify("that the proposal was indexed against its end height", func() {
				res := logic.RepoKeeper().GetProposalsEndingAt(repoUpd.Config.Governance.ProposalDur + curHeight + 1)
				Expect(res).To(HaveLen(1))
			})
		})

		When("repo config has proposal deposit fee duration set to a non-zero number", func() {
			repoName := "repo"
			proposalFee := util.String("1")
			currentHeight := uint64(200)

			BeforeEach(func() {
				repoUpd.Config.Governance.ProposalDur = 1000
				repoUpd.Config.Governance.ProposalFeeDepDur = 100
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				config := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalDur: 2000,
					},
				}

				err = txLogic.execRepoProposalUpdate(spk, repoName, "1", config.ToMap(),
					proposalFee, "1.5", currentHeight)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal with expected `endAt` and `feeDepEndAt` values", func() {
				repo := logic.RepoKeeper().GetRepoOnly(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").FeeDepositEndAt).To(Equal(uint64(301)))
				Expect(repo.Proposals.Get("1").EndAt).To(Equal(uint64(1301)))
			})
		})
	})

	Describe(".execRepoProposalMergeRequest", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var spk util.Bytes32
		// var key2 = crypto.NewKeyFromIntSeed(2)
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             util.String("10"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Governance.ProposalProposee = state.ProposeeOwner
		})

		When("sender is the only owner", func() {
			repoName := "repo"
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalMergeRequest(spk, repoName, "1",
					"base", "baseHash", "target", "targetHash", proposalFee, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(1)))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})
		})

		When("sender is not the only owner", func() {
			repoName := "repo"
			curHeight := uint64(0)
			proposalFee := util.String("1")

			BeforeEach(func() {
				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
				repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoProposalMergeRequest(spk, repoName, "1",
					"base", "baseHash", "target", "targetHash", proposalFee, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is not finalized or self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeFalse())
				Expect(repo.Proposals.Get("1").Yes).To(Equal(float64(0)))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), curHeight)
				Expect(acct.Balance.String()).To(Equal("7.5"))
			})

			Specify("that the fee by the sender is registered on the proposal", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveKey(sender.Addr().String()))
				Expect(repo.Proposals.Get("1").Fees[sender.Addr().String()]).To(Equal("1"))
			})

			Specify("that the proposal was indexed against its end height", func() {
				res := logic.RepoKeeper().GetProposalsEndingAt(repoUpd.Config.Governance.ProposalDur + curHeight + 1)
				Expect(res).To(HaveLen(1))
			})
		})
	})

	Describe(".applyProposalRepoUpdate", func() {
		var err error
		var repo *state.Repository

		BeforeEach(func() {
			repo = state.BareRepository()
			repo.Config = state.DefaultRepoConfig
		})

		When("update config object is empty", func() {
			It("should not change the config", func() {
				proposal := &state.RepoProposal{ActionData: map[string]interface{}{
					core.ProposalActionDataConfig: (&state.RepoConfig{}).ToMap(),
				}}
				err = applyProposalRepoUpdate(proposal, repo, 0)
				Expect(err).To(BeNil())
				Expect(repo.Config).To(Equal(state.DefaultRepoConfig))
			})
		})

		When("update config object is not empty", func() {
			It("should change the config", func() {
				proposal := &state.RepoProposal{ActionData: map[string]interface{}{
					core.ProposalActionDataConfig: (&state.RepoConfig{
						Governance: &state.RepoConfigGovernance{
							ProposalQuorum: 120,
							ProposalDur:    100,
						},
					}).ToMap(),
				}}
				err = applyProposalRepoUpdate(proposal, repo, 0)
				Expect(err).To(BeNil())
				Expect(repo.Config.Governance.ProposalQuorum).To(Equal(float64(120)))
				Expect(repo.Config.Governance.ProposalDur).To(Equal(uint64(100)))
			})
		})
	})
})
