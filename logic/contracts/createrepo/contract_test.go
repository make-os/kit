package createrepo_test

import (
	"os"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/createrepo"
	"github.com/make-os/kit/remote/policy"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestCreateRepoContract(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CreateRepo Suite")
}

var _ = Describe("CreateRepoContract", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = ed25519.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		Expect(err).To(BeNil())
		cfg, err = testutil.SetTestCfg()
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
			ct := createrepo.NewContract()
			Expect(ct.CanExec(txns.TxTypeRepoCreate)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeValidatorTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var repoCfg *state.RepoConfig
		var tx *txns.TxRepoCreate

		BeforeEach(func() {
			repoCfg = state.MakeDefaultRepoConfig()
			tx = &txns.TxRepoCreate{
				Name: "repo",
				TxDescription: &txns.TxDescription{
					Description: "a repository",
				},
				Config:   repoCfg,
				TxValue:  &txns.TxValue{Value: "4"},
				TxCommon: &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
			}
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Stakes: state.BareAccountStakes(), DelegatorCommission: 10})
		})

		Context("on successful", func() {
			var repo *state.Repository
			BeforeEach(func() {
				createrepo.NewContract().Init(logic, tx, 0).Exec()
				Expect(err).To(BeNil())
				repo = logic.RepoKeeper().Get("repo")
			})

			It("should set CreatedAt height to 1", func() {
				Expect(repo.CreatedAt.UInt64()).To(Equal(uint64(1)))
			})

			It("should set Description", func() {
				Expect(repo.Description).To(Equal(tx.Description))
			})

			Specify("that repo config is the default", func() {
				defCfg := state.MakeDefaultRepoConfig()
				policy.AddDefaultPolicies(defCfg)
				repo.Config.ResetCodec()
				Expect(repo.Config).To(Equal(defCfg))
			})

			Specify("that fee + value is deducted from sender account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("4.5")))
			})

			Specify("that the tx value is added to the repo's balance", func() {
				Expect(repo.Balance).To(Equal(util.String("4")))
			})

			Specify("that sender account nonce increased", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Nonce.UInt64()).To(Equal(uint64(1)))
			})

			When("voter type is VoteByOwner", func() {
				BeforeEach(func() {
					repoCfg.Gov.Voter = state.VoterOwner.Ptr()
					createrepo.NewContract().Init(logic, &txns.TxRepoCreate{Name: "repo",
						Config:        repoCfg,
						TxDescription: &txns.TxDescription{},
						TxValue:       &txns.TxValue{Value: "0"},
						TxCommon:      &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				Specify("that the repo was added to the tree, the send was added as an owner", func() {
					repo := logic.RepoKeeper().Get("repo")
					Expect(repo.IsEmpty()).To(BeFalse())
					Expect(repo.Owners).To(HaveKey(sender.Addr().String()))
				})
			})

			When("voter type is not VoteByOwner", func() {
				BeforeEach(func() {
					repoCfg.Gov.Voter = state.VoterNetStakers.Ptr()
					tx.TxValue = &txns.TxValue{Value: "0"}
					tx.TxCommon = &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()}
					tx.Config = repoCfg
					createrepo.NewContract().Init(logic, tx, 0).Exec()
					Expect(err).To(BeNil())
				})

				It("should not add the sender as an owner", func() {
					repo := logic.RepoKeeper().Get("repo")
					Expect(repo.Owners).To(BeEmpty())
				})
			})

			When("voter type is VoterNetStakersAndVetoOwner", func() {
				BeforeEach(func() {
					repoCfg.Gov.Voter = state.VoterNetStakersAndVetoOwner.Ptr()
					createrepo.NewContract().Init(logic, &txns.TxRepoCreate{Name: "repo",
						Config:        repoCfg,
						TxDescription: &txns.TxDescription{},
						TxValue:       &txns.TxValue{Value: "0"},
						TxCommon:      &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				Specify("that the repo was added to the tree, the send was added as a veto owner", func() {
					repo := logic.RepoKeeper().Get("repo")
					Expect(repo.IsEmpty()).To(BeFalse())
					Expect(repo.Owners).To(HaveKey(sender.Addr().String()))
					Expect(repo.Owners.Get(sender.Addr().String()).Veto).To(BeTrue())
				})
			})

			When("non-nil repo config is provided", func() {
				repoCfg2 := &state.RepoConfig{Gov: &state.RepoConfigGovernance{PropDuration: pointer.ToString("1000")}}
				BeforeEach(func() {
					tx.TxValue = &txns.TxValue{Value: "0"}
					tx.TxCommon = &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()}
					tx.Config = repoCfg2
					createrepo.NewContract().Init(logic, tx, 0).Exec()
					Expect(err).To(BeNil())
				})

				Specify("that repo config is not the default", func() {
					repo := logic.RepoKeeper().Get("repo")
					Expect(repo.Config).ToNot(Equal(state.DefaultRepoConfig))
					Expect(*repo.Config.Gov.PropDuration).To(Equal("1000"))
				})
			})
		})

		When("governance.CreatorAsContributor is true", func() {
			BeforeEach(func() {
				repoCfg.Gov.CreatorAsContributor = pointer.ToBool(true)
				createrepo.NewContract().Init(logic, tx, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that sender was added as a contributor", func() {
				repo := logic.RepoKeeper().Get("repo")
				Expect(repo.Contributors).To(HaveLen(1))
				Expect(repo.Contributors.Has(sender.PushAddr().String())).To(BeTrue())
				contrib := repo.Contributors[sender.PushAddr().String()]
				Expect(contrib.FeeUsed).To(Equal(util.String("0")))
				Expect(contrib.FeeCap).To(Equal(util.String("0")))
				Expect(contrib.FeeMode).To(Equal(state.FeeModePusherPays))
			})
		})

		When("governance.CreatorAsContributor is false", func() {
			BeforeEach(func() {
				repoCfg.Gov.CreatorAsContributor = pointer.ToBool(false)
				tx.Config = repoCfg
				createrepo.NewContract().Init(logic, tx, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that sender was added as a contributor", func() {
				repo := logic.RepoKeeper().Get("repo")
				Expect(repo.Contributors).To(HaveLen(0))
			})
		})
	})
})
