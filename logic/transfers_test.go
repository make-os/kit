package logic

import (
	"os"

	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/golang/mock/gomock"

	"gitlab.com/makeos/mosdef/crypto"

	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("Transfers", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var txLogic *Transaction
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe("CanExecCoinTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when sender variable type is not valid", func() {
			It("should panic", func() {
				Expect(func() {
					txLogic.CanExecCoinTransfer(123, "100", "0", 3, 1)
				}).Should(Panic())
			})
		})

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='sender's spendable account balance is insufficient'", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), "100", "0", 1, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:sender's spendable account balance is insufficient"))
			})
		})

		Context("when nonce is invalid", func() {
			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), "100", "0", 3, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:tx has invalid nonce (3), expected (1)"))
			})
		})

		Context("when sender account has sufficient spendable balance", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "1000",
					Stakes:  state.BareAccountStakes(),
				})
			})

			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), "100", "0", 1, 0)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".execCoinTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)
		var recipientKey = crypto.NewKeyFromIntSeed(2)

		Context("when sender has bal=100, recipient has bal=10", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "100",
					Stakes:  state.BareAccountStakes(),
				})
				logic.AccountKeeper().Update(recipientKey.Addr(), &state.Account{
					Balance: "10",
					Stakes:  state.BareAccountStakes(),
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.execCoinTransfer(senderPubKey, recipientKey.Addr(), "10", "1", 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 89 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("89")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})

				Specify("that recipient balance is equal to 20 and nonce=0", func() {
					recipientAcct := logic.AccountKeeper().Get(recipientKey.Addr())
					Expect(recipientAcct.Balance).To(Equal(util.String("20")))
					Expect(recipientAcct.Nonce).To(Equal(uint64(0)))
				})
			})
		})

		Context("when sender and recipient are the same; with bal=100", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "100",
					Stakes:  state.BareAccountStakes(),
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.execCoinTransfer(senderPubKey, sender.Addr(), "10", "1", 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 99 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("99")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})
			})
		})

		When("recipient address is a namespaced URI with a user account target", func() {
			ns := "namespace"
			var senderNamespaceURI = util.Address(ns + "/domain")

			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "100",
					Stakes:  state.BareAccountStakes(),
				})

				logic.NamespaceKeeper().Update(util.HashNamespace(ns), &state.Namespace{
					Domains: map[string]string{
						"domain": "a/" + sender.Addr().String(),
					},
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.execCoinTransfer(senderPubKey, senderNamespaceURI, "10", "1", 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 99 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("99")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})
			})
		})

		When("recipient address is a prefixed user account address", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "100",
					Stakes:  state.BareAccountStakes(),
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					recipient := "a/" + sender.Addr()
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.execCoinTransfer(senderPubKey, recipient, "10", "1", 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 99 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("99")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})
			})
		})

		When("recipient address is a namespace URI with a repo account target", func() {
			ns := "namespace"
			var senderNamespaceURI = util.Address(ns + "/domain")
			var repoName = "repo1"

			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "100",
					Stakes:  state.BareAccountStakes(),
				})

				logic.NamespaceKeeper().Update(util.HashNamespace(ns), &state.Namespace{
					Domains: map[string]string{
						"domain": "r/" + repoName,
					},
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.execCoinTransfer(senderPubKey, senderNamespaceURI, "10", "1", 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 89 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("89")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})

				Specify("that the repo has a balance=10", func() {
					repo := logic.RepoKeeper().Get(repoName)
					Expect(repo.GetBalance()).To(Equal(util.String("10")))
				})
			})
		})

		When("recipient address is a prefixed user account address", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "100",
					Stakes:  state.BareAccountStakes(),
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				var repoName = "repo1"

				BeforeEach(func() {
					recipient := util.Address("r/" + repoName)
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.execCoinTransfer(senderPubKey, recipient, "10", "1", 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 89 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("89")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})

				Specify("that the repo has a balance=10", func() {
					repo := logic.RepoKeeper().Get(repoName)
					Expect(repo.GetBalance()).To(Equal(util.String("10")))
				})
			})
		})
	})
})
