package burnforswap_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/burnforswap"
	"github.com/make-os/kit/params"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestTransferCoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GasToCoin Suite")
}

var _ = Describe("Contract", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *logic2.Logic
	var ctrl *gomock.Controller
	var sender = ed25519.NewKeyFromIntSeed(1)

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
			ct := burnforswap.NewContract()
			Expect(ct.CanExec(txns.TxTypeBurnForSwap)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		Context("when sender has gas bal=100, coin bal=100", func() {
			BeforeEach(func() {
				params.GasToCoinExRate = 0.02
				acct := &state.Account{Balance: "100", Stakes: state.BareAccountStakes()}
				acct.SetGasBalance("100")
				logic.AccountKeeper().Update(sender.Addr(), acct)
			})

			Context("tx gas=true burn amount=10, fee=1", func() {
				BeforeEach(func() {
					tx := &txns.TxBurnForSwap{
						Amount:   "10",
						Gas:      true,
						TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					}
					ct := burnforswap.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
					Expect(err).To(BeNil())
				})

				Specify("that sender balance=99, gas balance=90 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("99")))
					Expect(senderAcct.GetGasBalance()).To(Equal(util.String("90")))
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
				})
			})

			Context("tx gas=false burn amount=10, fee=1", func() {
				BeforeEach(func() {
					tx := &txns.TxBurnForSwap{
						Amount:   "10",
						Gas:      false,
						TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					}
					ct := burnforswap.NewContract().Init(logic, tx, 0)
					err = ct.Exec()
					Expect(err).To(BeNil())
				})

				Specify("that sender balance=89, gas balance=100 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().Get(sender.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("89")))
					Expect(senderAcct.GetGasBalance()).To(Equal(util.String("100")))
					Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
				})
			})
		})
	})
})
