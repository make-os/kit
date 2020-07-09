package createrepo_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts/createrepo"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("CreateRepoContract", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)

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
			ct := createrepo.NewContract()
			Expect(ct.CanExec(txns.TxTypeRepoCreate)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeValidatorTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repoCfg *state.RepoConfig

		BeforeEach(func() {
			repoCfg = state.MakeDefaultRepoConfig()
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Stakes: state.BareAccountStakes(), DelegatorCommission: 10})
		})

		When("successful", func() {
			BeforeEach(func() {
				tx := &txns.TxRepoCreate{
					Name:     "repo",
					Config:   repoCfg.ToMap(),
					TxValue:  &txns.TxValue{Value: "4"},
					TxCommon: &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
				}
				createrepo.NewContract().Init(logic, tx, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that repo config is the default", func() {
				repo := logic.RepoKeeper().Get("repo")
				defCfg := state.MakeDefaultRepoConfig()
				policy.AddDefaultPolicies(defCfg)
				Expect(repo.Config).To(Equal(defCfg))
			})

			Specify("that fee + value is deducted from sender account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("4.5")))
			})

			Specify("that the tx value is added to the repo's balance", func() {
				repo := logic.RepoKeeper().Get("repo")
				Expect(repo.Balance).To(Equal(util.String("4")))
			})

			Specify("that sender account nonce increased", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Nonce.UInt64()).To(Equal(uint64(1)))
			})

			When("voter type is VoteByOwner", func() {
				BeforeEach(func() {
					repoCfg.Governance.Voter = state.VoterOwner
					createrepo.NewContract().Init(logic, &txns.TxRepoCreate{Name: "repo",
						Config:   repoCfg.ToMap(),
						TxValue:  &txns.TxValue{Value: "0"},
						TxCommon: &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				Specify("that the repo was added to the tree", func() {
					repo := logic.RepoKeeper().Get("repo")
					Expect(repo.IsNil()).To(BeFalse())
					Expect(repo.Owners).To(HaveKey(sender.Addr().String()))
				})
			})

			When("voter type is not VoteByOwner", func() {
				BeforeEach(func() {
					repoCfg.Governance.Voter = state.VoterNetStakers
					createrepo.NewContract().Init(logic, &txns.TxRepoCreate{
						Name:     "repo",
						Config:   repoCfg.ToBasicMap(),
						TxValue:  &txns.TxValue{Value: "0"},
						TxCommon: &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				It("should not add the sender as an owner", func() {
					repo := logic.RepoKeeper().Get("repo")
					Expect(repo.Owners).To(BeEmpty())
				})
			})

			When("non-nil repo config is provided", func() {
				repoCfg2 := &state.RepoConfig{Governance: &state.RepoConfigGovernance{ProposalDuration: 1000}}
				BeforeEach(func() {
					tx := &txns.TxRepoCreate{
						Name:     "repo",
						Config:   repoCfg2.ToBasicMap(),
						TxValue:  &txns.TxValue{Value: "0"},
						TxCommon: &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					}
					createrepo.NewContract().Init(logic, tx, 0).Exec()
					Expect(err).To(BeNil())
				})

				Specify("that repo config is not the default", func() {
					repo := logic.RepoKeeper().Get("repo")
					Expect(repo.Config).ToNot(Equal(state.DefaultRepoConfig))
					Expect(repo.Config.Governance.ProposalDuration.UInt64()).To(Equal(uint64(1000)))
				})
			})
		})
	})
})
