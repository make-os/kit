package crypto

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/util"
)

var _ = Describe("PublicKey", func() {
	Describe(".Bytes", func() {
		It("should return 32 bytes", func() {
			pk := PublicKey{}
			copy(pk[:], util.RandBytes(32))
			Expect(pk.Bytes()).To(HaveLen(32))
		})

		It("should return empty slice when empty", func() {
			pk := PublicKey{}
			Expect(pk.Bytes()).To(HaveLen(0))
		})
	})

	Describe(".ToBytes32", func() {
		It("should convert to Bytes32 type", func() {
			pk := PublicKey{}
			copy(pk[:], util.RandBytes(32))
			b32 := pk.ToBytes32()
			Expect(b32).To(BeAssignableToTypeOf(util.Bytes32{}))
		})
	})

	Describe(".Equal", func() {
		It("should return true when equal and false when not", func() {
			src := util.RandBytes(32)
			pk := PublicKey{}
			copy(pk[:], src)

			pk2 := PublicKey{}
			copy(pk2[:], src)
			Expect(pk.Equal(pk2)).To(BeTrue())

			pk2 = PublicKey{}
			copy(pk2[:], util.RandBytes(32))
			Expect(pk.Equal(pk2)).To(BeFalse())
		})
	})

	Describe(".String", func() {
		It("should return hex encoding in string format", func() {
			src := util.RandBytes(32)
			pk := PublicKey{}
			copy(pk[:], src)
			hex := pk.String()
			Expect(hex[:2]).To(Equal("0x"))

			hexDec, err := util.FromHex(hex)
			Expect(err).To(BeNil())
			Expect(hexDec).To(Equal(src))
		})
	})

	Describe(".Hex", func() {
		It("should return hex encoding in raw bytes", func() {
			src := util.RandBytes(32)
			pk := PublicKey{}
			copy(pk[:], src)
			hex := pk.Hex()
			Expect(hex).To(HaveLen(64))
		})
	})

	Describe(".IsEmpty", func() {
		It("should return true when empty and false when not", func() {
			pk := PublicKey{}
			Expect(pk.IsEmpty()).To(BeTrue())
			src := util.RandBytes(32)
			copy(pk[:], src)
			Expect(pk.IsEmpty()).To(BeFalse())
		})
	})

	Describe(".BytesToPublicKey", func() {
		It("should convert bytes slice to PublicKey", func() {
			src := util.RandBytes(32)
			pk := BytesToPublicKey(src)
			Expect(pk.Bytes()).To(Equal(src))
		})
	})

	Describe(".StrToPublicKey", func() {
		It("should convert bytes slice to PublicKey", func() {
			src := util.RandString(10)
			pk := StrToPublicKey(src)
			Expect(pk.Bytes()).To(HaveLen(32))
		})
	})
})
