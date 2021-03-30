package push_test

import (
	"io"
	"os"
	"path/filepath"

	"github.com/make-os/kit/config"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/push"
	"github.com/make-os/kit/remote/push/types"
	repo3 "github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
)

var _ = Describe("PackHelpers", func() {
	var cfg *config.AppConfig
	var repo remotetypes.LocalRepo
	var path string
	var err error

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo3.GetWithGitModule(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".MakeReferenceUpdateRequestPack", func() {
		When("reference's new hash is a zero hash", func() {
			var pack io.ReadSeeker
			BeforeEach(func() {
				note := types.Note{References: []*types.PushedReference{
					{Name: "refs/heads/master", NewHash: plumbing.ZeroHash.String(), OldHash: "e070e3147d617e026e6ac08f1aac9ca3d0ae561a"},
				}}
				note.SetTargetRepo(repo)
				pack, err = push.MakeReferenceUpdateRequestPack(&note)
				Expect(err).To(BeNil())
			})

			Specify("that the packfile has the pushed reference and no object", func() {
				req := packp.NewReferenceUpdateRequest()
				err = req.Decode(pack)
				Expect(err).To(BeNil())
				Expect(req.Commands).To(HaveLen(1))
				Expect(req.Commands[0].Name.String()).To(Equal("refs/heads/master"))

				objects := 0
				err = plumbing2.UnpackPackfile(testutil.WrapReadSeeker{Rdr: req.Packfile}, func(header *packfile.ObjectHeader,
					read func() (object.Object, error)) error {
					objects++
					return nil
				})
				Expect(err).To(BeNil())
				Expect(objects).To(BeZero())
			})
		})

		When("reference's new hash is not a zero hash", func() {
			var pack io.ReadSeeker
			var commit2Hash string
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text msg", "commit msg")
				commit2Hash = testutil2.GetRecentCommitHash(path, "refs/heads/master")

				note := types.Note{References: []*types.PushedReference{
					{Name: "refs/heads/master", NewHash: commit2Hash, OldHash: plumbing.ZeroHash.String()},
				}}

				note.SetTargetRepo(repo)
				pack, err = push.MakeReferenceUpdateRequestPack(&note)
				Expect(err).To(BeNil())
			})

			Specify("that the packfile has the pushed reference and 1 object", func() {
				req := packp.NewReferenceUpdateRequest()
				err = req.Decode(pack)
				Expect(err).To(BeNil())
				Expect(req.Commands).To(HaveLen(1))
				Expect(req.Commands[0].Name.String()).To(Equal("refs/heads/master"))

				objects := 0
				err = plumbing2.UnpackPackfile(testutil.WrapReadSeeker{Rdr: req.Packfile}, func(header *packfile.ObjectHeader,
					read func() (object.Object, error)) error {
					objects++
					return nil
				})

				Expect(err).To(BeNil())
				Expect(objects).To(Equal(1))
			})
		})
	})

	Describe(".GetSizeOfObjects", func() {
		It("should return error when note does not contain a non-nil repo reference", func() {
			note := &types.Note{}
			_, err := push.GetSizeOfObjects(note)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("repo is required"))
		})

		It("should return expected size", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commit1Hash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			note := types.Note{References: []*types.PushedReference{
				{Name: "refs/heads/master", NewHash: commit1Hash, OldHash: plumbing.ZeroHash.String()},
			}}
			note.SetTargetRepo(repo)
			size, err := push.GetSizeOfObjects(&note)
			Expect(err).To(BeNil())

			commit1TotalSize := uint64(0)
			commit1, _ := repo.CommitObject(plumbing.NewHash(commit1Hash))
			commitSize, _ := repo.GetStorer().EncodedObjectSize(commit1.Hash)
			commit1TotalSize += uint64(commitSize)

			treeSize, _ := repo.GetStorer().EncodedObjectSize(commit1.TreeHash)
			commit1TotalSize += uint64(treeSize)

			tree, _ := commit1.Tree()
			for _, ent := range tree.Entries {
				treeSize, _ := repo.GetStorer().EncodedObjectSize(ent.Hash)
				commit1TotalSize += uint64(treeSize)
			}

			Expect(size).To(Equal(commit1TotalSize))
		})
	})
})
