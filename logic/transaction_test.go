package logic

import (
	"os"

	"github.com/makeos/mosdef/params"

	"github.com/golang/mock/gomock"
	abcitypes "github.com/tendermint/tendermint/abci/types"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/makeos/mosdef/testutil/mockutil"
)

var _ = Describe("Transaction", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.EngineConfig
	var logic *Logic
	var txLogic *Transaction
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)
	var mockLogic *mockutil.MockObjects

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = mockutil.MockLogic(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".PrepareExec", func() {

		Context("when tx bytes are not decodeable to types.Transaction", func() {
			It("should return err='failed to decode transaction from hex to bytes'", func() {
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: []byte([]byte("invalid_hex")),
				})
				resp := txLogic.PrepareExec(req, 1)
				Expect(resp.Code).To(Equal(types.ErrCodeFailedDecode))
				Expect(resp.Log).To(Equal("failed to decode transaction from bytes"))
			})
		})

		Context("when tx is invalid", func() {
			It("should return err='tx failed validation...'", func() {
				tx := types.NewBareTx(1)
				tx.Sig = []byte("sig")
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: tx.Bytes(),
				})
				resp := txLogic.PrepareExec(req, 1)
				Expect(resp.Code).To(Equal(types.ErrCodeFailedDecode))
				Expect(resp.Log).To(ContainSubstring("tx failed validation"))
			})
		})
	})

	Describe(".Exec", func() {
		Context("with unknown transaction type", func() {
			It("should return err", func() {
				tx := &types.Transaction{Type: 100}
				err := logic.Tx().Exec(tx, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown transaction type"))
			})
		})
	})

	Describe("CanExecCoinTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='sender's spendable account balance is insufficient'", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeValidatorTicket, sender.PubKey(), util.String("100"), util.String("0"), 1, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:sender's spendable account balance is insufficient"))
			})
		})

		Context("when nonce is invalid", func() {
			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeValidatorTicket, sender.PubKey(), util.String("100"), util.String("0"), 3, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:tx has invalid nonce (3), expected (1)"))
			})
		})

		Context("when sender account has sufficient spendable balance", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("1000"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeValidatorTicket, sender.PubKey(), util.String("100"), util.String("0"), 1, 0)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".execCoinTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)
		var recipientKey = crypto.NewKeyFromIntSeed(2)

		Context("when sender has bal=100, recipient has bal=10", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("100"),
					Stakes:  types.BareAccountStakes(),
				})
				logic.AccountKeeper().Update(recipientKey.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					senderPubKey := util.String(sender.PubKey().Base58())
					err := txLogic.execCoinTransfer(senderPubKey, recipientKey.Addr(), util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 89 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("89")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})

				Specify("that recipient balance is equal to 20 and nonce=0", func() {
					recipientAcct := logic.AccountKeeper().GetAccount(recipientKey.Addr())
					Expect(recipientAcct.Balance).To(Equal(util.String("20")))
					Expect(recipientAcct.Nonce).To(Equal(uint64(0)))
				})
			})
		})

		Context("when sender and recipient are the same; with bal=100", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("100"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					senderPubKey := util.String(sender.PubKey().Base58())
					err := txLogic.execCoinTransfer(senderPubKey, sender.Addr(), util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 99 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("99")))
					Expect(senderAcct.Nonce).To(Equal(uint64(1)))
				})
			})
		})
	})

	Describe(".execSetDelegatorCommission", func() {
		var sender = crypto.NewKeyFromIntSeed(1)
		var senderAcct *types.Account
		Context("when tx has incorrect nonce", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance:             util.String("10"),
					Stakes:              types.BareAccountStakes(),
					DelegatorCommission: 15.4,
				})
				spk := util.String(sender.PubKey().Base58())
				err := txLogic.execSetDelegatorCommission(spk, util.String("23.5"), util.String("2"), 0)
				Expect(err).To(BeNil())
				senderAcct = logic.AccountKeeper().GetAccount(sender.Addr())
			})

			It("should successfully set new commission", func() {
				Expect(senderAcct.DelegatorCommission).To(Equal(23.5))
			})

			It("should increment nonce", func() {
				Expect(senderAcct.Nonce).To(Equal(uint64(1)))
			})

			It("should have balance of 8", func() {
				Expect(senderAcct.Balance).To(Equal(util.String("8")))
			})
		})
	})

	Describe(".addStake", func() {

		Context("for tx of type TxTypeValidatorTicket & TxTypeStorerTicket", func() {
			Context("when [current block height]=1; an account balance is 100 with validator stake entry of value=50, unbondHeight=1", func() {
				BeforeEach(func() {
					stakes := types.BareAccountStakes()
					stakes.Add(types.StakeTypeValidator, util.String("50"), 1)
					acct := &types.Account{
						Balance: util.String("100"),
						Stakes:  stakes,
					}
					logic.AccountKeeper().Update(sender.Addr(), acct)
					Expect(acct.GetBalance()).To(Equal(util.String("100")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("100")))
				})

				Specify("that when another stake entry value=10, unbondHeight=100 is added with fee=1 then spendable balance = 89", func() {
					senderPubKey := util.String(sender.PubKey().Base58())
					err := txLogic.addStake(types.TxTypeValidatorTicket, senderPubKey, util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
					acct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(acct.GetBalance()).To(Equal(util.String("99")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("89")))
				})
			})

			Context("when [current block height]=1; an account balance is 100 with validator stake entry of value=50, unbondHeight=100", func() {
				BeforeEach(func() {
					stakes := types.BareAccountStakes()
					stakes.Add(types.StakeTypeValidator, util.String("50"), 100)
					acct := &types.Account{
						Balance: util.String("100"),
						Stakes:  stakes,
					}
					logic.AccountKeeper().Update(sender.Addr(), acct)
					Expect(acct.GetBalance()).To(Equal(util.String("100")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("50")))
				})

				Specify("that when another stake entry value=10, unbondHeight=100 is added with fee=1 then spendable balance = 39", func() {
					senderPubKey := util.String(sender.PubKey().Base58())
					err := txLogic.addStake(types.TxTypeValidatorTicket, senderPubKey, util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
					acct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(acct.GetBalance()).To(Equal(util.String("99")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("39")))
				})
			})
		})

		Context("types.TxTypeStorerTicket", func() {
			Context("add a stake with value=10", func() {
				var senderPubKey util.String

				BeforeEach(func() {
					acct := &types.Account{Balance: util.String("100"), Stakes: types.BareAccountStakes()}
					logic.AccountKeeper().Update(sender.Addr(), acct)
					Expect(acct.GetBalance()).To(Equal(util.String("100")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("100")))

					senderPubKey = util.String(sender.PubKey().Base58())
					err := txLogic.addStake(types.TxTypeStorerTicket, senderPubKey, util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
				})

				It("should add a stake entry with unbond height set to 0", func() {
					acct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(acct.Stakes).To(HaveLen(1))
					Expect(acct.Stakes.TotalStaked(1)).To(Equal(util.String("10")))
					Expect(acct.Stakes[types.StakeTypeStorer+"0"].UnbondHeight).To(Equal(uint64(0)))
				})
			})
		})
	})

	Describe(".execUnbond", func() {

		When("ticket is unknown", func() {
			var senderPubKey util.String
			var err error

			BeforeEach(func() {
				acct := types.BareAccount()
				logic.AccountKeeper().Update(sender.Addr(), acct)

				txLogic.logic = mockLogic.Logic

				mockLogic.AccountKeeper.EXPECT().GetAccount(sender.Addr(), int64(0)).Return(acct)
				mockLogic.TicketManager.EXPECT().GetByHash(gomock.Any()).Return(nil)

				senderPubKey = util.String(sender.PubKey().Base58())
				err = txLogic.execUnbond([]byte("ticket_id"), senderPubKey, util.String(1), 0)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='ticket not found'", func() {
				Expect(err.Error()).To(Equal("ticket not found"))
			})
		})

		When("storer stake=100, unbondHeight=0, balance=1000 and fee=1", func() {
			var senderPubKey util.String
			var err error
			var acct *types.Account

			BeforeEach(func() {
				params.NumBlocksInStorerThawPeriod = 200

				txLogic.logic = mockLogic.Logic

				acct = types.BareAccount()
				acct.Balance = util.String("1000")
				acct.Stakes.Add(types.StakeTypeStorer, "100", 0)

				mockLogic.AccountKeeper.EXPECT().GetAccount(sender.Addr(), int64(1)).Return(acct)

				returnTicket := &types.Ticket{Hash: "ticket_id", Value: "100"}
				mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)

				mockLogic.AccountKeeper.EXPECT().Update(sender.Addr(), acct)

				senderPubKey = util.String(sender.PubKey().Base58())
				err = txLogic.execUnbond([]byte(returnTicket.Hash), senderPubKey, util.String("1"), 1)
				Expect(err).To(BeNil())
			})

			Specify("that the unbondHeight is 202", func() {
				stake := acct.Stakes.Get("s0")
				Expect(stake.Value.String()).To(Equal("100"))
				Expect(stake.UnbondHeight).To(Equal(uint64(202)))
			})

			Specify("that the nonce is 1", func() {
				Expect(acct.Nonce).To(Equal(uint64(1)))
			})

			Specify("that balance is 999", func() {
				Expect(acct.Balance).To(Equal(util.String("999")))
			})
		})
	})
})
