package repo

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Gitops", func() {
	var err error
	var cfg *config.EngineConfig
	var path string
	// var repoMgr *Manager

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		// port, _ := freeport.GetFreePort()
		// repoMgr = NewManager(cfg, fmt.Sprintf(":%d", port))
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".getObjectsID", func() {
		var objsID ObjectsID

		BeforeEach(func() {
			repo, err := getRepo(path)
			Expect(err).To(BeNil())

			objsID = getObjectsID(repo)
			Expect(objsID).To(BeEmpty())

			appendCommit(path, "file.txt", "some text", "commit msg")
			objsID = getObjectsID(repo)
		})

		It("should return 3 objects", func() {
			Expect(objsID).To(HaveLen(3)) // commit, tree, blob
		})
	})

	Describe("ObjectsID.Size", func() {
		var err error
		var repo *Repo
		var objsID ObjectsID

		BeforeEach(func() {
			repo, err = getRepo(path)
			Expect(err).To(BeNil())
			appendCommit(path, "file.txt", "some text", "commit msg")
			appendCommit(path, "file.txt", "some text 2", "commit msg 2")
			objsID = getObjectsID(repo)
		})

		It("should return size", func() {
			Expect(objsID).To(HaveLen(6))
			size, err := objsID.GetSize(repo)
			Expect(err).To(BeNil())
			Expect(size).To(Equal(int64(493)))
		})
	})
})
