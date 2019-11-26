package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/phayes/freeport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Manager", func() {
	var err error
	var cfg *config.EngineConfig
	var repoMgr *Manager
	var path string
	var repo *Repo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		port, _ := freeport.GetFreePort()
		repoMgr = NewManager(cfg, fmt.Sprintf(":%d", port))
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepo(path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetRepoState", func() {
		When("no objects exist", func() {
			It("should return empty state", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.IsEmpty()).To(BeTrue())
			})
		})

		When("a commit exist", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 1 ref", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.References.Len()).To(Equal(int64(1)))
			})
		})

		When("two branches with 1 commit each exist", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				appendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 2 refs", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.References.Len()).To(Equal(int64(2)))
			})
		})

		When("match=refs/heads", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				createCommitAndAnnotatedTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag1")
			})

			Specify("that the repo has ref refs/heads/master", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/heads/master"))
				Expect(err).To(BeNil())
				Expect(st.References.Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref refs/heads/dev", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/heads/dev"))
				Expect(err).To(BeNil())
				Expect(st.References.Len()).To(Equal(int64(1)))
			})
		})

		When("branch master and dev exist and match=refs/heads/dev", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				appendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 1 ref", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/heads/dev"))
				Expect(err).To(BeNil())
				Expect(st.References.Len()).To(Equal(int64(1)))
			})
		})

		When("match=refs/tags", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				createCommitAndAnnotatedTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag")
				createCommitAndLightWeightTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag2")
			})

			Specify("that the repo has ref=refs/tags/tag", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/tags/tag"))
				Expect(err).To(BeNil())
				Expect(st.References.Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref=refs/tags/tag2", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/tags/tag2"))
				Expect(err).To(BeNil())
				Expect(st.References.Len()).To(Equal(int64(1)))
			})
		})
	})
})
