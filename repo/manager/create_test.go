package manager

import (
	"os"
	"path/filepath"

	"gitlab.com/makeos/mosdef/mocks"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("Create", func() {
	var err error
	var cfg *config.AppConfig
	var repoMgr *Manager
	var ctrl *gomock.Controller
	var mockLogic *testutil.MockObjects
	var mockDHT *mocks.MockDHTNode
	var mockMempool *mocks.MockMempool
	var mockBlockGetter *mocks.MockBlockGetter

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
		mockDHT = mocks.NewMockDHTNode(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		repoMgr = NewManager(cfg, ":45000", mockLogic.Logic, mockDHT, mockMempool, mockBlockGetter)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CreateRepository", func() {
		When("a repository with the matching name does not exist", func() {
			It("should return nil and create the repository", func() {
				err := repoMgr.CreateRepository("my_repo")
				Expect(err).To(BeNil())
				path := filepath.Join(repoMgr.rootDir, "my_repo")
				_, err = os.Stat(path)
				Expect(err).To(BeNil())
			})
		})

		When("a repository with the matching name already exist", func() {
			BeforeEach(func() {
				err := repoMgr.CreateRepository("my_repo")
				Expect(err).To(BeNil())
			})

			It("should return nil and create the repository", func() {
				err := repoMgr.CreateRepository("my_repo")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("a repository with name (my_repo) already exist"))
			})
		})
	})

	Describe(".HasRepository", func() {
		When("repo does not exist", func() {
			It("should return false", func() {
				res := repoMgr.HasRepository("repo1")
				Expect(res).To(BeFalse())
			})
		})

		When("repo does exist", func() {
			It("should return true", func() {
				err := repoMgr.CreateRepository("my_repo")
				Expect(err).To(BeNil())
				res := repoMgr.HasRepository("my_repo")
				Expect(res).To(BeTrue())
			})
		})

		When("repo directory exist but not a valid git project", func() {
			It("should return false", func() {
				path := filepath.Join(repoMgr.rootDir, "my_repo")
				err := os.MkdirAll(path, 0700)
				Expect(err).To(BeNil())
				res := repoMgr.HasRepository("my_repo")
				Expect(res).To(BeFalse())
			})
		})
	})
})
