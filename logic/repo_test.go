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

	Describe(".execRepoCreate", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var spk util.Bytes32

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &types.Account{
				Balance:             util.String("10"),
				Stakes:              types.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("successful", func() {
			BeforeEach(func() {
				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoCreate(spk, "repo", "1.5", 0)
				Expect(err).To(BeNil())
			})

			Specify("that the repo was added to the tree", func() {
				repo := txLogic.logic.RepoKeeper().GetRepo("repo")
				Expect(repo.IsNil()).To(BeFalse())
				Expect(repo.Owners).To(HaveKey(sender.Addr().String()))
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

	Describe(".execRepoUpsertOwner", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)
		var key2 = crypto.NewKeyFromIntSeed(2)
		var spk util.Bytes32

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &types.Account{
				Balance:             util.String("10"),
				Stakes:              types.BareAccountStakes(),
				DelegatorCommission: 10,
			})

		})

		When("sender is the only owner", func() {
			txID := util.ToHex(util.RandBytes(32))
			repoName := "repo"
			address := "owner_address"

			BeforeEach(func() {
				repoUpd := types.BareRepository()
				repoUpd.Config = types.DefaultRepoConfig()
				repoUpd.AddOwner(sender.Addr().String(), &types.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoUpsertOwner(spk, txID, repoName, address, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is finalized and self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeTrue())
				Expect(repo.Proposals.Get("1").IsSelfAccepted()).To(BeTrue())
			})

			Specify("that fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("8.5"))
			})
		})

		When("sender is not the only owner", func() {
			txID := util.ToHex(util.RandBytes(32))
			repoName := "repo"
			address := "owner_address"

			BeforeEach(func() {
				repoUpd := types.BareRepository()
				repoUpd.Config = types.DefaultRepoConfig()
				repoUpd.AddOwner(sender.Addr().String(), &types.RepoOwner{})
				repoUpd.AddOwner(key2.Addr().String(), &types.RepoOwner{})
				logic.RepoKeeper().Update(repoName, repoUpd)

				spk = sender.PubKey().MustBytes32()
				err = txLogic.execRepoUpsertOwner(spk, txID, repoName, address, "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add the new proposal to the repo", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
			})

			Specify("that the proposal is not finalized or self accepted", func() {
				repo := logic.RepoKeeper().GetRepo(repoName)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").IsFinalized()).To(BeFalse())
				Expect(repo.Proposals.Get("1").IsSelfAccepted()).To(BeFalse())
			})

			Specify("that fee was deducted", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr(), 0)
				Expect(acct.Balance.String()).To(Equal("8.5"))
			})
		})
	})
})
