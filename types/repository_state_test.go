package types

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repository", func() {
	Describe(".Bytes & .NewRepositoryFromBytes", func() {
		var r *Repository
		var expectedBz []byte

		BeforeEach(func() {
			r = BareRepository()
			r.CreatorAddress = "some_address"
			r.References = map[string]*Reference{
				"refs/heads/master": &Reference{
					Nonce: 20,
				},
			}
			expectedBz = r.Bytes()
		})

		It("should return bytes", func() {
			Expect(expectedBz).ToNot(BeEmpty())
		})

		Describe(".NewRepositoryFromBytes", func() {
			It("should return object", func() {
				r, err := NewRepositoryFromBytes(expectedBz)
				Expect(err).To(BeNil())
				Expect(r).To(Equal(r))
			})

			Context("with malformed byte slice", func() {
				It("should return err", func() {
					_, err := NewRepositoryFromBytes([]byte{1, 2, 3})
					Expect(err).ToNot(BeNil())
				})
			})
		})
	})

	Describe("BareRepository.IsNil", func() {
		It("should return true when no fields are set", func() {
			r := BareRepository()
			Expect(r.IsNil()).To(BeTrue())
		})

		It("should return false when at least one field is set", func() {
			r := BareRepository()
			r.CreatorAddress = "address"
			Expect(r.IsNil()).To(BeFalse())
		})

		It("should return false when at least one field is set", func() {
			r := BareRepository()
			r.References = map[string]*Reference{"refs/heads/master": &Reference{}}
			Expect(r.IsNil()).To(BeFalse())
		})
	})

	Describe("References", func() {
		Describe(".Get", func() {
			It("should return bare reference when not found", func() {
				refs := References(map[string]*Reference{
					"refs/heads/master": &Reference{Nonce: 10},
				})
				Expect(refs.Get("refs/heads/dev")).To(Equal(BareReference()))
			})

			It("should return ref when found", func() {
				ref := &Reference{Nonce: 10}
				refs := References(map[string]*Reference{
					"refs/heads/dev": ref,
				})
				Expect(refs.Get("refs/heads/dev")).To(Equal(ref))
			})
		})

		Describe(".Has", func() {
			When("reference does not exist", func() {
				It("should return false", func() {
					ref := &Reference{Nonce: 10}
					refs := References(map[string]*Reference{"refs/heads/dev": ref})
					Expect(refs.Has("refs/heads/master")).To(BeFalse())
				})
			})

			When("reference exist", func() {
				It("should return true", func() {
					ref := &Reference{Nonce: 10}
					refs := References(map[string]*Reference{"refs/heads/dev": ref})
					Expect(refs.Has("refs/heads/dev")).To(BeTrue())
				})
			})
		})
	})
})
