package logic

import (
	types3 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"os"

	"gitlab.com/makeos/mosdef/params"

	"github.com/golang/mock/gomock"

	"gitlab.com/makeos/mosdef/crypto"

	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("Staking", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var txLogic *Transaction
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)
	var mockLogic *testutil.MockObjects

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
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

	Describe(".addStake", func() {

		Context("for tx of type TxTypeValidatorTicket & TxTypeStorerTicket", func() {
			Context("when [current block height]=1; an account balance is 100 with validator stake entry of value=50, unbondHeight=1", func() {
				BeforeEach(func() {
					stakes := state.BareAccountStakes()
					stakes.Add(state.StakeTypeValidator, util.String("50"), 1)
					acct := &state.Account{
						Balance: util.String("100"),
						Stakes:  stakes,
					}
					logic.AccountKeeper().Update(sender.Addr(), acct)
					Expect(acct.GetBalance()).To(Equal(util.String("100")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("100")))
				})

				Specify("that when another stake entry value=10, unbondHeight=100 is added with fee=1 then spendable balance = 89", func() {
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.addStake(core.TxTypeValidatorTicket, senderPubKey, util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
					acct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(acct.GetBalance()).To(Equal(util.String("99")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("89")))
				})
			})

			Context("when [current block height]=1; an account balance is 100 with validator stake entry of value=50, unbondHeight=100", func() {
				BeforeEach(func() {
					stakes := state.BareAccountStakes()
					stakes.Add(state.StakeTypeValidator, util.String("50"), 100)
					acct := &state.Account{
						Balance: util.String("100"),
						Stakes:  stakes,
					}
					logic.AccountKeeper().Update(sender.Addr(), acct)
					Expect(acct.GetBalance()).To(Equal(util.String("100")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("50")))
				})

				Specify("that when another stake entry value=10, unbondHeight=100 is added with fee=1 then spendable balance = 39", func() {
					senderPubKey := sender.PubKey().MustBytes32()
					err := txLogic.addStake(core.TxTypeValidatorTicket, senderPubKey, util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
					acct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(acct.GetBalance()).To(Equal(util.String("99")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("39")))
				})
			})
		})

		Context("types.TxTypeStorerTicket", func() {
			Context("add a stake with value=10", func() {
				var senderPubKey util.Bytes32

				BeforeEach(func() {
					acct := &state.Account{Balance: util.String("100"), Stakes: state.BareAccountStakes()}
					logic.AccountKeeper().Update(sender.Addr(), acct)
					Expect(acct.GetBalance()).To(Equal(util.String("100")))
					Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("100")))

					senderPubKey = sender.PubKey().MustBytes32()
					err := txLogic.addStake(core.TxTypeStorerTicket, senderPubKey, util.String("10"), util.String("1"), 0)
					Expect(err).To(BeNil())
				})

				It("should add a stake entry with unbond height set to 0", func() {
					acct := logic.AccountKeeper().GetAccount(sender.Addr())
					Expect(acct.Stakes).To(HaveLen(1))
					Expect(acct.Stakes.TotalStaked(1)).To(Equal(util.String("10")))
					Expect(acct.Stakes[state.StakeTypeStorer+"0"].(*state.StakeInfo).UnbondHeight).To(Equal(uint64(0)))
				})
			})
		})
	})

	Describe(".execUnbond", func() {

		When("ticket is unknown", func() {
			var senderPubKey util.Bytes32
			var err error

			BeforeEach(func() {
				acct := state.BareAccount()
				logic.AccountKeeper().Update(sender.Addr(), acct)

				txLogic.logic = mockLogic.Logic

				mockLogic.AccountKeeper.EXPECT().GetAccount(sender.Addr(), uint64(0)).Return(acct)
				mockLogic.TicketManager.EXPECT().GetByHash(gomock.Any()).Return(nil)

				senderPubKey = sender.PubKey().MustBytes32()
				err = txLogic.execUnbond(senderPubKey, util.StrToBytes32("ticket_id"), util.String(1), 0)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='ticket not found'", func() {
				Expect(err.Error()).To(Equal("ticket not found"))
			})
		})

		When("storer stake=100, unbondHeight=0, balance=1000 and fee=1", func() {
			var senderPubKey util.Bytes32
			var err error
			var acct *state.Account

			BeforeEach(func() {
				params.NumBlocksInStorerThawPeriod = 200

				txLogic.logic = mockLogic.Logic

				acct = state.BareAccount()
				acct.Balance = util.String("1000")
				acct.Stakes.Add(state.StakeTypeStorer, "100", 0)

				mockLogic.AccountKeeper.EXPECT().GetAccount(sender.Addr(), uint64(1)).Return(acct)

				returnTicket := &types3.Ticket{Hash: util.StrToBytes32("ticket_id"), Value: "100"}
				mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)

				mockLogic.AccountKeeper.EXPECT().Update(sender.Addr(), acct)

				senderPubKey = sender.PubKey().MustBytes32()
				err = txLogic.execUnbond(senderPubKey, returnTicket.Hash, util.String("1"), 1)
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
