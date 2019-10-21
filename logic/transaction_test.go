package logic

import (
	"os"

	"github.com/golang/mock/gomock"
	abcitypes "github.com/tendermint/tendermint/abci/types"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transaction", func() {
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var logic *Logic
	var txLogic *Transaction
	var sysLogic *System
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		logic = New(c, cfg)
		txLogic = &Transaction{logic: logic}
		sysLogic = &System{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
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

	Describe("CanTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)
		var receiver = crypto.NewKeyFromIntSeed(2)

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='invalid recipient address...'", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeGetTicket, sender.PubKey(),
					receiver.Addr(), util.String("100"), util.String("0"), 1, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("sender's spendable account balance is insufficient"))
			})
		})

		Context("when sender account has sufficient spendable balance", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("1000"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should not return err='invalid recipient address...'", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeGetTicket, sender.PubKey(),
					receiver.Addr(), util.String("100"), util.String("0"), 1, 1)
				Expect(err).To(BeNil())
			})
		})

		Context("when tx type is types.TxTypeGetTicket", func() {
			It("should not return err='invalid recipient address...'", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeGetTicket, sender.PubKey(),
					util.String("invalid"), util.String("100"),
					util.String("0"), 0, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).NotTo(ContainSubstring("invalid recipient address"))
			})
		})

		Context("tx type is TxTypeGetTicket", func() {
			When("current ticket price = 10; sender's account balance = 5; ticket value = 4", func() {
				BeforeEach(func() {
					params.InitialTicketPrice = 10
					params.NumBlocksPerPriceWindow = 100
					params.PricePercentIncrease = 0.2
					price := sysLogic.GetCurValidatorTicketPrice()
					Expect(price).To(Equal(float64(10)))

					logic.AccountKeeper().Update(sender.Addr(), &types.Account{
						Balance: util.String("5"),
						Stakes:  types.BareAccountStakes(),
					})
				})

				Specify("that err='sender's spendable account balance is insufficient to cover ticket price (10.000000)' is returned", func() {
					err := txLogic.CanExecCoinTransfer(types.TxTypeGetTicket, sender.PubKey(),
						"", util.String("4"),
						util.String("0"), 1, 1)
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("sender's spendable account balance is insufficient to cover ticket price (10.000000)"))
				})
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
					err := txLogic.execCoinTransfer(senderPubKey, recipientKey.Addr(), util.String("10"), util.String("1"), 1, 1)
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
					err := txLogic.execCoinTransfer(senderPubKey, sender.Addr(), util.String("10"), util.String("1"), 1, 1)
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

	Describe(".setDelegatorCommission", func() {
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
				err := txLogic.setDelegatorCommission(spk, util.String("23.5"))
				Expect(err).To(BeNil())
				senderAcct = logic.AccountKeeper().GetAccount(sender.Addr())
			})

			It("should successfully set new commission", func() {
				Expect(senderAcct.DelegatorCommission).To(Equal(23.5))
			})

			It("should increment nonce", func() {
				Expect(senderAcct.Nonce).To(Equal(uint64(1)))
			})
		})
	})

	Describe(".execValidatorStake", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

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
				err := txLogic.execValidatorStake(senderPubKey, util.String("10"), util.String("1"), 1, 1)
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
				err := txLogic.execValidatorStake(senderPubKey, util.String("10"), util.String("1"), 1, 1)
				Expect(err).To(BeNil())
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("99")))
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("39")))
			})
		})
	})

	// Describe(".unexecValidatorStake", func() {
	// 	When("error occurred when fetching ticket", func() {
	// 		var err error
	// 		BeforeEach(func() {
	// 			mockTicketMgr := mocks.NewMockTicketManager(ctrl)
	// 			mockTicketMgr.EXPECT().QueryOne(gomock.Any()).Return(nil, fmt.Errorf("error"))
	// 			txLogic.logic.SetTicketManager(mockTicketMgr)
	// 			err = txLogic.unExecValidatorStake(util.String("pubkey"), []byte("ticketID"))
	// 		})

	// 		It("should return err='failed to get ticket: error'", func() {
	// 			Expect(err).ToNot(BeNil())
	// 			Expect(err.Error()).To(Equal("failed to get ticket: error"))
	// 		})
	// 	})

	// 	When("an account has a balance=100 with stake entry value=50, unbondHeight=100", func() {
	// 		var sender = crypto.NewKeyFromIntSeed(1)
	// 		BeforeEach(func() {
	// 			stakes := types.BareAccountStakes()
	// 			stakes.Add(types.StakeTypeValidator, util.String("50"), 100)
	// 			acct := &types.Account{
	// 				Balance: util.String("100"),
	// 				Stakes:  stakes,
	// 			}
	// 			logic.AccountKeeper().Update(sender.Addr(), acct)
	// 			Expect(acct.GetBalance()).To(Equal(util.String("100")))
	// 			Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("50")))
	// 		})

	// 		When("ticket value is 50", func() {
	// 			var acct *types.Account
	// 			BeforeEach(func() {
	// 				mockTicketMgr := mocks.NewMockTicketManager(ctrl)
	// 				mockTicketMgr.EXPECT().QueryOne(gomock.Any()).Return(&types.Ticket{
	// 					Value: "50",
	// 				}, nil)
	// 				mockTicketMgr.EXPECT().MarkAsUnbonded(gomock.Any()).Return(nil)
	// 				txLogic.logic.SetTicketManager(mockTicketMgr)
	// 				err := txLogic.unExecValidatorStake(util.String(sender.PubKey().Base58()), []byte("ticketID"))
	// 				Expect(err).To(BeNil())
	// 				acct = logic.AccountKeeper().GetAccount(sender.Addr())
	// 			})

	// 			It("should subtract 50 from the account's validator stake, leave 0 as update value", func() {
	// 				Expect(acct.Stakes.Get(types.StakeTypeValidator)).To(Equal(util.String("0")))
	// 			})

	// 			It("should increment nonce", func() {
	// 				Expect(acct.Nonce).To(Equal(uint64(1)))
	// 			})
	// 		})

	// 		When("failed to mark ticket as unbonded", func() {
	// 			var err error
	// 			BeforeEach(func() {
	// 				mockTicketMgr := mocks.NewMockTicketManager(ctrl)
	// 				mockTicketMgr.EXPECT().QueryOne(gomock.Any()).Return(&types.Ticket{
	// 					Value: "50",
	// 				}, nil)
	// 				mockTicketMgr.EXPECT().MarkAsUnbonded(gomock.Any()).Return(fmt.Errorf("error unbonding"))
	// 				txLogic.logic.SetTicketManager(mockTicketMgr)
	// 				err = txLogic.unExecValidatorStake(util.String(sender.PubKey().Base58()), []byte("ticketID"))
	// 			})

	// 			It("should return err='failed to unbond ticket: error unbonding'", func() {
	// 				Expect(err).ToNot(BeNil())
	// 				Expect(err.Error()).To(Equal("failed to unbond ticket: error unbonding"))
	// 			})
	// 		})
	// 	})
	// })
})
