package pushhandler_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/pushhandler"
	types2 "gitlab.com/makeos/mosdef/remote/pushpool/types"
	repo3 "gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gopkg.in/src-d/go-git.v4/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thoas/go-funk"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
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
	var repo types2.LocalRepo

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
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe("PackedReferences", func() {
		Describe(".Names", func() {
			It("should return the Names of references", func() {
				packedRefs := pushhandler.PackedReferences{
					"ref1": {OldHash: plumbing.ZeroHash.String()},
					"ref2": {OldHash: plumbing.ZeroHash.String()},
				}
				Expect(packedRefs.Names()).To(ContainElement("ref1"))
				Expect(packedRefs.Names()).To(ContainElement("ref2"))
			})
		})
	})

	Describe("pushhandler.ObjRefMap", func() {
		Describe(".GetObjectsOf", func() {
			It("should return expected objects", func() {
				m := pushhandler.ObjRefMap(map[string][]string{
					"obj1": {"ref1", "ref2"},
					"obj2": {"ref", "ref2"},
					"obj3": {"ref1", "ref3"},
				})
				objs := m.GetObjectsOf("ref1")
				Expect(objs).To(HaveLen(2))
				Expect(objs).To(ConsistOf("obj1", "obj3"))
			})
		})

		Describe(".RemoveRef", func() {
			Describe(".GetObjectsOf", func() {
				It("should remove ref2 from obj2 ", func() {
					m := pushhandler.ObjRefMap(map[string][]string{
						"obj1": {"ref1", "ref2"},
						"obj2": {"ref", "ref2"},
						"obj3": {"ref1", "ref3"},
					})
					err := m.RemoveRef("obj2", "ref2")
					Expect(err).To(BeNil())
					Expect(m["obj2"]).To(HaveLen(1))
					Expect(m["obj2"]).To(ConsistOf("ref"))
				})

				It("should return err if object is not found", func() {
					m := pushhandler.ObjRefMap(map[string][]string{
						"obj1": {"ref1", "ref2"},
					})
					err := m.RemoveRef("obj2", "ref2")
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("object not found"))
				})
			})
		})
	})

	Describe("pushReader", func() {
		var pr *pushhandler.PushReader
		var dst = bytes.NewBuffer(nil)
		var err error

		BeforeEach(func() {
			oldState := plumbing2.GetRepoState(repo)
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			newState := plumbing2.GetRepoState(repo)

			reader, err := pushhandler.MakePackfile(repo, oldState, newState)
			Expect(err).To(BeNil())
			packData, err := ioutil.ReadAll(reader)
			Expect(err).To(BeNil())

			pr, err = pushhandler.NewPushReader(&WriteCloser{Buffer: dst}, repo)
			pr.Write(packData)
			err = pr.Read()
		})

		It("should return no error", func() {
			Expect(err).To(BeNil())
		})

		Specify("that the push reader decoded 3 objects", func() {
			Expect(pr.Objects).To(HaveLen(3))
		})

		Specify("that only 1 ref is decoded", func() {
			refs := pr.References
			Expect(refs).To(HaveLen(1))
			Expect(refs.Names()).To(Equal([]string{"refs/heads/master"}))
		})

		Specify("object ref map has 3 objects with value 'refs/heads/master'", func() {
			Expect(pr.ObjectsRefs).To(HaveLen(3))
			Expect(funk.Values(pr.ObjectsRefs)).To(Equal([][]string{
				{"refs/heads/master"},
				{"refs/heads/master"},
				{"refs/heads/master"}}))
		})
	})
})
