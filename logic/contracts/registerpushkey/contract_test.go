package registerpushkey_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	logic2 "github.com/themakeos/lobe/logic"
	"github.com/themakeos/lobe/logic/contracts/registerpushkey"
	"github.com/themakeos/lobe/storage"
	"github.com/themakeos/lobe/testutil"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
)

var _ = Describe("RegisterPushKeyContract", func() {
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
			ct := registerpushkey.NewContract()
			Expect(ct.CanExec(txns.TxTypeRegisterPushKey)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var pushKey *crypto.PubKey
		var scopes = []string{"repo1", "repo2"}
		var feeCap = util.String("10")

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10"})
		})

		When("successful", func() {
			BeforeEach(func() {
				pushKey = crypto.NewKeyFromIntSeed(1).PubKey()
				err = registerpushkey.NewContract().Init(logic, &txns.TxRegisterPushKey{
					TxCommon:  &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					Scopes:    scopes,
					FeeCap:    feeCap,
					PublicKey: pushKey.ToPublicKey(),
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that the push key was added to the tree", func() {
				pushKeyID := crypto.CreatePushKeyID(pushKey.ToPublicKey())
				pk := logic.PushKeyKeeper().Get(pushKeyID, 0)
				Expect(pk.IsNil()).To(BeFalse())
				Expect(pk.Address).To(Equal(sender.Addr()))
				Expect(pk.PubKey).To(Equal(pushKey.ToPublicKey()))
				Expect(pk.Scopes).To(Equal(scopes))
				Expect(pk.FeeCap).To(Equal(feeCap))
			})

			Specify("that fee is deducted from sender account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("8.5")))
			})
		})

		When("sender account update is disabled", func() {
			BeforeEach(func() {
				pushKey = crypto.NewKeyFromIntSeed(1).PubKey()
				err = registerpushkey.NewContractWithNoSenderUpdate().Init(logic, &txns.TxRegisterPushKey{
					TxCommon:  &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					Scopes:    scopes,
					FeeCap:    feeCap,
					PublicKey: pushKey.ToPublicKey(),
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that fee is not deducted from sender account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("10")))
			})

			Specify("that sender account nonce is changed", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Nonce.UInt64()).To(Equal(uint64(0)))
			})
		})
	})
})
