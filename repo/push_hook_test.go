package repo

import (
	"fmt"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("PushHook", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo types.BareRepo
	var repoMgr *Manager
	var hook *PushHook
	var mockDHT *mocks.MockDHT

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepo(path)
		Expect(err).To(BeNil())

		port, _ := freeport.GetFreePort()
		ctrl := gomock.NewController(GinkgoT())
		mockLogic := testutil.MockLogic(ctrl)
		mockDHT = mocks.NewMockDHT(ctrl)
		repoMgr = NewManager(cfg, fmt.Sprintf(":%d", port), mockLogic.Logic, mockDHT)

		hook = newPushHook(repo, repoMgr, cfg.G().Log)
	})

	Describe(".BeforePush", func() {
		BeforeEach(func() {
			appendCommit(path, "file.txt", "some text", "commit msg")
			Expect(hook.oldState).To(BeNil())
			err := hook.BeforePush()
			Expect(err).To(BeNil())
		})

		It("should set oldState", func() {
			Expect(hook.oldState.IsEmpty()).To(BeFalse())
		})
	})

	Describe(".AfterPush", func() {
		var pr *PushReader

		When("old repo state was not captured", func() {
			BeforeEach(func() {
				pr, err = newPushReader(nil, repo)
				Expect(err).To(BeNil())
				err = hook.AfterPush(pr)
			})

			It("should return err='hook: expected old state to have been captured'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("hook: expected old state to have been captured"))
			})
		})
	})

})
