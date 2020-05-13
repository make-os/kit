package gitpush_test

import (
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	logic2 "gitlab.com/makeos/mosdef/logic"
	"gitlab.com/makeos/mosdef/logic/contracts/gitpush"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("AcquireNamespaceContract", func() {
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
			ct := gitpush.NewContract()
			Expect(ct.CanExec(core.TxTypePush)).To(BeTrue())
			Expect(ct.CanExec(core.TxTypeCoinTransfer)).To(BeFalse())
		})
	})

	Describe(".Exec", func() {
		var repo = "repo1"
		var pushKeyID = crypto.CreatePushKeyID(crypto.StrToPublicKey("pushKeyID"))
		var pushKeyID2 = crypto.CreatePushKeyID(crypto.StrToPublicKey("pushKeyID2"))
		var refs []*core.PushedReference

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{Balance: "10", Nonce: 1})
			logic.PushKeyKeeper().Update(pushKeyID, &state.PushKey{PubKey: crypto.StrToPublicKey("pub_key"), Address: sender.Addr()})
			logic.RepoKeeper().Update(repo, &state.Repository{
				References: map[string]*state.Reference{
					"refs/heads/master": {Nonce: 1, Creator: util.MustDecodePushKeyID(pushKeyID)},
				},
			})
		})

		When("pushed reference did not previously exist (new reference)", func() {
			It("should add pusher as creator of the reference", func() {
				refs = []*core.PushedReference{
					{Name: "refs/heads/dev", Data: &core.ReferenceData{}},
				}

				rawPkID := util.MustDecodePushKeyID(pushKeyID2)
				err = gitpush.NewContract().Init(logic, &core.TxPush{
					TxCommon: &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					PushNote: &core.PushNote{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()

				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Get("refs/heads/dev").Creator).To(Equal(crypto.PushKey(rawPkID)))
			})
		})

		When("pushed reference new hash is zero (meaning delete is required)", func() {
			It("should remove the reference from the repo", func() {
				refs = []*core.PushedReference{
					{Name: "refs/heads/master", NewHash: strings.Repeat("0", 40)},
				}

				rawPkID := util.MustDecodePushKeyID(pushKeyID2)
				err = gitpush.NewContract().Init(logic, &core.TxPush{
					TxCommon: &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					PushNote: &core.PushNote{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()

				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Has("refs/heads/master")).To(BeFalse())
			})
		})

		When("pushed reference already exist", func() {
			It("should not update reference creator", func() {
				refs = []*core.PushedReference{
					{Name: "refs/heads/master", Data: &core.ReferenceData{}},
				}

				rawPkID := util.MustDecodePushKeyID(pushKeyID2)
				err = gitpush.NewContract().Init(logic, &core.TxPush{
					TxCommon: &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					PushNote: &core.PushNote{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()

				rep := logic.RepoKeeper().Get(repo)
				Expect(rep.References.Get("refs/heads/master").Creator).ToNot(Equal(rawPkID))

				actual := util.MustDecodePushKeyID(pushKeyID)
				Expect(rep.References.Get("refs/heads/master").Creator).To(Equal(crypto.PushKey(actual)))
			})
		})

		When("reference has nonce = 1. After a successful exec:", func() {
			BeforeEach(func() {
				refs = []*core.PushedReference{
					{Name: "refs/heads/master", Data: &core.ReferenceData{}},
				}

				rawPkID := util.MustDecodePushKeyID(pushKeyID)
				err = gitpush.NewContract().Init(logic, &core.TxPush{
					TxCommon: &core.TxCommon{Fee: "1", SenderPubKey: sender.PubKey().ToPublicKey()},
					PushNote: &core.PushNote{RepoName: repo, References: refs, PushKeyID: rawPkID},
				}, 0).Exec()
				Expect(err).To(BeNil())
			})

			Specify("that the reference's nonce was incremented", func() {
				repo := logic.RepoKeeper().Get(repo)
				Expect(repo.References.Get("refs/heads/master").Nonce).To(Equal(uint64(2)))
			})

			Specify("that fee was deducted from pusher account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})

			Specify("that sender account nonce was incremented", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.Nonce).To(Equal(uint64(2)))
			})
		})
	})
})
