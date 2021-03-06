package server

import (
	"os"
	"path/filepath"

	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/net/dht/announcer"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/testutil"
)

var _ = Describe("Create", func() {
	var err error
	var cfg *config.AppConfig
	var repoMgr *Server
	var ctrl *gomock.Controller
	var mockObjects *testutil.MockObjects
	var mockDHT *mocks.MockDHT
	var mockMempool *mocks.MockMempool
	var mockBlockGetter *mocks.MockBlockGetter

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockObjects = testutil.Mocks(ctrl)

		mockDHT = mocks.NewMockDHT(ctrl)
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeRepoName, gomock.Any())
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeGit, gomock.Any())

		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		repoMgr = New(cfg, ":45000", mockObjects.Logic, mockDHT, mockMempool, mockObjects.Service, mockBlockGetter)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".InitRepository", func() {
		When("a repository with the matching name does not exist", func() {
			It("should return nil and create the repository", func() {
				err := repoMgr.InitRepository("my_repo")
				Expect(err).To(BeNil())
				path := filepath.Join(repoMgr.rootDir, "my_repo")
				_, err = os.Stat(path)
				Expect(err).To(BeNil())
			})
		})

		When("a repository with the matching name already exist", func() {
			BeforeEach(func() {
				err := repoMgr.InitRepository("my_repo")
				Expect(err).To(BeNil())
			})

			It("should return nil and create the repository", func() {
				err := repoMgr.InitRepository("my_repo")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to create repo: repository already exists"))
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
				err := repoMgr.InitRepository("my_repo")
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
