package unbondticket_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/unbondticket"
	"github.com/make-os/kit/params"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	tickettypes "github.com/make-os/kit/ticket/types"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestUnbondTicket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UnbondTicket Suite")
}

var _ = Describe("Contract", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = ed25519.NewKeyFromIntSeed(1)
	var mockLogic *testutil.MockObjects

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB()
		logic = logic2.New(appDB, stateTreeDB, cfg)
		err := logic.SysKeeper().SaveBlockInfo(&state.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
		mockLogic = testutil.Mocks(ctrl)
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
			Expect(ct.CanExec(txns.TxTypeUnbondHostTicket)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		When("ticket is unknown", func() {
			var err error
			BeforeEach(func() {
				acct := state.NewBareAccount()
				logic.AccountKeeper().Update(sender.Addr(), acct)

				mockLogic.AccountKeeper.EXPECT().Get(sender.Addr(), uint64(0)).Return(acct)
				mockLogic.TicketManager.EXPECT().GetByHash(gomock.Any()).Return(nil)

				err = unbondticket.NewContract().Init(mockLogic.Logic, &txns.TxTicketUnbond{
					TicketHash: util.StrToHexBytes("ticket_id"),
					TxCommon:   &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
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

				acct = state.NewBareAccount()
				acct.Balance = "1000"
				acct.Stakes.Add(state.StakeTypeHost, "100", 0)
				mockLogic.AccountKeeper.EXPECT().Update(sender.Addr(), acct)
				mockLogic.AccountKeeper.EXPECT().Get(sender.Addr(), uint64(1)).Return(acct)

				returnTicket := &tickettypes.Ticket{Hash: util.StrToHexBytes("ticket_id"), Value: "100"}
				mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)

				err = unbondticket.NewContract().Init(mockLogic.Logic, &txns.TxTicketUnbond{
					TicketHash: returnTicket.Hash,
					TxCommon:   &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
				}, 1).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that the unbondHeight is 202", func() {
				stake := acct.Stakes.Get("s0")
				Expect(stake.Value.String()).To(Equal("100"))
				Expect(stake.UnbondHeight.UInt64()).To(Equal(uint64(202)))
			})

			Specify("that the nonce is 1", func() {
				Expect(acct.Nonce.UInt64()).To(Equal(uint64(1)))
			})

			Specify("that balance is 999", func() {
				Expect(acct.Balance).To(Equal(util.String("999")))
			})
		})
	})
})
