package setdelcommission_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/crypto"
	logic2 "gitlab.com/makeos/lobe/logic"
	"gitlab.com/makeos/lobe/logic/contracts/setdelcommission"
	"gitlab.com/makeos/lobe/storage"
	"gitlab.com/makeos/lobe/testutil"
	"gitlab.com/makeos/lobe/types/core"
	"gitlab.com/makeos/lobe/types/state"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
)

var _ = Describe("SetDelegateCommissionContract", func() {
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
			ct := setdelcommission.NewContract()
			Expect(ct.CanExec(txns.TxTypeSetDelegatorCommission)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var senderAcct *state.Account
		Context("when tx has incorrect nonce", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Stakes: state.BareAccountStakes(), DelegatorCommission: 15.4})
				tx := &txns.TxSetDelegateCommission{
					Commission: "23.5",
					TxCommon:   &txns.TxCommon{Fee: "2", SenderPubKey: sender.PubKey().ToPublicKey()}}
				ct := setdelcommission.NewContract().Init(logic, tx, 0)
				err = ct.Exec()
				Expect(err).To(BeNil())
				senderAcct = logic.AccountKeeper().Get(sender.Addr())
			})

			It("should successfully set new commission", func() {
				Expect(senderAcct.DelegatorCommission).To(Equal(23.5))
			})

			It("should increment nonce", func() {
				Expect(senderAcct.Nonce.UInt64()).To(Equal(uint64(1)))
			})

			It("should have balance of 8", func() {
				Expect(senderAcct.Balance).To(Equal(util.String("8")))
			})
		})
	})

})
