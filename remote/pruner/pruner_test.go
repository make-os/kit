package pruner_test

import (
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/pruner"
	types2 "gitlab.com/makeos/mosdef/remote/pushpool/types"
	repo3 "gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("RepoPruner", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo types2.LocalRepo
	var ctrl *gomock.Controller
	var repoName string
	var prn *pruner.Pruner
	var svr *mocks.MockPoolGetter

	BeforeEach(func() {

		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo3.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		svr = mocks.NewMockPoolGetter(ctrl)
		prn = pruner.NewPruner(svr, cfg.GetRepoRoot())
		go prn.Start()
	})

	AfterEach(func() {
		prn.Stop()
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Schedule", func() {
		When("repo is scheduled", func() {
			BeforeEach(func() {
				defer GinkgoRecover()
			})

			It("should be added as a target", func() {
				prn.Schedule(repoName)
				Expect(prn.GetTargets()).To(HaveLen(1))
				Expect(prn.GetTargets()).To(HaveKey(repoName))
			})
		})
	})

	Describe(".Prune", func() {
		BeforeEach(func() {
			defer GinkgoRecover()
		})

		When("repo has tx in the push pool", func() {
			BeforeEach(func() {
				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().RepoHasPushNote(repoName).Return(true)
				svr.EXPECT().GetPushPool().Return(mockPushPool)
				err = prn.Prune(repoName, false)
			})

			It("should return err='refused because repo still has transactions in the push pool'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("refused because repo still has transactions in the push pool"))
			})
		})

		When("repo does not have tx in the push pool", func() {
			var hash string
			BeforeEach(func() {
				hash = testutil2.CreateBlob(path, "hello world")
				Expect(repo.ObjectExist(hash)).To(BeTrue())

				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().RepoHasPushNote(repoName).Return(false)
				svr.EXPECT().GetPushPool().Return(mockPushPool)
				err = prn.Prune(repoName, false)
			})

			It("should successfully prune repo", func() {
				Expect(err).To(BeNil())
				Expect(repo.ObjectExist(hash)).To(BeFalse())
			})
		})

		When("repo has tx in the push pool and force is true", func() {
			var hash string

			BeforeEach(func() {
				hash = testutil2.CreateBlob(path, "hello world")
				Expect(repo.ObjectExist(hash)).To(BeTrue())

				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().RepoHasPushNote(repoName).Return(true)
				svr.EXPECT().GetPushPool().Return(mockPushPool)
				err = prn.Prune(repoName, true)
			})

			It("should successfully prune repo", func() {
				Expect(err).To(BeNil())
				Expect(repo.ObjectExist(hash)).To(BeFalse())
			})
		})
	})
})
