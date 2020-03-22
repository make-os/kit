package repo

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"gitlab.com/makeos/mosdef/types/core"

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
	var repo core.BareRepo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe("packedReferences", func() {
		Describe(".names", func() {
			It("should return the names of references", func() {
				packedRefs := packedReferences([]*packedReferenceObject{
					{name: "ref1"},
					{name: "ref2"},
				})
				Expect(packedRefs.names()).To(Equal([]string{"ref1", "ref2"}))
			})
		})
	})

	Describe("objRefMap", func() {
		Describe(".getObjectsOf", func() {
			It("should return expected objects", func() {
				m := objRefMap(map[string][]string{
					"obj1": {"ref1", "ref2"},
					"obj2": {"ref", "ref2"},
					"obj3": {"ref1", "ref3"},
				})
				objs := m.getObjectsOf("ref1")
				Expect(objs).To(HaveLen(2))
				Expect(objs).To(ConsistOf("obj1", "obj3"))
			})
		})

		Describe(".removeRef", func() {
			Describe(".getObjectsOf", func() {
				It("should remove ref2 from obj2 ", func() {
					m := objRefMap(map[string][]string{
						"obj1": {"ref1", "ref2"},
						"obj2": {"ref", "ref2"},
						"obj3": {"ref1", "ref3"},
					})
					err := m.removeRef("obj2", "ref2")
					Expect(err).To(BeNil())
					Expect(m["obj2"]).To(HaveLen(1))
					Expect(m["obj2"]).To(ConsistOf("ref"))
				})

				It("should return err if object is not found", func() {
					m := objRefMap(map[string][]string{
						"obj1": {"ref1", "ref2"},
					})
					err := m.removeRef("obj2", "ref2")
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("object not found"))
				})
			})
		})
	})

	Describe("pushReader", func() {
		var pr *PushReader
		var dst = bytes.NewBuffer(nil)
		var err error

		BeforeEach(func() {
			oldState := getRepoState(repo)
			appendCommit(path, "file.txt", "some text", "commit msg")
			newState := getRepoState(repo)

			reader, err := makePackfile(repo, oldState, newState)
			Expect(err).To(BeNil())
			packData, err := ioutil.ReadAll(reader)
			Expect(err).To(BeNil())

			pr, err = newPushReader(&WriteCloser{Buffer: dst}, repo)
			pr.Write(packData)
			err = pr.Read()
		})

		It("should return no error", func() {
			Expect(err).To(BeNil())
		})

		Specify("that the push reader decoded 3 objects", func() {
			Expect(pr.objects).To(HaveLen(3))
		})

		Specify("that only 1 ref is decoded", func() {
			refs := pr.references
			Expect(refs).To(HaveLen(1))
			Expect(refs.names()).To(Equal([]string{"refs/heads/master"}))
		})

		Specify("object ref map has 3 objects with value 'refs/heads/master'", func() {
			Expect(pr.objectsRefs).To(HaveLen(3))
			Expect(funk.Values(pr.objectsRefs)).To(Equal([][]string{
				{"refs/heads/master"},
				{"refs/heads/master"},
				{"refs/heads/master"}}))
		})
	})
})
