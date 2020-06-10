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
	pl "gitlab.com/makeos/mosdef/remote/plumbing"
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

type fakePackfile struct {
	name string
}

func (f *fakePackfile) Read(p []byte) (n int, err error) {
	return
}

func (f *fakePackfile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (f *fakePackfile) Close() error {
	return nil
}

type gitObject struct {
	typ plumbing.ObjectType
	id  plumbing.Hash
}

func (u gitObject) ID() plumbing.Hash {
	return u.id
}

func (u *gitObject) Type() plumbing.ObjectType {
	return u.typ
}

func (u *gitObject) Decode(encodedObject plumbing.EncodedObject) error {
	panic("implement me")
}

func (u *gitObject) Encode(encodedObject plumbing.EncodedObject) error {
	panic("implement me")
}

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

	Describe(".PackObject", func() {

		It("should set default filter type if unset", func() {
			args := &pl.PackObjectArgs{Obj: &gitObject{}}
			_, _, err := pl.PackObject(repo, args)
			Expect(err).ToNot(BeNil())
			Expect(args.Filter).ToNot(BeNil())
		})

		Context("object type is unknown", func() {
			It("should return error", func() {
				_, _, err := pl.PackObject(repo, &pl.PackObjectArgs{Obj: &gitObject{}})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("unsupported object type"))
			})
		})

		Context("object is a commit", func() {
			var pack io.Reader
			var err error

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
				commit, _ := repo.CommitObject(plumbing.NewHash(commitHash))
				pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: commit})
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

		When("object is a commit and filter returned false for its tree", func() {
			var pack io.Reader
			var err error
			var commit *object.Commit

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
				commit, _ = repo.CommitObject(plumbing.NewHash(commitHash))
				pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: commit, Filter: func(hash plumbing.Hash) bool {
					return hash.String() != commit.TreeHash.String()
				}})
			})

			It("should successfully pack the commit", func() {
				Expect(err).To(BeNil())
			})

			It("should pack 1 object (the commit and the tree entries)", func() {
				objs := []string{}
				pl.UnpackPackfile(testutil.WrapReadSeeker{Rdr: pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
					obj, _ := read()
					objs = append(objs, obj.ID().String())
					return nil
				})
				Expect(objs).To(HaveLen(2))
				Expect(objs).To(ContainElement(commit.Hash.String()))
				tree, _ := commit.Tree()
				Expect(objs).To(ContainElement(tree.Entries[0].Hash.String()))
			})
		})

		Context("object is a commit with files and a tree", func() {
			var pack io.Reader
			var err error
			var commit *object.Commit

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "dir/file.txt", "some text", "commit msg")
				commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
				commit, _ = repo.CommitObject(plumbing.NewHash(commitHash))
				pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: commit})
			})

			It("should successfully pack the commit", func() {
				Expect(err).To(BeNil())
			})

			It("should pack 3 objects", func() {
				objs := []string{}
				pl.UnpackPackfile(testutil.WrapReadSeeker{Rdr: pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
					obj, _ := read()
					objs = append(objs, obj.ID().String())
					return nil
				})
				Expect(objs).To(HaveLen(5))
				Expect(objs).To(ContainElement(commit.Hash.String()))
				Expect(objs).To(ContainElement(commit.TreeHash.String()))
				tree, _ := commit.Tree()
				for _, e := range tree.Entries {
					Expect(objs).To(ContainElement(e.Hash.String()))
					if !e.Mode.IsFile() {
						dirObj, _ := repo.GetObject(e.Hash.String())
						Expect(objs).To(ContainElement(dirObj.ID().String()))
					}
				}
			})
		})

		Context("object is a tag pointed to a commit", func() {
			var pack io.Reader
			var err error

			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "some content", "commit msg", "tag1")
				tagHash, err := repo.RefGet("refs/tags/tag1")
				Expect(err).To(BeNil())
				tag, _ := repo.TagObject(plumbing.NewHash(tagHash))
				pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: tag})
			})

			It("should successfully pack the commit", func() {
				Expect(err).To(BeNil())
			})

			It("should pack 4 objects", func() {
				scn := packfile.NewScanner(pack)
				_, objCount, err := scn.Header()
				Expect(err).To(BeNil())
				Expect(objCount).To(Equal(uint32(4)))
			})
		})

		Context("object is a tag pointed to a tag", func() {
			var pack io.Reader
			var err error

			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "some content", "commit msg", "tag1")
				testutil2.CreateTagPointedToTag(path, "message", "tag2", "tag1")
				tagHash, err := repo.RefGet("refs/tags/tag2")
				Expect(err).To(BeNil())
				tag, _ := repo.TagObject(plumbing.NewHash(tagHash))
				pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: tag})
			})

			It("should successfully pack the tag", func() {
				Expect(err).To(BeNil())
			})

			It("should pack 5 objects", func() {
				scn := packfile.NewScanner(pack)
				_, objCount, err := scn.Header()
				Expect(err).To(BeNil())
				Expect(objCount).To(Equal(uint32(5)))
			})
		})

		Context("object is a tag pointed to a blob", func() {
			var pack io.Reader
			var err error

			BeforeEach(func() {
				blobHash := testutil2.CreateBlob(path, "blob.txt")
				testutil2.CreateTagPointedToTag(path, "message", "tag2", blobHash)
				tagHash, err := repo.RefGet("refs/tags/tag2")
				Expect(err).To(BeNil())
				tag, _ := repo.TagObject(plumbing.NewHash(tagHash))
				pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: tag})
			})

			It("should successfully pack the tag", func() {
				Expect(err).To(BeNil())
			})

			It("should pack 2 objects", func() {
				scn := packfile.NewScanner(pack)
				_, objCount, err := scn.Header()
				Expect(err).To(BeNil())
				Expect(objCount).To(Equal(uint32(2)))
			})
		})

		Context("object is a tag pointed to a tree", func() {
			var pack io.Reader
			var err error

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
				commit, _ := repo.CommitObject(plumbing.NewHash(commitHash))
				testutil2.CreateTagPointedToTag(path, "message", "tag2", commit.TreeHash.String())
				tagHash, err := repo.RefGet("refs/tags/tag2")
				Expect(err).To(BeNil())
				tag, _ := repo.TagObject(plumbing.NewHash(tagHash))
				pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: tag})
			})

			It("should successfully pack the tag", func() {
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

	Describe(".UnpackPackfile", func() {
		var pack io.Reader
		var err error
		var commit *object.Commit

		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			commit, _ = repo.CommitObject(plumbing.NewHash(commitHash))
			pack, _, err = pl.PackObject(repo, &pl.PackObjectArgs{Obj: commit})
			Expect(err).To(BeNil())
			commit, err = repo.CommitObject(plumbing.NewHash(commitHash))
			Expect(err).To(BeNil())
		})

		It("should return error when pack is nil", func() {
			err := pl.UnpackPackfile(nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("pack is nil"))
		})

		It("should decode packfile correctly", func() {
			var hashes = map[string]struct{}{}
			err := pl.UnpackPackfile(testutil.WrapReadSeeker{Rdr: pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
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
			err := pl.UnpackPackfile(
				testutil.WrapReadSeeker{Rdr: strings.NewReader("bad bad packfile")},
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
			err := pl.UnpackPackfile(testutil.WrapReadSeeker{Rdr: pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
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
			err := pl.UnpackPackfile(testutil.WrapReadSeeker{Rdr: pack}, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
				o, err := read()
				Expect(err).To(BeNil())
				hashes[o.ID().String()] = struct{}{}
				return types2.ErrExit
			})
			Expect(err).To(BeNil())
			Expect(hashes).To(HaveLen(1))
		})
	})

	Describe(".UnpackPackfileToRepo", func() {
		var dest types.LocalRepo

		BeforeEach(func() {
			repo2Name := util.RandString(5)
			path2 := filepath.Join(cfg.GetRepoRoot(), repo2Name)
			testutil2.ExecGit(cfg.GetRepoRoot(), "init", repo2Name)
			dest, err = r.GetWithLiteGit(cfg.Node.GitBinPath, path2)
			Expect(err).To(BeNil())
		})

		It("should write pack objects to a given repository", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			commit, _ := repo.CommitObject(plumbing.NewHash(commitHash))
			pack, _, err := pl.PackObject(repo, &pl.PackObjectArgs{Obj: commit})
			Expect(err).To(BeNil())
			err = pl.UnpackPackfileToRepo(dest, testutil.WrapReadSeekerCloser{Rdr: pack})
			Expect(err).To(BeNil())
			res, err := dest.CommitObject(plumbing.NewHash(commitHash))
			Expect(err).To(BeNil())
			Expect(res.Hash).To(Equal(commit.Hash))
		})
	})

	Describe(".GetObjectFromPack", func() {
		It("should return nil if pack does not contain the given hash", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			commit, _ := repo.CommitObject(plumbing.NewHash(commitHash))
			pack, _, err := pl.PackObject(repo, &pl.PackObjectArgs{Obj: commit})
			Expect(err).To(BeNil())
			obj, err := pl.GetObjectFromPack(testutil.WrapReadSeeker{Rdr: pack}, "")
			Expect(err).To(BeNil())
			Expect(obj).To(BeNil())
		})

		It("should return object if pack contains the given hash", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			commit, _ := repo.CommitObject(plumbing.NewHash(commitHash))
			pack, _, err := pl.PackObject(repo, &pl.PackObjectArgs{Obj: commit})
			Expect(err).To(BeNil())
			obj, err := pl.GetObjectFromPack(testutil.WrapReadSeeker{Rdr: pack}, commitHash)
			Expect(err).To(BeNil())
			Expect(obj).ToNot(BeNil())
			Expect(obj.ID().String()).To(Equal(commitHash))
		})

		It("should return error if packfile is bad", func() {
			Expect(err).To(BeNil())
			_, err := pl.GetObjectFromPack(testutil.WrapReadSeeker{Rdr: strings.NewReader("bad pack")}, "")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("bad packfile"))
		})
	})
})
