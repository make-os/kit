package repo

// import (
// 	"fmt"
// 	"path/filepath"

// 	"github.com/golang/mock/gomock"
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// 	"github.com/phayes/freeport"

// 	"github.com/makeos/mosdef/config"
// 	"github.com/makeos/mosdef/testutil"
// 	"github.com/makeos/mosdef/util"
// )

// var _ = FDescribe("PushHook", func() {
// 	var err error
// 	var cfg *config.EngineConfig
// 	var path string
// 	var repo *Repo
// 	var repoMgr *Manager
// 	var hook *PushHook

// 	BeforeEach(func() {
// 		cfg, err = testutil.SetTestCfg()
// 		Expect(err).To(BeNil())
// 		cfg.Node.GitBinPath = "/usr/bin/git"
// 	})

// 	BeforeEach(func() {
// 		repoName := util.RandString(5)
// 		path = filepath.Join(cfg.GetRepoRoot(), repoName)
// 		execGit(cfg.GetRepoRoot(), "init", repoName)
// 		repo, err = getRepo(path)
// 		Expect(err).To(BeNil())

// 		port, _ := freeport.GetFreePort()
// 		ctrl := gomock.NewController(GinkgoT())
// 		mockLogic := testutil.MockLogic(ctrl)
// 		repoMgr = NewManager(cfg, fmt.Sprintf(":%d", port), mockLogic.Logic)

// 		hook = newPushHook(repo, repoMgr)
// 	})

// 	Describe(".BeforePush", func() {
// 		BeforeEach(func() {
// 			appendCommit(path, "file.txt", "some text", "commit msg")
// 			Expect(hook.oldState).To(BeNil())
// 			err := hook.BeforePush()
// 			Expect(err).To(BeNil())
// 		})

// 		It("should set oldState", func() {
// 			Expect(hook.oldState.IsEmpty()).To(BeFalse())
// 		})
// 	})

// })
