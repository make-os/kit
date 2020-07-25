package push_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/themakeos/lobe/remote/push"
	types2 "github.com/themakeos/lobe/remote/push/types"
	repo3 "github.com/themakeos/lobe/remote/repo"
	testutil2 "github.com/themakeos/lobe/remote/testutil"
	"github.com/themakeos/lobe/remote/types"
	"gopkg.in/src-d/go-git.v4/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/testutil"
	"github.com/themakeos/lobe/util"
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
		repo, err = repo3.GetWithLiteGit(cfg.Node.GitBinPath, path)
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
		var pr *push.PushReader
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
			err = pr.Read(nil)
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
})
