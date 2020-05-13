package unbondticket_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts/unbondticket"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	types3 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("TicketUnbondContract", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)
	var mockLogic *testutil.MockObjects

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = logic2.New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
		mockLogic = testutil.MockLogic(ctrl)
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
			ct := unbondticket.NewContract()
			Expect(ct.CanExec(core.TxTypeUnbondHostTicket)).To(BeTrue())
			Expect(ct.CanExec(core.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		When("ticket is unknown", func() {
			var err error
			BeforeEach(func() {
				acct := state.BareAccount()
				logic.AccountKeeper().Update(sender.Addr(), acct)

				mockLogic.AccountKeeper.EXPECT().Get(sender.Addr(), uint64(0)).Return(acct)
				mockLogic.TicketManager.EXPECT().GetByHash(gomock.Any()).Return(nil)

				err = unbondticket.NewContract().Init(mockLogic.Logic, &core.TxTicketUnbond{
					TicketHash: util.StrToBytes32("ticket_id"),
					TxCommon:   &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
				}, 0).Exec()
				Expect(err).ToNot(BeNil())
			})

			It("should return err='ticket not found'", func() {
				Expect(err.Error()).To(Equal("ticket not found"))
			})
		})

		When("account stake=100, unbondHeight=0, balance=1000 and fee=1", func() {
			var err error
			var acct *state.Account

			BeforeEach(func() {
				params.NumBlocksInHostThawPeriod = 200

				acct = state.BareAccount()
				acct.Balance = "1000"
				acct.Stakes.Add(state.StakeTypeHost, "100", 0)
				mockLogic.AccountKeeper.EXPECT().Update(sender.Addr(), acct)
				mockLogic.AccountKeeper.EXPECT().Get(sender.Addr(), uint64(1)).Return(acct)

				returnTicket := &types3.Ticket{Hash: util.StrToBytes32("ticket_id"), Value: "100"}
				mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)

				err = unbondticket.NewContract().Init(mockLogic.Logic, &core.TxTicketUnbond{
					TicketHash: returnTicket.Hash,
					TxCommon:   &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
				}, 1).Exec()
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
