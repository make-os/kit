package repo

import (
	"os"
	"path/filepath"

	"gitlab.com/makeos/mosdef/types/core"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Pruner", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo core.BareRepo
	var ctrl *gomock.Controller
	var repoName string
	var pruner *Pruner
	var mgr *mocks.MockPoolGetter

	BeforeEach(func() {

		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mgr = mocks.NewMockPoolGetter(ctrl)
		pruner = NewPruner(mgr, cfg.GetRepoRoot())
		go pruner.Start()
	})

	AfterEach(func() {
		pruner.Stop()
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
				pruner.Schedule(repoName)
				Expect(pruner.targets).To(HaveLen(1))
				Expect(pruner.targets).To(HaveKey(repoName))
			})
		})
	})

	Describe(".doPrune", func() {
		BeforeEach(func() {
			defer GinkgoRecover()
		})

		When("repo has tx in the push pool", func() {
			BeforeEach(func() {
				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().RepoHasPushNote(repoName).Return(true)
				mgr.EXPECT().GetPushPool().Return(mockPushPool)
				err = pruner.doPrune(repoName, false)
			})

			It("should return err='refused because repo still has transactions in the push pool'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("refused because repo still has transactions in the push pool"))
			})
		})

		When("repo does not have tx in the push pool", func() {
			var hash string
			BeforeEach(func() {
				hash = createBlob(path, "hello world")
				Expect(repo.ObjectExist(hash)).To(BeTrue())

				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().RepoHasPushNote(repoName).Return(false)
				mgr.EXPECT().GetPushPool().Return(mockPushPool)
				err = pruner.doPrune(repoName, false)
			})

			It("should successfully prune repo", func() {
				Expect(err).To(BeNil())
				Expect(repo.ObjectExist(hash)).To(BeFalse())
			})
		})

		When("repo has tx in the push pool and force is true", func() {
			var hash string

			BeforeEach(func() {
				hash = createBlob(path, "hello world")
				Expect(repo.ObjectExist(hash)).To(BeTrue())

				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().RepoHasPushNote(repoName).Return(true)
				mgr.EXPECT().GetPushPool().Return(mockPushPool)
				err = pruner.doPrune(repoName, true)
			})

			It("should successfully prune repo", func() {
				Expect(err).To(BeNil())
				Expect(repo.ObjectExist(hash)).To(BeFalse())
			})
		})
	})
})
