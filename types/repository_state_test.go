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
			r.CreatorPubKey = "some_pub_key"
			r.References = map[string]*Reference{
				"refs/heads/master": &Reference{
					Nonce: 20,
				},
			}
			expectedBz = r.Bytes()
		})

		It("should return bytes", func() {
			Expect(expectedBz).To(Equal([]uint8{
				0xac, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x70, 0x75, 0x62, 0x5f, 0x6b, 0x65, 0x79, 0x81, 0xb1, 0x72,
				0x65, 0x66, 0x73, 0x2f, 0x68, 0x65, 0x61, 0x64, 0x73, 0x2f, 0x6d, 0x61, 0x73, 0x74, 0x65, 0x72,
				0x81, 0xa5, 0x6e, 0x6f, 0x6e, 0x63, 0x65, 0xcf, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14,
			}))
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
			r.CreatorPubKey = "pk"
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
	})
})
