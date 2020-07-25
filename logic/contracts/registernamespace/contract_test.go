package registernamespace_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	logic2 "github.com/themakeos/lobe/logic"
	"github.com/themakeos/lobe/logic/contracts/registernamespace"
	"github.com/themakeos/lobe/params"
	"github.com/themakeos/lobe/storage"
	"github.com/themakeos/lobe/testutil"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
	"github.com/themakeos/lobe/util/identifier"
)

var _ = Describe("RegisterNamespaceContract", func() {
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
			ct := registernamespace.NewContract()
			Expect(ct.CanExec(txns.TxTypeNamespaceRegister)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeHostTicket)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var nsName = "name1"

		When("when transfer account and repo are not set", func() {
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
		})

		When("when transfer account is set to an account", func() {
			var transferAcct = "account"
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})

				err = registernamespace.NewContract().Init(logic, &txns.TxNamespaceRegister{
					Name:       nsName,
					TxCommon:   &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					TxValue:    &txns.TxValue{Value: "1"},
					TransferTo: transferAcct,
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
					Name:       nsName,
					TxCommon:   &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					TxValue:    &txns.TxValue{Value: "1"},
					TransferTo: transferToRepo,
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
