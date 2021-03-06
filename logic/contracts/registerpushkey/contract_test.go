package registerpushkey_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/registerpushkey"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

func TestRegisterPushkey(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RegisterPushkey Suite")
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
			ct := registerpushkey.NewContract()
			Expect(ct.CanExec(txns.TxTypeRegisterPushKey)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error
		var pushKey *ed25519.PubKey
		var scopes = []string{"repo1", "repo2"}
		var feeCap = util.String("10")

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10"})
		})

		When("successful", func() {
			BeforeEach(func() {
				pushKey = ed25519.NewKeyFromIntSeed(1).PubKey()
				err = registerpushkey.NewContract().Init(logic, &txns.TxRegisterPushKey{
					TxCommon:  &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					Scopes:    scopes,
					FeeCap:    feeCap,
					PublicKey: pushKey.ToPublicKey(),
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that the push key was added to the tree", func() {
				pushKeyID := ed25519.CreatePushKeyID(pushKey.ToPublicKey())
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
				pushKey = ed25519.NewKeyFromIntSeed(1).PubKey()
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
