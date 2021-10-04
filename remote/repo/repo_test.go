package repo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	config2 "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	rr "github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	state2 "github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RepoContext Suite")
}

var _ = Describe("Repo", func() {
	var err error
	var cfg *config.AppConfig
	var path, repoName string
	var r types.LocalRepo
	var key *ed25519.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		key = ed25519.NewKeyFromIntSeed(1)
	})

	BeforeEach(func() {
		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		r, err = rr.GetWithGitModule(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ObjectExist", func() {
		It("should return true when object exist", func() {
			hash := testutil2.CreateBlob(path, "hello world")
			Expect(r.ObjectExist(hash)).To(BeTrue())
		})

		It("should return false when object does not exist", func() {
			hash := strings.Repeat("0", 40)
			Expect(r.ObjectExist(hash)).To(BeFalse())
		})
	})

	Describe(".Prune", func() {
		It("should remove unreachable objects", func() {
			hash := testutil2.CreateBlob(path, "hello world")
			Expect(r.ObjectExist(hash)).To(BeTrue())
			err := r.Prune(time.Time{})
			Expect(err).To(BeNil())
			Expect(r.ObjectExist(hash)).To(BeFalse())
		})
	})

	Describe(".GetObject", func() {
		It("should return object if it exists", func() {
			hash := testutil2.CreateBlob(path, "hello world")
			obj, err := r.GetObject(hash)
			Expect(err).To(BeNil())
			Expect(obj).NotTo(BeNil())
			Expect(obj.ID().String()).To(Equal(hash))
		})

		It("should return error if object does not exists", func() {
			obj, err := r.GetObject("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(plumbing.ErrObjectNotFound))
			Expect(obj).To(BeNil())
		})
	})

	Describe(".GetReferences", func() {
		It("should return all references", func() {
			testutil2.AppendCommit(path, "body", "content", "m1")
			testutil2.CreateCheckoutBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "content", "m2")
			refs, err := r.GetReferences()
			Expect(err).To(BeNil())
			Expect(refs).To(HaveLen(3))
			Expect(refs[0].String()).To(Equal("refs/heads/issues/1"))
			Expect(refs[1].String()).To(Equal("refs/heads/master"))
			Expect(refs[2].String()).To(Equal("HEAD"))
		})
	})

	Describe(".HeadObject", func() {
		It("should return ErrReferenceNotFound when HEAD is unknown", func() {
			_, err := r.HeadObject()
			Expect(err).To(Equal(plumbing.ErrReferenceNotFound))
		})

		It("should return ErrReferenceNotFound when HEAD is unknown", func() {
			testutil2.AppendCommit(path, "body", "content", "m1")
			tipHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			o, err := r.HeadObject()
			Expect(err).To(BeNil())
			Expect(o.(*object.Commit).Hash.String()).To(Equal(tipHash))
		})
	})

	Describe(".NumIssueBranches", func() {
		It("should return 1 when only one issue branch exists", func() {
			testutil2.CreateCheckoutBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "content", "added body")
			n, err := r.NumIssueBranches()
			Expect(err).To(BeNil())
			Expect(n).To(Equal(1))
		})

		It("should return 2 when only one issue branch exists", func() {
			testutil2.CreateCheckoutBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "content", "added body")
			testutil2.CreateCheckoutBranch(path, "issues/2")
			testutil2.AppendCommit(path, "body", "content", "added body")
			n, err := r.NumIssueBranches()
			Expect(err).To(BeNil())
			Expect(n).To(Equal(2))
		})

		It("should return 0 when no issue branch exists", func() {
			n, err := r.NumIssueBranches()
			Expect(err).To(BeNil())
			Expect(n).To(Equal(0))
		})
	})

	Describe(".GetRemoteURLs", func() {
		It("should get all remote URLs", func() {
			r.(*rr.Repo).Repository.CreateRemote(&config2.RemoteConfig{Name: "r1", URLs: []string{"http://r.com"}})
			r.(*rr.Repo).Repository.CreateRemote(&config2.RemoteConfig{Name: "r2", URLs: []string{"http://r2.com"}})
			urls := r.GetRemoteURLs()
			Expect(urls).To(ContainElements("http://r.com", "http://r2.com"))
		})

		It("should get only remotes with matching name", func() {
			r.(*rr.Repo).Repository.CreateRemote(&config2.RemoteConfig{Name: "r1", URLs: []string{"http://r.com"}})
			r.(*rr.Repo).Repository.CreateRemote(&config2.RemoteConfig{Name: "r2", URLs: []string{"http://r2.com"}})
			r.(*rr.Repo).Repository.CreateRemote(&config2.RemoteConfig{Name: "r3", URLs: []string{"http://r3.com"}})
			urls := r.GetRemoteURLs("r1", "r3")
			Expect(urls).To(ContainElements("http://r.com", "http://r3.com"))
		})
	})

	Describe(".UpdateRepoConfig & .GetRepoConfig", func() {
		Specify("that .GetRepoConfig returns empty config object when no repo config file exist", func() {
			lcfg, err := r.GetRepoConfig()
			Expect(err).To(BeNil())
			Expect(lcfg).To(Equal(&types.LocalConfig{Tokens: map[string][]string{}}))
		})

		It("should update and get local config object correctly", func() {
			repocfg := &types.LocalConfig{Tokens: map[string][]string{"origin": {"a", "b"}}}
			err = r.UpdateRepoConfig(repocfg)
			Expect(err).To(BeNil())

			repocfg.Tokens["origin"] = append(repocfg.Tokens["origin"], "something")
			err = r.UpdateRepoConfig(repocfg)
			Expect(err).To(BeNil())

			lcfg, err := r.GetRepoConfig()
			Expect(err).To(BeNil())
			Expect(lcfg).To(Equal(&types.LocalConfig{Tokens: map[string][]string{"origin": {"a", "b", "something"}}}))
		})

		It("should set .Token field to zero value if field does not exist", func() {
			repocfg := &types.LocalConfig{Tokens: nil}
			err = r.UpdateRepoConfig(repocfg)
			Expect(err).To(BeNil())

			lcfg, err := r.GetRepoConfig()
			Expect(err).To(BeNil())
			Expect(lcfg.Tokens).To(Not(BeNil()))
		})
	})

	Describe(".GetObjectSize", func() {
		It("should return size of content", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			hash, err := r.GetRecentCommitHash()
			Expect(err).To(BeNil())
			size, err := r.GetObjectSize(hash)
			Expect(err).To(BeNil())
			Expect(size).ToNot(Equal(int64(0)))
		})
	})

	Describe(".GetGitConfigOption", func() {
		It("should empty result if key does not contain a section", func() {
			Expect(r.GetGitConfigOption("key")).To(BeEmpty())
		})

		It("should empty result if key does contains more than one subsections", func() {
			Expect(r.GetGitConfigOption("key")).To(BeEmpty())
		})

		It("should empty result if key does contains more than one subsections", func() {
			Expect(r.GetGitConfigOption("section.key")).To(BeEmpty())
		})

		It("should empty result if key does not exist", func() {
			Expect(r.GetGitConfigOption("section.subsection.subsection.key")).To(BeEmpty())
		})

		It("should expected result if key exist", func() {
			c, err := r.Config()
			Expect(err).To(BeNil())
			c.Raw.Section("section").SetOption("key", "stuff")
			Expect(r.SetConfig(c)).To(BeNil())
			c.Raw.Section("section").Subsection("subsection").SetOption("key", "stuff")
			Expect(r.SetConfig(c)).To(BeNil())
			Expect(r.GetGitConfigOption("section.subsection.key")).To(Equal("stuff"))
		})
	})

	Describe(".IsAncestor", func() {
		It("should return no error when child is a descendant of parent", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			rootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text appended", "commit msg")
			childOfRootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			Expect(r.IsAncestor(rootHash, childOfRootHash)).To(BeNil())
		})

		It("should return ErrNotAncestor when child is not a descendant of parent", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			rootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text appended", "commit msg")
			childOfRootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			err = r.IsAncestor(childOfRootHash, rootHash)
			Expect(err).ToNot(BeNil())
		})

		It("should return non-ErrNotAncestor when child is an unknown hash", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			rootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text appended", "commit msg")
			err = r.IsAncestor(util.RandString(40), rootHash)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(plumbing.ErrObjectNotFound))
		})
	})

	Describe(".IsContributor", func() {
		It("should return true when push key is a repo contributor", func() {
			r.(*rr.Repo).State = &state2.Repository{Contributors: map[string]*state2.RepoContributor{key.PushAddr().String(): {}}}
			Expect(r.IsContributor(key.PushAddr().String())).To(BeTrue())
		})

		It("should return true when push key is a namespace contributor", func() {
			r.(*rr.Repo).Namespace = &state2.Namespace{Contributors: map[string]*state2.BaseContributor{key.PushAddr().String(): {}}}
			Expect(r.IsContributor(key.PushAddr().String())).To(BeTrue())
		})

		It("should return false when push key is a namespace or repo contributor", func() {
			Expect(r.IsContributor(key.PushAddr().String())).To(BeFalse())
		})
	})

	Describe(".IsClean", func() {
		It("should return true when repo is empty", func() {
			clean, err := r.IsClean()
			Expect(err).To(BeNil())
			Expect(clean).To(BeTrue())
		})

		It("should return true when there is a commit", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			clean, err := r.IsClean()
			Expect(err).To(BeNil())
			Expect(clean).To(BeTrue())
		})

		It("should return false when there is an un-staged file", func() {
			testutil2.AppendToFile(path, "file.txt", "un-staged file")
			clean, err := r.IsClean()
			Expect(err).To(BeNil())
			Expect(clean).To(BeFalse())
		})

		It("should return false when there is a staged file", func() {
			testutil2.AppendToFile(path, "file.txt", "staged file")
			testutil2.ExecGitAdd(path, "file.txt")
			clean, err := r.IsClean()
			Expect(err).To(BeNil())
			Expect(clean).To(BeFalse())
		})
	})

	Describe(".GetAncestors", func() {
		var recentHash, c1, c2 string

		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			c1 = testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text 2", "commit msg 2")
			c2 = testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text 3", "commit msg 2")
			recentHash = testutil2.GetRecentCommitHash(path, "refs/heads/master")
		})

		It("should return two hashes (c2, c1)", func() {
			commit, err := r.CommitObject(plumbing.NewHash(recentHash))
			Expect(err).To(BeNil())
			ancestors, err := r.GetAncestors(commit, "", false)
			Expect(err).To(BeNil())
			Expect(ancestors).To(HaveLen(2))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
			Expect(ancestors[1].Hash.String()).To(Equal(c1))
			Expect(ancestors[1].Hash.String()).To(Equal(c1))
		})

		It("should return two hashes (c1, c2) when reverse=true", func() {
			commit, err := r.CommitObject(plumbing.NewHash(recentHash))
			Expect(err).To(BeNil())
			ancestors, err := r.GetAncestors(commit, "", true)
			Expect(err).To(BeNil())
			Expect(ancestors).To(HaveLen(2))
			Expect(ancestors[0].Hash.String()).To(Equal(c1))
			Expect(ancestors[0].Hash.String()).To(Equal(c1))
			Expect(ancestors[1].Hash.String()).To(Equal(c2))
			Expect(ancestors[1].Hash.String()).To(Equal(c2))
		})

		It("should return 1 hashes (c2) when stopHash=c1", func() {
			commit, err := r.CommitObject(plumbing.NewHash(recentHash))
			Expect(err).To(BeNil())
			ancestors, err := r.GetAncestors(commit, c1, false)
			Expect(err).To(BeNil())
			Expect(ancestors).To(HaveLen(1))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
		})
	})

	Describe(".ListPath", func() {
		BeforeEach(func() {
			r, err = rr.GetWithGitModule(cfg.Node.GitBinPath, "testdata/repo1")
			Expect(err).To(BeNil())
		})

		It("should return [/a, air, file.txt, x.exe] when path is empty or '.'", func() {
			for _, path := range []string{"", "."} {
				res, err := r.ListPath("HEAD", path)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(4))
				Expect(res[0].Name).To(Equal("a"))
				Expect(res[1].Name).To(Equal("air"))
				Expect(res[2].Name).To(Equal("file.txt"))
				Expect(res[3].Name).To(Equal("x.exe"))
			}
		})

		It("should return [/b, /c, file2.txt] when path is 'a'", func() {
			res, err := r.ListPath("HEAD", "a")
			Expect(err).To(BeNil())
			Expect(res).To(HaveLen(3))
			Expect(res[0].Name).To(Equal("b"))
			Expect(res[0].IsDir).To(BeTrue())
			Expect(res[0].IsBinary).To(BeFalse())
			Expect(res[0].UpdatedAt).ToNot(BeZero())
			Expect(res[1].Name).To(Equal("c"))
			Expect(res[2].Name).To(Equal("file2.txt"))
			Expect(res[2].IsDir).To(BeFalse())
		})

		It("should return [air] when path is 'air'", func() {
			res, err := r.ListPath("HEAD", "air")
			Expect(err).To(BeNil())
			Expect(res).To(HaveLen(1))
			Expect(res[0].Name).To(Equal("air"))
			Expect(res[0].IsDir).To(BeFalse())
			Expect(res[0].IsBinary).To(BeTrue())
			Expect(res[0].UpdatedAt).ToNot(BeZero())
		})
	})

	Describe(".ReadFileLines", func() {
		BeforeEach(func() {
			r, err = rr.GetWithGitModule(cfg.Node.GitBinPath, "testdata/repo1")
			Expect(err).To(BeNil())
		})

		It("should return error when reference is unknown", func() {
			_, err := r.GetFileLines("unknown", "a")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(plumbing.ErrReferenceNotFound))
		})

		It("should return expected lines when path is 'file.txt'", func() {
			lines, err := r.GetFileLines("HEAD", "file.txt")
			Expect(err).To(BeNil())
			Expect(lines).To(HaveLen(3))
			Expect(lines).To(Equal([]string{
				"Hello World",
				"Hello Friend",
				"Hello Degens",
			}))
		})

		It("should return expected lines when path is 'a/b/file3.txt'", func() {
			lines, err := r.GetFileLines("HEAD", "a/b/file3.txt")
			Expect(err).To(BeNil())
			Expect(lines).To(HaveLen(1))
			Expect(lines).To(Equal([]string{
				"File 3",
			}))
		})

		It("should return expected lines when ref is a commit hash", func() {
			lines, err := r.GetFileLines("435747a11d7186d2e7fb831027e137a9d7104ab5", "a/b/file3.txt")
			Expect(err).To(BeNil())
			Expect(lines).To(HaveLen(1))
			Expect(lines).To(Equal([]string{
				"File 3",
			}))
		})

		It("should return 'path not found' error when path is a different case", func() {
			_, err := r.GetFileLines("HEAD", "a/b/File3.txt")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(rr.ErrPathNotFound))
		})

		It("should return 'path not found' error when path is unknown", func() {
			_, err := r.GetFileLines("HEAD", "unknown")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(rr.ErrPathNotFound))
		})

		It("should return 'path is not a file' error when path is not a file", func() {
			_, err := r.GetFileLines("HEAD", "a")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(rr.ErrPathNotAFile))
		})
	})

	Describe(".GetBranches", func() {
		BeforeEach(func() {
			r, err = rr.GetWithGitModule(cfg.Node.GitBinPath, "testdata/repo1")
			Expect(err).To(BeNil())
		})

		It("should return expected branches", func() {
			branches, err := r.GetBranches()
			Expect(err).To(BeNil())
			Expect(branches).To(Equal([]string{"master"}))
		})
	})

	Describe(".GetLatestCommit", func() {
		BeforeEach(func() {
			r, err = rr.GetWithGitModule(cfg.Node.GitBinPath, "testdata/repo1")
			Expect(err).To(BeNil())
		})

		It("should return an error if branch is unknown", func() {
			_, err := r.GetLatestCommit("unknown")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(plumbing.ErrReferenceNotFound))
		})

		It("should return an error if branch is not branch reference", func() {
			_, err := r.GetLatestCommit("refs/tags/v1")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(plumbing.ErrReferenceNotFound))
		})

		It("should return a commit even when branch name is short", func() {
			bc, err := r.GetLatestCommit("master")
			Expect(err).To(BeNil())
			Expect(bc).ToNot(BeNil())
			Expect(bc.Hash).To(Equal("435747a11d7186d2e7fb831027e137a9d7104ab5"))
		})

		It("should return a commit when branch is valid", func() {
			bc, err := r.GetLatestCommit("refs/heads/master")
			Expect(err).To(BeNil())
			Expect(bc).ToNot(BeNil())
			Expect(bc.Hash).To(Equal("435747a11d7186d2e7fb831027e137a9d7104ab5"))
		})
	})

	Describe(".GetCommits", func() {
		BeforeEach(func() {
			r, err = rr.GetWithGitModule(cfg.Node.GitBinPath, "testdata/repo2")
			Expect(err).To(BeNil())
		})

		It("should return an error if branch is unknown", func() {
			_, err := r.GetCommits("unknown", 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(plumbing.ErrReferenceNotFound))
		})

		It("should return an error if branch is not branch reference", func() {
			_, err := r.GetCommits("refs/tags/v1", 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(plumbing.ErrReferenceNotFound))
		})

		It("should return a commits even when branch name is short", func() {
			commits, err := r.GetCommits("master", 0)
			Expect(err).To(BeNil())
			Expect(commits).To(HaveLen(11))
			Expect(commits[0].Hash).To(Equal("bc2d3657cad5fb7a3ed2f4f9b178c38587ba2fc6"))
			Expect(commits[10].Hash).To(Equal("932401fb0bf48f602c501334b773fbc3422ceb31"))
		})

		It("should return a commits even when branch name is short", func() {
			commits, err := r.GetCommits("refs/heads/master", 0)
			Expect(err).To(BeNil())
			Expect(commits).To(HaveLen(11))
			Expect(commits[0].Hash).To(Equal("bc2d3657cad5fb7a3ed2f4f9b178c38587ba2fc6"))
			Expect(commits[10].Hash).To(Equal("932401fb0bf48f602c501334b773fbc3422ceb31"))
		})

		It("should return limited number if commits when limit is > 0", func() {
			commits, err := r.GetCommits("master", 3)
			Expect(err).To(BeNil())
			Expect(commits).To(HaveLen(3))
			Expect(commits[0].Hash).To(Equal("bc2d3657cad5fb7a3ed2f4f9b178c38587ba2fc6"))
			Expect(commits[2].Hash).To(Equal("d6a23829e6787f8d16bd61effad57b88b500167a"))
		})
	})

	Describe(".GetCommitAncestors", func() {
		BeforeEach(func() {
			r, err = rr.GetWithGitModule(cfg.Node.GitBinPath, "testdata/repo1")
			Expect(err).To(BeNil())
		})

		It("should return an error if commit is not unknown", func() {
			_, err := r.GetCommitAncestors("bad_hash", 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(plumbing.ErrObjectNotFound))
		})

		It("should return ancestors even when commit exists and has ancestors", func() {
			commits, err := r.GetCommitAncestors("aef606780a3f857fdd7fe8270efa547f118bef5f", 0)
			Expect(err).To(BeNil())
			Expect(commits).To(HaveLen(5))
			Expect(commits[0].Hash).To(Equal("c28e295ca030fa4ac9537f9f583f6b4b48be302b"))
			Expect(commits[4].Hash).To(Equal("932401fb0bf48f602c501334b773fbc3422ceb31"))
		})

		It("should return limited ancestors even when limit is > 0", func() {
			commits, err := r.GetCommitAncestors("aef606780a3f857fdd7fe8270efa547f118bef5f", 1)
			Expect(err).To(BeNil())
			Expect(commits).To(HaveLen(1))
			Expect(commits[0].Hash).To(Equal("c28e295ca030fa4ac9537f9f583f6b4b48be302b"))
		})
	})
})
