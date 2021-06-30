package gasmaker_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/gasmaker"
	"github.com/make-os/kit/params"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestGasMaker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GasMaker Suite")
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
			ct := gasmaker.NewContract()
			Expect(ct.CanExec(txns.TxTypeSubmitWork)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeRegisterPushKey)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		When("sender account does not exist", func() {
			var ct core.SystemContract
			BeforeEach(func() {
				tx := &txns.TxSubmitWork{
					TxCommon:  &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					Epoch:     1,
					WorkNonce: 1,
				}
				ct = gasmaker.NewContract().Init(logic, tx, 0)
			})

			It("should return error", func() {
				err = ct.Exec()
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("sender account not found"))
			})
		})

		When("sender exists and tx fee is zero", func() {
			var ct core.SystemContract
			var tx *txns.TxSubmitWork
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Gas: "0", Stakes: state.BareAccountStakes()})
				tx = &txns.TxSubmitWork{
					TxCommon:  &txns.TxCommon{Fee: "0", SenderPubKey: sender.PubKey().ToPublicKey()},
					Epoch:     1,
					WorkNonce: 1,
				}
				ct = gasmaker.NewContract().Init(logic, tx, 0)
			})

			It("should allocate gas to sender", func() {
				err = ct.Exec()
				Expect(err).To(BeNil())
				senderAcct := logic.AccountKeeper().Get(sender.Addr())
				Expect(senderAcct.Gas).ToNot(BeEmpty())
				Expect(senderAcct.Gas).To(Equal(params.GasReward))
				Expect(senderAcct.Balance.String()).To(Equal("100"))
				Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
			})

			It("should index work nonce", func() {
				err = ct.Exec()
				Expect(err).To(BeNil())
				Expect(logic.SysKeeper().IsWorkNonceRegistered(tx.Epoch, tx.WorkNonce)).To(BeNil())
			})
		})

		When("sender exists and tx fee non-zero and some gas already exists", func() {
			var ct core.SystemContract
			var tx *txns.TxSubmitWork
			var curGasBal = util.String("2000")
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "100", Gas: curGasBal, Stakes: state.BareAccountStakes()})
				tx = &txns.TxSubmitWork{
					TxCommon:  &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					Epoch:     1,
					WorkNonce: 1,
				}
				ct = gasmaker.NewContract().Init(logic, tx, 0)
			})

			It("should return error", func() {
				err = ct.Exec()
				Expect(err).To(BeNil())
				senderAcct := logic.AccountKeeper().Get(sender.Addr())
				Expect(senderAcct.Gas).ToNot(BeEmpty())
				Expect(senderAcct.Gas.String()).To(Equal(curGasBal.Decimal().Add(params.GasReward.Decimal()).String()))
				Expect(senderAcct.Balance.String()).To(Equal("99"))
				Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
			})

			It("should index work nonce", func() {
				err = ct.Exec()
				Expect(err).To(BeNil())
				Expect(logic.SysKeeper().IsWorkNonceRegistered(tx.Epoch, tx.WorkNonce)).To(BeNil())
			})
		})
	})
})
