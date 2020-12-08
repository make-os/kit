package depositproposalfee_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/depositproposalfee"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestDepositProposalFee(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DepositProposalFee Suite")
}

var _ = Describe("DepositProposalFeeContract", func() {
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
			ct := depositproposalfee.NewContract()
			Expect(ct.CanExec(txns.TxTypeRepoProposalSendFee)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeValidatorTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repoUpd *state.Repository

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "20", DelegatorCommission: 0})
			logic.AccountKeeper().Update(key2.Addr(), &state.Account{Balance: "20", DelegatorCommission: 0})
			repoUpd = state.BareRepository()
			repoUpd.Config = state.DefaultRepoConfig
			repoUpd.Config.Gov.Voter = state.VoterOwner
		})

		When("sender has not previously deposited", func() {
			repoName := "repo"
			proposalFee := util.String("10")
			propID := "1"

			BeforeEach(func() {
				repoUpd.Proposals.Add(propID, &state.RepoProposal{Fees: map[string]string{}})
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = depositproposalfee.NewContract().Init(logic, &txns.TxRepoProposalSendFee{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{Value: proposalFee, RepoName: repoName, ID: propID},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add fee to proposal", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).Fees).To(HaveLen(1))
				Expect(repo.Proposals.Get(propID).Fees.Get(sender.Addr().String())).To(Equal(proposalFee))
			})

			Specify("that network fee + proposal fee was deducted", func() {
				acct := logic.AccountKeeper().Get(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("8.5"))
			})

			When("sender already deposited", func() {
				proposalFee := util.String("2")

				BeforeEach(func() {
					err = depositproposalfee.NewContract().Init(logic, &txns.TxRepoProposalSendFee{
						TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
						TxProposalCommon: &txns.TxProposalCommon{Value: proposalFee, RepoName: repoName, ID: propID},
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				It("should add fee to existing senders deposited proposal fee", func() {
					repo := logic.RepoKeeper().Get(repoName)
					Expect(repo.Proposals).To(HaveLen(1))
					Expect(repo.Proposals.Get("1").Fees).To(HaveLen(1))
					Expect(repo.Proposals.Get("1").Fees.Get(sender.Addr().String())).To(Equal(util.String("12")))
				})

				Specify("that network fee + proposal fee was deducted", func() {
					acct := logic.AccountKeeper().Get(sender.Addr(), 0)
					Expect(acct.Balance.String()).To(Equal("5"))
				})
			})
		})

		When("two different senders deposit proposal fees", func() {
			repoName := "repo"
			proposalFee := util.String("10")
			propID := "1"

			BeforeEach(func() {
				repoUpd.Proposals.Add(propID, &state.RepoProposal{Fees: map[string]string{}})
				logic.RepoKeeper().Update(repoName, repoUpd)

				err = depositproposalfee.NewContract().Init(logic, &txns.TxRepoProposalSendFee{
					TxCommon:         &txns.TxCommon{SenderPubKey: sender.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{Value: proposalFee, RepoName: repoName, ID: propID},
				}, 0).Exec()
				Expect(err).To(BeNil())

				err = depositproposalfee.NewContract().Init(logic, &txns.TxRepoProposalSendFee{
					TxCommon:         &txns.TxCommon{SenderPubKey: key2.PubKey().ToPublicKey(), Fee: "1.5"},
					TxProposalCommon: &txns.TxProposalCommon{Value: proposalFee, RepoName: repoName, ID: propID},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that the proposal has 2 entries from both depositors", func() {
				repo := logic.RepoKeeper().Get(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Fees).To(HaveLen(2))
				Expect(repo.Proposals.Get("1").Fees.Get(sender.Addr().String())).To(Equal(proposalFee))
				Expect(repo.Proposals.Get("1").Fees.Get(key2.Addr().String())).To(Equal(proposalFee))
				Expect(repo.Proposals.Get("1").Fees.Total().String()).To(Equal("20"))
			})
		})
	})
})
