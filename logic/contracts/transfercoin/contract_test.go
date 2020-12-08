package transfercoin_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/transfercoin"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	crypto2 "github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestTransferCoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TransferCoin Suite")
}

var _ = Describe("Contract", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = ed25519.NewKeyFromIntSeed(1)
	var recipientKey = ed25519.NewKeyFromIntSeed(2)

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
				err = ct.DryExec(123, false)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("unexpected address type"))
			})
		})

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='sender's spendable account balance is insufficient'", func() {
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 1}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 1)
				err := ct.DryExec(sender.PubKey(), false)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value+fee, msg:sender's spendable account balance is insufficient"))
			})
		})

		Context("when nonce is not the next nonce and allowNonceGap=false", func() {
			It("should return error", func() {
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 3}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 1)
				err := ct.DryExec(sender.PubKey(), false)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:tx has an invalid nonce (3); expected (1)"))
			})
		})

		Context("when nonce is lower than or equal to the current nonce and allowNonceGap=true", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "1000", Nonce: 10, Stakes: state.BareAccountStakes()})
			})

			It("should return error when lower", func() {
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 3}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 1)
				err := ct.DryExec(sender.PubKey(), true)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:tx has an invalid nonce (3); expected a nonce that is greater than the current account nonce (10)"))
			})

			It("should return error when equal", func() {
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "100"}, TxCommon: &txns.TxCommon{Fee: "0", Nonce: 10}}
				ct := transfercoin.NewContract()
				ct.Init(logic, tx, 1)
				err := ct.DryExec(sender.PubKey(), true)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:tx has an invalid nonce (10); expected a nonce that is greater than the current account nonce (10)"))
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
				err := ct.DryExec(sender.PubKey(), false)
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
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
				})

				Specify("that recipient balance is equal to 20 and nonce=0", func() {
					recipientAcct := logic.AccountKeeper().Get(recipientKey.Addr())
					Expect(recipientAcct.Balance).To(Equal(util.String("20")))
					Expect(recipientAcct.Nonce.UInt64()).To(Equal(uint64(0)))
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
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
				})
			})
		})

		When("recipient address is a user namespace path with a user account target", func() {
			ns := "namespace"
			var senderNamespaceURI = identifier.Address(ns + "/domain")

			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
				nsObj := &state.Namespace{Domains: map[string]string{"domain": "a/" + sender.Addr().String()}}
				logic.NamespaceKeeper().Update(crypto2.MakeNamespaceHash(ns), nsObj)
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
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
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
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
				})
			})
		})

		When("recipient address is a user namespace path with a repo account target", func() {
			ns := "namespace"
			var senderNamespaceURI = identifier.Address(ns + "/domain")
			var repoName = "repo1"

			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
				nsObj := &state.Namespace{Domains: map[string]string{"domain": "r/" + repoName}}
				logic.NamespaceKeeper().Update(crypto2.MakeNamespaceHash(ns), nsObj)
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
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
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
					recipient := identifier.Address("r/" + repoName)
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
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
				})

				Specify("that the repo has a balance=10", func() {
					repo := logic.RepoKeeper().Get(repoName)
					Expect(repo.GetBalance()).To(Equal(util.String("10")))
				})
			})
		})

		When("recipient address is a partial user namespace", func() {
			var recipient = identifier.Address("ns1/")

			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
					TxRecipient: &txns.TxRecipient{To: recipient},
					TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
				ct := transfercoin.NewContract().Init(logic, tx, 0)
				err = ct.Exec()
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("namespace not found"))
			})
		})

		When("recipient address is a partial native namespace", func() {
			var recipient = identifier.Address("r/")

			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Stakes: state.BareAccountStakes()})
				tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: "10"},
					TxRecipient: &txns.TxRecipient{To: recipient},
					TxCommon:    &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()}}
				ct := transfercoin.NewContract().Init(logic, tx, 0)
				err = ct.Exec()
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("recipient account not found"))
			})
		})
	})
})
