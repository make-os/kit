package purchaseticket_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts/purchaseticket"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("TicketPurchaseContract", func() {
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
			ct := purchaseticket.NewContract()
			Expect(ct.CanExec(core.TxTypeValidatorTicket)).To(BeTrue())
			Expect(ct.CanExec(core.TxTypeHostTicket)).To(BeTrue())
			Expect(ct.CanExec(core.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec (TxTypeValidatorTicket)", func() {
		When("when chainHeight=1; and "+
			"account balance is 100 and "+
			"existing validator stake entry of value=50 and "+
			"unbondHeight=1", func() {

			BeforeEach(func() {
				stakes := state.BareAccountStakes()
				stakes.Add(state.StakeTypeValidator, "50", 1)
				acct := &state.Account{Balance: util.String("100"), Stakes: stakes}
				logic.AccountKeeper().Update(sender.Addr(), acct)
				Expect(acct.GetBalance()).To(Equal(util.String("100")))
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("100")))

				err = purchaseticket.NewContract().Init(logic, &core.TxTicketPurchase{
					TxType:   &core.TxType{Type: core.TxTypeValidatorTicket},
					TxValue:  &core.TxValue{Value: "10"},
					TxCommon: &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that spendable balance is 89", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("89")))
			})

			Specify("that balance is 99", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("99")))
			})
		})

		When("chainHeight=1 and "+
			"account balance is 100 and "+
			"existing validator stake entry of value=50, "+
			"unbondHeight=100", func() {

			BeforeEach(func() {
				stakes := state.BareAccountStakes()
				stakes.Add(state.StakeTypeValidator, "50", 100)
				acct := &state.Account{Balance: util.String("100"), Stakes: stakes}
				logic.AccountKeeper().Update(sender.Addr(), acct)
				Expect(acct.GetBalance()).To(Equal(util.String("100")))
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("50")))

				err = purchaseticket.NewContract().Init(logic, &core.TxTicketPurchase{
					TxType:   &core.TxType{Type: core.TxTypeValidatorTicket},
					TxValue:  &core.TxValue{Value: "10"},
					TxCommon: &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that spendable balance is 39", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("39")))
			})

			Specify("that balance is 99", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("99")))
			})
		})
	})

	Context(".Exec (TxTypeHostTicket)", func() {
		When("account balance is 100 and ticket value is 10", func() {
			BeforeEach(func() {
				acct := &state.Account{Balance: util.String("100"), Stakes: state.BareAccountStakes()}
				logic.AccountKeeper().Update(sender.Addr(), acct)
				Expect(acct.GetBalance()).To(Equal(util.String("100")))
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("100")))

				err = purchaseticket.NewContract().Init(logic, &core.TxTicketPurchase{
					TxType:   &core.TxType{Type: core.TxTypeHostTicket},
					TxValue:  &core.TxValue{Value: "10"},
					TxCommon: &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add a stake entry with unbond height set to 0", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Stakes).To(HaveLen(1))
				Expect(acct.Stakes[state.StakeTypeHost+"0"].UnbondHeight).To(Equal(uint64(0)))
			})

			Specify("that total staked is 10", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Stakes.TotalStaked(1)).To(Equal(util.String("10")))
			})

			Specify("that total balance is 99", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("99")))
			})

			Specify("that total spendable balance is 89 at chainHeight=1", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("89")))
			})

			Specify("that total spendable balance is 89 at chainHeight=1000", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetSpendableBalance(1000)).To(Equal(util.String("89")))
			})
		})
	})
})
