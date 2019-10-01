package logic

import (
	"os"

	abcitypes "github.com/tendermint/tendermint/abci/types"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transaction", func() {
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var state *tree.SafeTree
	var logic *Logic
	var txLogic *Transaction
	var sysLogic *System

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = New(c, state, cfg)
		txLogic = &Transaction{logic: logic}
		sysLogic = &System{logic: logic}
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
				resp := txLogic.PrepareExec(req)
				Expect(resp.Code).To(Equal(ErrCodeFailedDecode))
				Expect(resp.Log).To(Equal("failed to decode transaction from bytes"))
			})
		})

		Context("when tx is invalid", func() {
			It("should return err='tx failed validation...'", func() {
				tx := &types.Transaction{Sig: []byte("sig")}
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: tx.Bytes(),
				})
				resp := txLogic.PrepareExec(req)
				Expect(resp.Code).To(Equal(ErrCodeFailedDecode))
				Expect(resp.Log).To(ContainSubstring("tx failed validation"))
			})
		})
	})

	Describe(".Exec", func() {
		Context("with unknown transaction type", func() {
			It("should return err", func() {
				tx := &types.Transaction{Type: 100}
				err := logic.Tx().Exec(tx)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown transaction type"))
			})
		})
	})

	Describe("CanTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when tx type is types.TxTypeTicketValidator", func() {
			It("should not return err='invalid recipient address...'", func() {
				err := txLogic.CanTransferCoin(types.TxTypeTicketValidator, sender.PubKey(),
					util.String("invalid"), util.String("100"),
					util.String("0"), 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).NotTo(ContainSubstring("invalid recipient address"))
			})
		})

		Context("tx type is TxTypeTicketValidator", func() {
			When("current ticket price = 10; sender's account balance = 5; ticket value = 4", func() {
				BeforeEach(func() {
					params.InitialTicketPrice = 10
					params.NumBlocksPerPriceWindow = 100
					params.PricePercentIncrease = 0.2
					price := sysLogic.GetCurTicketPrice()
					Expect(price).To(Equal(float64(10)))

					logic.AccountKeeper().Update(sender.Addr(), &types.Account{
						Balance: util.String("5"),
						Stakes:  types.BareAccountStakes(),
					})
				})

				Specify("that err='sender's spendable account balance is insufficient to cover ticket price (10.000000)' is returned", func() {
					err := txLogic.CanTransferCoin(types.TxTypeTicketValidator, sender.PubKey(),
						"", util.String("4"),
						util.String("0"), 1)
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("sender's spendable account balance is insufficient to cover ticket price (10.000000)"))
				})
			})
		})
	})

	Describe(".transferCoin", func() {
		var sender = crypto.NewKeyFromIntSeed(1)
		var recipientKey = crypto.NewKeyFromIntSeed(2)

		Context("when sender public key is not valid", func() {
			It("should return err='invalid sender public key...'", func() {
				err := txLogic.transferCoin(util.String("invalid"), util.String(""),
					util.String("100"), util.String("0"), 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid sender public key"))
			})
		})

		Context("when recipient public key is not valid", func() {
			It("should return err='invalid recipient address...'", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.transferCoin(senderPubKey, util.String("invalid"), util.String("100"),
					util.String("0"), 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid recipient address"))
			})
		})

		Context("when transaction has a nonce > currentNonce + 1", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("1"),
					Nonce:   0,
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return err='sender's account balance is insufficient'", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.transferCoin(senderPubKey, recipientKey.Addr(), util.String("11"), util.String("1"), 2)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("tx has invalid nonce (2), expected (1)"))
			})
		})

		Context("when transaction has a nonce < currentNonce", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("1"),
					Nonce:   2,
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return err='sender's account balance is insufficient'", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.transferCoin(senderPubKey, recipientKey.Addr(), util.String("11"), util.String("1"), 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("tx has invalid nonce (1), expected (3)"))
			})
		})

		Context("when transaction has a nonce == currentNonce", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("1"),
					Nonce:   2,
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return err='sender's account balance is insufficient'", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.transferCoin(senderPubKey, recipientKey.Addr(), util.String("11"), util.String("1"), 2)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("tx has invalid nonce (2), expected (3)"))
			})
		})

		Context("when sender account has insufficient balance", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return err='sender's account balance is insufficient'", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.transferCoin(senderPubKey, recipientKey.Addr(), util.String("11"), util.String("1"), 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("sender's spendable account balance is insufficient"))
			})
		})

		Context("when sender account has balance=10 and one staked balance = 1", func() {
			BeforeEach(func() {
				stakes := types.BareAccount().Stakes
				stakes.Add("s1", util.String("1"))
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  stakes,
				})
			})

			It("should return err='sender's account balance is insufficient' when tx.value = 9 and fee=1", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.transferCoin(senderPubKey, recipientKey.Addr(), util.String("9"), util.String("1"), 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("sender's spendable account balance is insufficient"))
			})
		})

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
					err := txLogic.transferCoin(senderPubKey, recipientKey.Addr(), util.String("10"), util.String("1"), 1)
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
	})

	Describe(".stakeValidatorCoin", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when sender public key is invalid", func() {
			It("should return err='invalid sender public key...'", func() {
				err := txLogic.stakeValidatorCoin(util.String("invalid"), util.String("10"), util.String("1"), 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid sender public key"))
			})
		})

		Context("when tx has incorrect nonce", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return err='tx has invalid...'", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.stakeValidatorCoin(senderPubKey, util.String("100"), util.String("1"), 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("tx has invalid nonce"))
			})
		})

		Context("when account balance is insufficient", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return err='sender's account balance is insufficient'", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.stakeValidatorCoin(senderPubKey, util.String("100"), util.String("1"), 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("sender's spendable account balance is insufficient"))
			})
		})

		Context("when account balance is 100 and 50 is validator stake", func() {
			BeforeEach(func() {
				stakes := types.BareAccountStakes()
				stakes.Add(types.StakeNameValidator, util.String("50"))
				acct := &types.Account{
					Balance: util.String("100"),
					Stakes:  stakes,
				}
				logic.AccountKeeper().Update(sender.Addr(), acct)
				Expect(acct.GetBalance()).To(Equal(util.String("100")))
				Expect(acct.GetSpendableBalance()).To(Equal(util.String("50")))
			})

			Specify("that staking value=10 with fee=1 will make spendable balance = 39", func() {
				senderPubKey := util.String(sender.PubKey().Base58())
				err := txLogic.stakeValidatorCoin(senderPubKey, util.String("10"), util.String("1"), 1)
				Expect(err).To(BeNil())
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("99")))
				Expect(acct.GetSpendableBalance()).To(Equal(util.String("39")))
			})
		})
	})
})
