package logic

import (
	"os"

	"github.com/golang/mock/gomock"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Push", func() {
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

	Describe(".execPush", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var repo = "repo1"
		var creator = crypto.NewKeyFromIntSeed(2)
		var pkID = "pkID"

		When("reference has nonce = 1", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("10"),
					Stakes:  types.BareAccountStakes(),
					Nonce:   1,
				})

				logic.GPGPubKeyKeeper().Update(pkID, &types.GPGPubKey{
					PubKey:  "pub_key",
					Address: sender.Addr(),
				})

				logic.RepoKeeper().Update(repo, &types.Repository{
					CreatorAddress: creator.Addr(),
					References: map[string]interface{}{
						"refs/heads/master": &types.Reference{Nonce: 1},
					},
				})

				refs := []*types.PushedReference{
					&types.PushedReference{Name: "refs/heads/master"},
				}
				err = txLogic.execPush(repo, refs, "1", pkID, 0)
				Expect(err).To(BeNil())
			})

			Specify("that the reference's nonce was incremented", func() {
				repo := txLogic.logic.RepoKeeper().GetRepo(repo)
				Expect(repo.References.Get("refs/heads/master").Nonce).To(Equal(uint64(2)))
			})

			Specify("that fee was deducted from pusher account", func() {
				acct := txLogic.logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.Balance).To(Equal(util.String("9")))
			})

			Specify("that sender account nonce was incremented", func() {
				acct := txLogic.logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.Nonce).To(Equal(uint64(2)))
			})
		})
	})
})
