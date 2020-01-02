package logic

import (
	"os"

	"github.com/golang/mock/gomock"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delegation", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var txLogic *Transaction
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
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
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
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
				spk := sender.PubKey().MustBytes32()
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
})
