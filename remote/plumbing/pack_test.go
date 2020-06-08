package plumbing_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	r "gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/testutil"
	types2 "gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("Packfile", func() {
	var err error
	var cfg *config.AppConfig
	var repo types.LocalRepo
	var repoName, path string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)

		repo, err = r.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".PackCommit", func() {
		When("only one commit exist in the repo", func() {
			var pack io.Reader
			var err error

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
				pack, err = plumbing2.PackCommit(repo, plumbing.NewHash(commitHash))
			})

			It("should successfully pack the commit", func() {
				Expect(err).To(BeNil())
			})

			It("should pack 3 objects", func() {
				scn := packfile.NewScanner(pack)
				_, objCount, err := scn.Header()
				Expect(err).To(BeNil())
				Expect(objCount).To(Equal(uint32(3)))
			})
		})
	})

	Describe(".UnPackfile", func() {
		var pack io.Reader
		var err error
		var commit *object.Commit

		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			pack, err = plumbing2.PackCommit(repo, plumbing.NewHash(commitHash))
			Expect(err).To(BeNil())
			commit, err = repo.CommitObject(plumbing.NewHash(commitHash))
			Expect(err).To(BeNil())
		})

		It("should decode packfile correctly", func() {
			var hashes = map[string]struct{}{}
			err := plumbing2.UnPackfile(testutil.WrapReadSeeker{pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
				o, err := read()
				Expect(err).To(BeNil())
				hashes[o.ID().String()] = struct{}{}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(hashes).To(HaveLen(3))
			Expect(hashes).To(HaveKey(commit.Hash.String()))
			Expect(hashes).To(HaveKey(commit.TreeHash.String()))
			tree, _ := commit.Tree()
			Expect(hashes).To(HaveKey(tree.Entries[0].Hash.String()))
		})

		It("should return error if packfile is malformed", func() {
			var hashes = map[string]struct{}{}
			err := plumbing2.UnPackfile(
				testutil.WrapReadSeeker{strings.NewReader("bad bad packfile")},
				func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
					o, err := read()
					Expect(err).To(BeNil())
					hashes[o.ID().String()] = struct{}{}
					return nil
				})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("bad packfile: malformed pack file signature"))
		})

		It("should return non-ErrExit error returned by the callback and it should exit immediately", func() {
			var hashes = map[string]struct{}{}
			err := plumbing2.UnPackfile(testutil.WrapReadSeeker{pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
				o, err := read()
				Expect(err).To(BeNil())
				hashes[o.ID().String()] = struct{}{}
				return fmt.Errorf("error here")
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error here"))
			Expect(hashes).To(HaveLen(1))
		})

		It("should return nil if ErrExit is returned by the callback and it should exit immediately", func() {
			var hashes = map[string]struct{}{}
			err := plumbing2.UnPackfile(testutil.WrapReadSeeker{pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
				o, err := read()
				Expect(err).To(BeNil())
				hashes[o.ID().String()] = struct{}{}
				return types2.ErrExit
			})
			Expect(err).To(BeNil())
			Expect(hashes).To(HaveLen(1))
		})
	})
})
