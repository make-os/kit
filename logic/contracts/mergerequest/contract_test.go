package mergerequest_test

// import (
// 	"os"
//
// 	"github.com/golang/mock/gomock"
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// 	"gitlab.com/makeos/mosdef/config"
// 	"gitlab.com/makeos/mosdef/crypto"
// 	logic2 "gitlab.com/makeos/mosdef/logic"
// 	"gitlab.com/makeos/mosdef/logic/contracts/mergerequest"
// 	"gitlab.com/makeos/mosdef/storage"
// 	"gitlab.com/makeos/mosdef/testutil"
// 	"gitlab.com/makeos/mosdef/types/core"
// 	"gitlab.com/makeos/mosdef/types/state"
// 	"gitlab.com/makeos/mosdef/util"
// )
//
// var _ = Describe("MergeRequestContract", func() {
// 	var appDB, stateTreeDB storage.Engine
// 	var err error
// 	var cfg *config.AppConfig
// 	var logic *logic2.Logic
// 	var ctrl *gomock.Controller
// 	var sender = crypto.NewKeyFromIntSeed(1)
// 	var key2 = crypto.NewKeyFromIntSeed(2)
//
// 	BeforeEach(func() {
// 		ctrl = gomock.NewController(GinkgoT())
// 		cfg, err = testutil.SetTestCfg()
// 		Expect(err).To(BeNil())
// 		appDB, stateTreeDB = testutil.GetDB(cfg)
// 		logic = logic2.New(appDB, stateTreeDB, cfg)
// 		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
// 		Expect(err).To(BeNil())
// 	})
//
// 	AfterEach(func() {
// 		ctrl.Finish()
// 		Expect(appDB.Close()).To(BeNil())
// 		Expect(stateTreeDB.Close()).To(BeNil())
// 		err = os.RemoveAll(cfg.DataDir())
// 		Expect(err).To(BeNil())
// 	})
//
// 	Describe(".CanExec", func() {
// 		It("should return true when able to execute tx type", func() {
// 			ct := mergerequest.NewContract()
// 			Expect(ct.CanExec(core.TxTypeRepoProposalMergeRequest)).To(BeTrue())
// 			Expect(ct.CanExec(core.TxTypeHostTicket)).To(BeFalse())
// 		})
// 	})
//
// 	Describe(".Exec", func() {
// 		var err error
// 		var repoUpd *state.Repository
//
// 		BeforeEach(func() {
// 			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
// 				Balance:             "10",
// 				Stakes:              state.BareAccountStakes(),
// 				DelegatorCommission: 10,
// 			})
// 			repoUpd = state.BareRepository()
// 			repoUpd.Config = state.DefaultRepoConfig
// 			repoUpd.Config.Governance.Voter = state.VoterOwner
// 		})
//
// 		When("sender is the only owner", func() {
// 			repoName := "repo"
// 			proposalFee := util.String("1")
// 			propID := "1"
//
// 			BeforeEach(func() {
// 				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
// 				logic.RepoKeeper().Update(repoName, repoUpd)
//
// 				err = mergerequest.NewContract().Init(logic, &core.TxRepoProposalMergeRequest{
// 					TxCommon:         &core.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
// 					TxProposalCommon: &core.TxProposalCommon{RepoName: repoName, Value: proposalFee, ProposalID: propID},
// 					BaseBranch:       "base",
// 					BaseBranchHash:   "baseHash",
// 					TargetBranch:     "target",
// 					TargetBranchHash: "targetHash",
// 				}, 0).Exec()
// 				Expect(err).To(BeNil())
// 			})
//
// 			It("should add the new proposal to the repo", func() {
// 				repo := logic.RepoKeeper().Get(repoName)
// 				Expect(repo.Proposals).To(HaveLen(1))
// 			})
//
// 			Specify("that the proposal is finalized and self accepted", func() {
// 				repo := logic.RepoKeeper().Get(repoName)
// 				Expect(repo.Proposals).To(HaveLen(1))
// 				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeTrue())
// 				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(1)))
// 			})
//
// 			Specify("that network fee + proposal fee was deducted", func() {
// 				acct := logic.AccountKeeper().Get(sender.Addr(), 0)
// 				Expect(acct.Balance.String()).To(Equal("7.5"))
// 			})
//
// 			Specify("that the proposal fee by the sender is registered on the proposal", func() {
// 				repo := logic.RepoKeeper().Get(repoName)
// 				Expect(repo.Proposals.Get(propID).Fees).To(HaveLen(1))
// 				Expect(repo.Proposals.Get(propID).Fees).To(HaveKey(sender.Addr().String()))
// 				Expect(repo.Proposals.Get(propID).Fees[sender.Addr().String()]).To(Equal(propID))
// 			})
// 		})
//
// 		When("sender is not the only owner", func() {
// 			repoName := "repo"
// 			curHeight := uint64(0)
// 			proposalFee := util.String("1")
// 			propID := "1"
//
// 			BeforeEach(func() {
// 				repoUpd.AddOwner(sender.Addr().String(), &state.RepoOwner{})
// 				repoUpd.AddOwner(key2.Addr().String(), &state.RepoOwner{})
// 				logic.RepoKeeper().Update(repoName, repoUpd)
//
// 				err = mergerequest.NewContract().Init(logic, &core.TxRepoProposalMergeRequest{
// 					TxCommon:         &core.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
// 					TxProposalCommon: &core.TxProposalCommon{RepoName: repoName, Value: proposalFee, ProposalID: propID},
// 					BaseBranch:       "base",
// 					BaseBranchHash:   "baseHash",
// 					TargetBranch:     "target",
// 					TargetBranchHash: "targetHash",
// 				}, 0).Exec()
// 				Expect(err).To(BeNil())
// 			})
//
// 			It("should add the new proposal to the repo", func() {
// 				repo := logic.RepoKeeper().Get(repoName)
// 				Expect(repo.Proposals).To(HaveLen(1))
// 			})
//
// 			Specify("that the proposal is not finalized or self accepted", func() {
// 				repo := logic.RepoKeeper().Get(repoName)
// 				Expect(repo.Proposals).To(HaveLen(1))
// 				Expect(repo.Proposals.Get(propID).IsFinalized()).To(BeFalse())
// 				Expect(repo.Proposals.Get(propID).Yes).To(Equal(float64(0)))
// 			})
//
// 			Specify("that network fee + proposal fee was deducted", func() {
// 				acct := logic.AccountKeeper().Get(sender.Addr(), curHeight)
// 				Expect(acct.Balance.String()).To(Equal("7.5"))
// 			})
//
// 			Specify("that the proposal fee by the sender is registered on the proposal", func() {
// 				repo := logic.RepoKeeper().Get(repoName)
// 				Expect(repo.Proposals.Get(propID).Fees).To(HaveLen(1))
// 				Expect(repo.Proposals.Get(propID).Fees).To(HaveKey(sender.Addr().String()))
// 				Expect(repo.Proposals.Get(propID).Fees[sender.Addr().String()]).To(Equal(propID))
// 			})
//
// 			Specify("that the proposal was indexed against its end height", func() {
// 				res := logic.RepoKeeper().GetProposalsEndingAt(repoUpd.Config.Governance.DurOfProposal + curHeight + 1)
// 				Expect(res).To(HaveLen(1))
// 			})
// 		})
// 	})
// })
