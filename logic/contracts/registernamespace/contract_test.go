package registernamespace_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/registernamespace"
	"github.com/make-os/kit/params"
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

func TestAcquireNamespace(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AcquireNamespace Suite")
}

var _ = Describe("Contract", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = ed25519.NewKeyFromIntSeed(1)

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
			ct := registernamespace.NewContract()
			Expect(ct.CanExec(txns.TxTypeNamespaceRegister)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeHostTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var nsName = "name1"

		When("when owner and repo are not set", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})
				err = registernamespace.NewContract().Init(logic, &txns.TxNamespaceRegister{
					Name:     nsName,
					TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					TxValue:  &txns.TxValue{Value: "1"},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that namespace was created", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
			})

			Specify("that expireAt is set 10", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.ExpiresAt.UInt64()).To(Equal(uint64(10)))
			})

			Specify("that graceEndAt is set 20", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.GraceEndAt.UInt64()).To(Equal(uint64(20)))
			})

			Specify("that sender account is deduct of fee+value", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("8")))
			})

			Specify("that value is paid to the treasury address", func() {
				acct := logic.AccountKeeper().Get(identifier.Address(params.TreasuryAddress))
				Expect(acct.Balance).To(Equal(util.String("1")))
			})

			Specify("that nonce was incremented", func() {
				acct := logic.AccountKeeper().Get(identifier.Address(sender.Addr()))
				Expect(acct.Nonce.UInt64()).To(Equal(uint64(2)))
			})

			Specify("that owner is the sender", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.Owner).To(Equal(sender.Addr().String()))
			})
		})

		When("owner is set to a user account", func() {
			var transferAcct = "account"
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})

				err = registernamespace.NewContract().Init(logic, &txns.TxNamespaceRegister{
					Name:     nsName,
					TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					TxValue:  &txns.TxValue{Value: "1"},
					To:       transferAcct,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that owner is set to the value of transferToAccount", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Owner).To(Equal(transferAcct))
			})
		})

		When("when transfer repo is set to a repo name", func() {
			var transferToRepo = "repo"
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})

				err = registernamespace.NewContract().Init(logic, &txns.TxNamespaceRegister{
					Name:     nsName,
					TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					TxValue:  &txns.TxValue{Value: "1"},
					To:       transferToRepo,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that owner is set to the value of transferToRepo", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Owner).To(Equal(transferToRepo))
			})
		})
	})
})
