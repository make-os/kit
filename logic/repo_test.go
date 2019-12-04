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

var _ = Describe("Repo", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.EngineConfig
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

	Describe(".execRepoCreate", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var spk util.String

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &types.Account{
				Balance:             util.String("10"),
				Stakes:              types.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("successful", func() {
			BeforeEach(func() {
				spk = util.String(sender.PubKey().Base58())
				err = txLogic.execRepoCreate(spk, "repo", "1.5", 0)
				Expect(err).To(BeNil())
			})

			Specify("that the repo was added to the tree", func() {
				repo := txLogic.logic.RepoKeeper().GetRepo("repo")
				Expect(repo.IsEmpty()).To(BeFalse())
				Expect(repo.CreatorAddress).To(Equal(sender.Addr()))
			})

			Specify("that fee is deducted from sender account", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("8.5")))
			})

			Specify("that sender account nonce increased", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.Nonce).To(Equal(uint64(1)))
			})
		})
	})
})
