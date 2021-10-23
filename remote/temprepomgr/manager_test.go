package temprepomgr

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTempRepoManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BasicTempRepoManager Suite")
}

var _ = Describe("BasicTempRepoManager", func() {
	Describe(".Add", func() {
		It("should add a path", func() {
			m := New()
			id := m.Add("a/dir")
			Expect(id).ToNot(BeEmpty())
			Expect(m.entries).To(HaveLen(1))
		})

		It("should overwrite Entry if path already exists", func() {
			dir := "a/dir"
			m := New()
			id := m.Add(dir)
			Expect(id).ToNot(BeEmpty())
			Expect(m.entries).To(HaveLen(1))
			touchedAt := m.entries[id].touchedAt

			id2 := m.Add(dir)
			Expect(m.entries).To(HaveLen(1))
			Expect(id).To(Equal(id2))
			Expect(m.entries[id2].touchedAt).ToNot(Equal(touchedAt))
		})
	})

	Describe(".GetPath", func() {
		It("should return empty string if no path is associated with the ID", func() {
			m := New()
			Expect(m.GetPath("some_id")).To(BeEmpty())
		})

		It("should return path if path was found", func() {
			dir := "a/dir"
			m := New()
			id := m.Add(dir)
			path := m.GetPath(id)
			Expect(path).To(Equal(dir))
		})
	})

	Describe(".Remove", func() {
		It("should remove path from index and filesystem", func() {
			dir, err := os.MkdirTemp("", "")
			Expect(err).To(BeNil())
			m := New()
			id := m.Add(dir)
			Expect(m.entries).To(HaveLen(1))
			m.Remove(id)
			Expect(m.entries).To(HaveLen(0))
			_, err = os.Stat(dir)
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("check old Entry remover timer", func() {
		It("should remove old entries", func() {
			maxDuration = 1 * time.Millisecond
			timerDuration = 2 * time.Millisecond
			dir, err := os.MkdirTemp("", "")
			Expect(err).To(BeNil())
			m := New()
			m.Add(dir)
			Expect(m.entries).To(HaveLen(1))
			time.Sleep(5 * time.Millisecond)
			Expect(m.entries).To(HaveLen(0))
		})
	})
})
