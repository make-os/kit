package repo

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("PushReader", func() {
	var err error
	var cfg *config.EngineConfig
	var path string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		_, err = getRepo(path)
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
					"obj1": []string{"ref1", "ref2"},
					"obj2": []string{"ref", "ref2"},
					"obj3": []string{"ref1", "ref3"},
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
						"obj1": []string{"ref1", "ref2"},
						"obj2": []string{"ref", "ref2"},
						"obj3": []string{"ref1", "ref3"},
					})
					err := m.removeRef("obj2", "ref2")
					Expect(err).To(BeNil())
					Expect(m["obj2"]).To(HaveLen(1))
					Expect(m["obj2"]).To(ConsistOf("ref"))
				})

				It("should return err if object is not found", func() {
					m := objRefMap(map[string][]string{
						"obj1": []string{"ref1", "ref2"},
					})
					err := m.removeRef("obj2", "ref2")
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("object not found"))
				})
			})
		})
	})
})
