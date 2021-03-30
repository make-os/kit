package push_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/make-os/kit/remote/push"
	types2 "github.com/make-os/kit/remote/push/types"
	repo3 "github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/remote/types"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type WriteCloser struct {
	*bytes.Buffer
}

func (mwc *WriteCloser) Close() error {
	return nil
}

var _ = Describe("PushReader", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo types.LocalRepo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo3.GetWithGitModule(cfg.Node.GitBinPath, path)
		_ = repo
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe("PackedReferences", func() {
		Describe(".Names", func() {
			It("should return the Names of references", func() {
				packedRefs := push.PackedReferences{
					"ref1": {OldHash: plumbing.ZeroHash.String()},
					"ref2": {OldHash: plumbing.ZeroHash.String()},
				}
				Expect(packedRefs.Names()).To(ContainElement("ref1"))
				Expect(packedRefs.Names()).To(ContainElement("ref2"))
			})
		})
	})

	Describe(".Read", func() {
		var pr *push.Reader
		var dst = bytes.NewBuffer(nil)
		var err error

		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commit1Hash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			note := types2.Note{
				TargetRepo: repo,
				References: []*types2.PushedReference{
					{Name: "refs/heads/master", NewHash: commit1Hash, OldHash: plumbing.ZeroHash.String()},
				},
			}

			reader, err := push.MakeReferenceUpdateRequestPack(&note)
			Expect(err).To(BeNil())

			pr, err = push.NewPushReader(&WriteCloser{Buffer: dst}, repo)
			Expect(err).To(BeNil())

			io.Copy(pr, reader)
			err = pr.Read()
		})

		It("should return no error", func() {
			Expect(err).To(BeNil())
		})

		Specify("that the push reader decoded 1 object", func() {
			Expect(pr.Objects).To(HaveLen(1))
		})

		Specify("that only 1 ref is decoded", func() {
			refs := pr.References
			Expect(refs).To(HaveLen(1))
			Expect(refs.Names()).To(Equal([]string{"refs/heads/master"}))
		})
	})

	Describe("ObjectObserver", func() {
		Describe(".OnInflatedObjectHeader", func() {
			It("should return error if blob object exceeded MaxBlobSize", func() {
				oo := push.ObjectObserver{MaxBlobSize: 100}
				err := oo.OnInflatedObjectHeader(plumbing.BlobObject, 111, 1)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("size error: a file's size exceeded the network limit"))
			})

			It("should add new object entry if size of blob did not exceed MaxBlobSize", func() {
				oo := push.ObjectObserver{MaxBlobSize: 100}
				err := oo.OnInflatedObjectHeader(plumbing.BlobObject, 100, 1)
				Expect(err).To(BeNil())
				Expect(oo.Objects).To(HaveLen(1))
				Expect(oo.Objects[0].Type).To(Equal(plumbing.BlobObject))
			})
		})

		Describe(".OnInflatedObjectContent", func() {
			It("should set hash of object", func() {
				oo := push.ObjectObserver{MaxBlobSize: 100}
				oo.OnInflatedObjectHeader(plumbing.BlobObject, 100, 1)
				hash := plumbing.NewHash("d43c6e3a78216a44ecd0aba54e8cf888547b634a")
				err := oo.OnInflatedObjectContent(hash, 0, 0, nil)
				Expect(err).To(BeNil())
				Expect(oo.Objects).To(HaveLen(1))
				Expect(oo.Objects[0].Hash).To(Equal(hash))
			})
		})
	})
})
