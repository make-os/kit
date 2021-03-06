package updatenamespacedomains_test

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	logic2 "github.com/make-os/kit/logic"
	"github.com/make-os/kit/logic/contracts/updatenamespacedomains"
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

func TestUpdateNamespaceDomains(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UpdateNamespaceDomains Suite")
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
			ct := updatenamespacedomains.NewContract()
			Expect(ct.CanExec(txns.TxTypeNamespaceDomainUpdate)).To(BeTrue())
			Expect(ct.CanExec(txns.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var nsName = "name1"

		When("target domain (domain1) has a value and the domain already exist (update)", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})
				logic.NamespaceKeeper().Update(nsName, &state.Namespace{
					Domains: map[string]string{"domain1": "target"},
				})

				update := map[string]string{"domain1": "target_update"}
				err = updatenamespacedomains.NewContract().Init(logic, &txns.TxNamespaceDomainUpdate{
					TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					Name:     nsName,
					Domains:  update,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that domain 'domain1' has changed", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Domains["domain1"]).To(Equal("target_update"))
			})

			Specify("that sender account is deduct of fee", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})

			Specify("that sender account nonce is incremented", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Nonce.UInt64()).To(Equal(uint64(2)))
			})
		})

		When("target domain (domain1) has a value but the domain does not already exist (new)", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})

				update := map[string]string{"domain1": "target_update"}
				err = updatenamespacedomains.NewContract().Init(logic, &txns.TxNamespaceDomainUpdate{
					TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					Name:     nsName,
					Domains:  update,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that domain 'domain1' was added", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Domains["domain1"]).To(Equal("target_update"))
			})

			Specify("that sender account is deduct of fee", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})
		})

		When("target domain (domain1) has no value and the domain already exist (remove)", func() {
			BeforeEach(func() {
				params.NamespaceTTL = 10
				params.NamespaceGraceDur = 10

				logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})

				logic.NamespaceKeeper().Update(nsName, &state.Namespace{
					Domains: map[string]string{"domain1": "target", "domain2": "other_target"},
				})

				update := map[string]string{"domain1": ""}
				err = updatenamespacedomains.NewContract().Init(logic, &txns.TxNamespaceDomainUpdate{
					TxCommon: &txns.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					Name:     nsName,
					Domains:  update,
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that domain 'domain1' has been removed", func() {
				ns := logic.NamespaceKeeper().Get(nsName)
				Expect(ns.IsNil()).To(BeFalse())
				Expect(ns.Domains).ToNot(HaveKey("domain1"))
			})

			Specify("that sender account is deduct of fee", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})
		})
	})
})
