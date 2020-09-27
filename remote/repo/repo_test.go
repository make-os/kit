package repo_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/make-os/lobe/crypto"
	rr "github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/remote/types"
	state2 "github.com/make-os/lobe/types/state"
	config2 "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/make-os/lobe/config"
	testutil2 "github.com/make-os/lobe/remote/testutil"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
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
	var key *crypto.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		key = crypto.NewKeyFromIntSeed(1)
	})

	BeforeEach(func() {
		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		r, err = rr.GetWithLiteGit(cfg.Node.GitBinPath, path)
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

	Describe(".UpdateCredentialFile", func() {
		It("should return error if url is malformed", func() {
			err := r.UpdateCredentialFile("http://x.com:www")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("bad url"))
		})

		It("should add url to credential file", func() {
			err := r.UpdateCredentialFile("http://user:pass@127.0.0.1:9002/r/repo1")
			Expect(err).To(BeNil())
			credentialFile := filepath.Join(r.GetPath(), ".git/.git-credentials")
			bz, err := ioutil.ReadFile(credentialFile)
			Expect(err).To(BeNil())
			Expect(string(bz)).To(Equal("http://user:pass@127.0.0.1:9002/r/repo1"))

			// Add unique URL
			err = r.UpdateCredentialFile("http://xyz:abc@127.0.0.2:9002/r/repo1")
			Expect(err).To(BeNil())

			// Add matching URL
			err = r.UpdateCredentialFile("http://user2:pass2@127.0.0.1:9002/r/repo1")
			Expect(err).To(BeNil())

			bz, err = ioutil.ReadFile(credentialFile)
			Expect(err).To(BeNil())
			parts := bytes.Split(bz, []byte("\n"))
			Expect(parts).To(ContainElement([]byte("http://user2:pass2@127.0.0.1:9002/r/repo1")))
			Expect(parts).To(ContainElement([]byte("http://xyz:abc@127.0.0.2:9002/r/repo1")))
		})

		It("should remove bad urls found in the file", func() {
			credentialFile := filepath.Join(r.GetPath(), ".git/.git-credentials")
			err = ioutil.WriteFile(credentialFile, []byte("http://x.com:bad-url"), 0644)
			Expect(err).To(BeNil())

			err = r.UpdateCredentialFile("http://user2:pass2@127.0.0.1:9002/r/repo1")
			Expect(err).To(BeNil())

			bz, err := ioutil.ReadFile(credentialFile)
			Expect(err).To(BeNil())
			parts := bytes.Split(bz, []byte("\n"))
			Expect(parts).To(ContainElement([]byte("http://user2:pass2@127.0.0.1:9002/r/repo1")))
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
	})

	Describe(".ReadCredentialFile", func() {
		It("should return error if credential file does not exist", func() {
			_, err := r.ReadCredentialFile()
			Expect(err).ToNot(BeNil())
		})

		It("should return valid urls if file exists", func() {
			err = r.UpdateCredentialFile("http://user2:pass2@127.0.0.1:9001/r/repo1")
			Expect(err).To(BeNil())
			err = r.UpdateCredentialFile("http://user2:pass2@127.0.0.1:9002/r/repo1")
			Expect(err).To(BeNil())
			urls, err := r.ReadCredentialFile()
			Expect(err).To(BeNil())
			Expect(urls).To(HaveLen(2))
			Expect(urls).To(ContainElement("http://user2:pass2@127.0.0.1:9001/r/repo1"))
			Expect(urls).To(ContainElement("http://user2:pass2@127.0.0.1:9002/r/repo1"))
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
})
