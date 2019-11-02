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
			expectedBz = r.Bytes()
		})

		It("should return bytes", func() {
			Expect(expectedBz).To(Equal([]uint8{
				0x91, 0xac, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x70, 0x75, 0x62, 0x5f, 0x6b, 0x65, 0x79,
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

	Describe(".IsNil", func() {
		It("should return true when no fields are set", func() {
			r := BareRepository()
			Expect(r.IsNil()).To(BeTrue())
		})

		It("should return false when at least one field is set", func() {
			r := BareRepository()
			r.CreatorPubKey = "pk"
			Expect(r.IsNil()).To(BeFalse())
		})
	})

})
