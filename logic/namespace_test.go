package logic

import (
	"os"

	"github.com/golang/mock/gomock"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespace", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var txLogic *Transaction
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
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

	Describe(".execPush", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var nsName = "name1"

		When("when transfer account and repo are not set", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
					Nonce:   1,
				})

				_ = txLogic
				err = txLogic.execAcquireNamespace(sender.PubKey().MustBytes32(), nsName, "1", "1", "", "", 0)
				Expect(err).To(BeNil())
			})

			Specify("that namespace was created", func() {
				ns := logic.NamespaceKeeper().GetNamespace(nsName)
				Expect(ns.IsNil()).To(BeFalse())
			})

			Specify("that expireAt is set 10", func() {
				ns := logic.NamespaceKeeper().GetNamespace(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.ExpiresAt).To(Equal(uint64(10)))
			})

			Specify("that graceEndAt is set 20", func() {
				ns := logic.NamespaceKeeper().GetNamespace(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.GraceEndAt).To(Equal(uint64(20)))
			})

			Specify("that sender account is deduct of fee+value", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("8")))
			})

			Specify("that fee is paid to the treasury address", func() {
				acct := logic.AccountKeeper().GetAccount(params.TreasuryAddress)
				Expect(acct.Balance).To(Equal(util.String("1")))
			})
		})

		When("when transfer account is set", func() {
			var transferAcct = "account"
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
					Nonce:   1,
				})

				_ = txLogic
				err = txLogic.execAcquireNamespace(sender.PubKey().MustBytes32(),
					nsName, "1", "1", "", transferAcct, 0)
				Expect(err).To(BeNil())
			})

			Specify("that owner is set to the value of transferToAccount", func() {
				ns := logic.NamespaceKeeper().GetNamespace(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Owner).To(Equal(transferAcct))
			})
		})

		When("when transfer repo is set", func() {
			var transferToRepo = "repo"
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
					Nonce:   1,
				})

				_ = txLogic
				err = txLogic.execAcquireNamespace(sender.PubKey().MustBytes32(),
					nsName, "1", "1", transferToRepo, "", 0)
				Expect(err).To(BeNil())
			})

			Specify("that owner is set to the value of transferToRepo", func() {
				ns := logic.NamespaceKeeper().GetNamespace(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Owner).To(Equal(transferToRepo))
			})
		})
	})
})
