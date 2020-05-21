package transfercoin_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts/transfercoin"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("CoinTransferContract", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)
	var recipientKey = crypto.NewKeyFromIntSeed(2)

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
			ct := transfercoin.NewContract()
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeRegisterPushKey)).To(BeFalse())
		})
	})

	Describe(".DryExec", func() {

		Context("when sender variable type is not valid", func() {
			It("should return type error", func() {
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 1}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 1)
				err = ct.DryExec(123)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("unexpected address type"))
			})
		})

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='sender's spendable account balance is insufficient'", func() {
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 1}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 1)
				err := ct.DryExec(sender.PubKey())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:sender's spendable account balance is insufficient"))
			})
		})

		Context("when nonce is invalid", func() {
			It("should return no error", func() {
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 3}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 1)
				err := ct.DryExec(sender.PubKey())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:tx has invalid nonce (3); expected (1)"))
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
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 1}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 0)
				err := ct.DryExec(sender.PubKey())
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".Exec", func() {

		Context("when sender has bal=100, recipient has bal=10", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
				logic.AccountKeeper().Update(recipientKey.Addr(), &state.Account{Balance: "10", Stakes: state.BareAccountStakes()})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
						TxRecipient: &txns.TxRecipient{To: recipientKey.Addr()},
						TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
					ct := transfercoin.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
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
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
						TxRecipient: &txns.TxRecipient{To: sender.Addr()},
						TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
					ct := transfercoin.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
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
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
				nsObj := &state.Namespace{Domains: map[string]string{"domain": "a/" + sender.Addr().String()}}
				logic.NamespaceKeeper().Update(util.HashNamespace(ns), nsObj)
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
						TxRecipient: &txns.TxRecipient{To: senderNamespaceURI},
						TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
					ct := transfercoin.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
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
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					recipient := "a/" + sender.Addr()
					tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
						TxRecipient: &txns.TxRecipient{To: recipient},
						TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
					ct := transfercoin.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
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
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
				nsObj := &state.Namespace{Domains: map[string]string{"domain": "r/" + repoName}}
				logic.NamespaceKeeper().Update(util.HashNamespace(ns), nsObj)
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
						TxRecipient: &txns.TxRecipient{To: senderNamespaceURI},
						TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
					ct := transfercoin.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
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

		When("recipient address is a prefixed repo account address", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				var repoName = "repo1"

				BeforeEach(func() {
					recipient := util.Address("r/" + repoName)
					tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
						TxRecipient: &txns.TxRecipient{To: recipient},
						TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
					ct := transfercoin.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
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
