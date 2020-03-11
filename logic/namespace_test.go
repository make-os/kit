package logic

import (
	"os"

	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/golang/mock/gomock"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
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

	Describe(".execPush", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var nsName = "name1"

		When("when transfer account and repo are not set", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: util.String("10"),
					Stakes:  state.BareAccountStakes(),
					Nonce:   1,
				})

				err = txLogic.execAcquireNamespace(sender.PubKey().MustBytes32(), nsName,
					"1", "1", "", nil, 0)
				Expect(err).To(BeNil())
			})

			Specify("that namespace was created", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
			})

			Specify("that expireAt is set 10", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.ExpiresAt).To(Equal(uint64(10)))
			})

			Specify("that graceEndAt is set 20", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.GraceEndAt).To(Equal(uint64(20)))
			})

			Specify("that sender account is deduct of fee+value", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("8")))
			})

			Specify("that value is paid to the treasury address", func() {
				acct := logic.AccountKeeper().Get(util.Address(params.TreasuryAddress))
				Expect(acct.Balance).To(Equal(util.String("1")))
			})
		})

		When("when transfer account is set", func() {
			var transferAcct = "account"
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: util.String("10"),
					Stakes:  state.BareAccountStakes(),
					Nonce:   1,
				})

				err = txLogic.execAcquireNamespace(sender.PubKey().MustBytes32(),
					nsName, "1", "1", transferAcct, nil, 0)
				Expect(err).To(BeNil())
			})

			Specify("that owner is set to the value of transferToAccount", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Owner).To(Equal(transferAcct))
			})
		})

		When("when transfer repo is set", func() {
			var transferToRepo = "repo"
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: util.String("10"),
					Stakes:  state.BareAccountStakes(),
					Nonce:   1,
				})

				err = txLogic.execAcquireNamespace(sender.PubKey().MustBytes32(),
					nsName, "1", "1", transferToRepo, nil, 0)
				Expect(err).To(BeNil())
			})

			Specify("that owner is set to the value of transferToRepo", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Owner).To(Equal(transferToRepo))
			})
		})
	})

	Describe(".execUpdateNamespaceDomains", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var nsName = "name1"

		When("target domain (domain1) has a value and the domain already exist", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: util.String("10"),
					Stakes:  state.BareAccountStakes(),
					Nonce:   1,
				})

				logic.NamespaceKeeper().Update(nsName, &state.Namespace{
					Domains: map[string]string{
						"domain1": "target",
					},
				})

				update := map[string]string{"domain1": "target_update"}
				err = txLogic.execUpdateNamespaceDomains(sender.PubKey().MustBytes32(), nsName, "1", update, 0)
				Expect(err).To(BeNil())
			})

			Specify("that domain 'domain1' has changed", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Domains["domain1"]).To(Equal("target_update"))
			})

			Specify("that sender account is deduct of fee", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})
		})

		When("target domain (domain1) has a value but the domain does not already exist", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: util.String("10"),
					Stakes:  state.BareAccountStakes(),
					Nonce:   1,
				})

				update := map[string]string{"domain1": "target_update"}
				err = txLogic.execUpdateNamespaceDomains(sender.PubKey().MustBytes32(), nsName, "1", update, 0)
				Expect(err).To(BeNil())
			})

			Specify("that domain 'domain1' was added", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Domains["domain1"]).To(Equal("target_update"))
			})

			Specify("that sender account is deduct of fee", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})
		})

		When("target domain (domain1) has no value and the domain already exist", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: util.String("10"),
					Stakes:  state.BareAccountStakes(),
					Nonce:   1,
				})

				logic.NamespaceKeeper().Update(nsName, &state.Namespace{
					Domains: map[string]string{
						"domain1": "target",
						"domain2": "other_target",
					},
				})

				update := map[string]string{"domain1": ""}
				err = txLogic.execUpdateNamespaceDomains(sender.PubKey().MustBytes32(), nsName, "1", update, 0)
				Expect(err).To(BeNil())
			})

			Specify("that domain 'domain1' has been removed", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Domains).ToNot(HaveKey("domain1"))
			})

			Specify("that sender account is deduct of fee", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})
		})
	})
})
