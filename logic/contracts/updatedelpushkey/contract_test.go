package updatedelpushkey_test

import (
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	logic2 "github.com/make-os/lobe/logic"
	"github.com/make-os/lobe/logic/contracts/updatedelpushkey"
	"github.com/make-os/lobe/storage"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PushKeyUpdateDeleteContract", func() {
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
			ct := updatedelpushkey.NewContract()
			Expect(ct.CanExec(txns.TxTypeUpDelPushKey)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var err error

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10"})
		})

		When("delete is set to true", func() {
			var pushKeyID = "pk1_abc"
			BeforeEach(func() {
				key := state.BarePushKey()
				key.Address = "addr1"
				logic.PushKeyKeeper().Update(pushKeyID, key)
				Expect(logic.PushKeyKeeper().Get(pushKeyID).IsNil()).To(BeFalse())

				err = updatedelpushkey.NewContract().Init(logic, &txns.TxUpDelPushKey{
					TxCommon: &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					ID:       pushKeyID,
					Delete:   true,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should delete key", func() {
				Expect(logic.PushKeyKeeper().Get(pushKeyID).IsNil()).To(BeTrue())
			})

			Specify("that fee is deducted from sender account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("8.5")))
			})
		})

		When("removeScope includes indices 0,2", func() {
			var pushKeyID = "pk1_abc"
			BeforeEach(func() {
				key := state.BarePushKey()
				key.Address = "addr1"
				key.Scopes = []string{"scope1", "scope2", "scope3"}
				logic.PushKeyKeeper().Update(pushKeyID, key)
				Expect(logic.PushKeyKeeper().Get(pushKeyID).IsNil()).To(BeFalse())

				rmScopes := []int{0, 2}
				err = updatedelpushkey.NewContract().Init(logic, &txns.TxUpDelPushKey{
					TxCommon:     &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					ID:           pushKeyID,
					Delete:       false,
					RemoveScopes: rmScopes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should remove scopes at indices 0,2", func() {
				key := logic.PushKeyKeeper().Get(pushKeyID)
				Expect(key.Scopes).To(HaveLen(1))
				Expect(key.Scopes).To(ContainElement("scope2"))
			})
		})

		When("removeScope includes indices 0,5,2 or 0,2,5", func() {
			var pushKeyID = "pk1_abc"
			for _, indicesSlice := range [][]int{{0, 5, 2}, {0, 2, 5}} {
				BeforeEach(func() {
					key := state.BarePushKey()
					key.Address = "addr1"
					key.Scopes = []string{"scope1", "scope2", "scope3", "scope4", "scope5", "scope6", "scope7"}
					logic.PushKeyKeeper().Update(pushKeyID, key)
					Expect(logic.PushKeyKeeper().Get(pushKeyID).IsNil()).To(BeFalse())

					rmScopes := indicesSlice
					err = updatedelpushkey.NewContract().Init(logic, &txns.TxUpDelPushKey{
						TxCommon:     &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
						ID:           pushKeyID,
						Delete:       false,
						RemoveScopes: rmScopes,
					}, 0).Exec()
					Expect(err).To(BeNil())
				})

				It("should remove scopes at indices 0,2", func() {
					key := logic.PushKeyKeeper().Get(pushKeyID)
					Expect(key.Scopes).To(HaveLen(4))
					Expect(key.Scopes).ToNot(ContainElement("scope1"))
					Expect(key.Scopes).ToNot(ContainElement("scope3"))
					Expect(key.Scopes).ToNot(ContainElement("scope6"))
				})
			}
		})

		When("addScopes includes scope10, scope11", func() {
			var pushKeyID = "pk1_abc"
			BeforeEach(func() {
				key := state.BarePushKey()
				key.Address = "addr1"
				key.Scopes = []string{"scope1", "scope2", "scope3"}
				logic.PushKeyKeeper().Update(pushKeyID, key)
				Expect(logic.PushKeyKeeper().Get(pushKeyID).IsNil()).To(BeFalse())

				addScopes := []string{"scope10", "scope11"}
				err = updatedelpushkey.NewContract().Init(logic, &txns.TxUpDelPushKey{
					TxCommon:  &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					ID:        pushKeyID,
					Delete:    false,
					AddScopes: addScopes,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should add scopes scope10, scope11", func() {
				key := logic.PushKeyKeeper().Get(pushKeyID)
				Expect(key.Scopes).To(HaveLen(5))
				Expect(key.Scopes).To(ContainElement("scope10"))
				Expect(key.Scopes).To(ContainElement("scope11"))
			})
		})

		When("feeCap is set", func() {
			var pushKeyID = "pk1_abc"
			BeforeEach(func() {
				key := state.BarePushKey()
				key.Address = "addr1"
				logic.PushKeyKeeper().Update(pushKeyID, key)
				Expect(logic.PushKeyKeeper().Get(pushKeyID).IsNil()).To(BeFalse())

				err = updatedelpushkey.NewContract().Init(logic, &txns.TxUpDelPushKey{
					TxCommon: &txns.TxCommon{Fee: "1.5", SenderPubKey: sender.PubKey().ToPublicKey()},
					ID:       pushKeyID,
					Delete:   false,
					FeeCap:   "100",
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			It("should update fee cap", func() {
				key := logic.PushKeyKeeper().Get(pushKeyID)
				Expect(key.FeeCap).To(Equal(util.String("100")))
			})
		})
	})
})
