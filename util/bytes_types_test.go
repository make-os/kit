package util

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {
	Describe("PublicKey", func() {

		Describe(".Bytes", func() {
			It("should return 32 bytes", func() {
				pk := PublicKey{}
				copy(pk[:], RandBytes(32))
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
				copy(pk[:], RandBytes(32))
				b32 := pk.ToBytes32()
				Expect(b32).To(BeAssignableToTypeOf(Bytes32{}))
			})
		})

		Describe(".Equal", func() {
			It("should return true when equal and false when not", func() {
				src := RandBytes(32)
				pk := PublicKey{}
				copy(pk[:], src)

				pk2 := PublicKey{}
				copy(pk2[:], src)
				Expect(pk.Equal(pk2)).To(BeTrue())

				pk2 = PublicKey{}
				copy(pk2[:], RandBytes(32))
				Expect(pk.Equal(pk2)).To(BeFalse())
			})
		})

		Describe(".String", func() {
			It("should return hex encoding in string format", func() {
				src := RandBytes(32)
				pk := PublicKey{}
				copy(pk[:], src)
				hex := pk.String()
				Expect(hex[:2]).To(Equal("0x"))

				hexDec, err := FromHex(hex)
				Expect(err).To(BeNil())
				Expect(hexDec).To(Equal(src))
			})
		})

		Describe(".Hex", func() {
			It("should return hex encoding in raw bytes", func() {
				src := RandBytes(32)
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
				src := RandBytes(32)
				copy(pk[:], src)
				Expect(pk.IsEmpty()).To(BeFalse())
			})
		})

		Describe(".BytesToPublicKey", func() {
			It("should convert bytes slice to PublicKey", func() {
				src := RandBytes(32)
				pk := BytesToPublicKey(src)
				Expect(pk.Bytes()).To(Equal(src))
			})
		})

		Describe(".StrToPublicKey", func() {
			It("should convert bytes slice to PublicKey", func() {
				src := RandString(10)
				pk := StrToPublicKey(src)
				Expect(pk.Bytes()).To(HaveLen(32))
			})
		})
	})

	Describe("Bytes32", func() {
		Describe(".Bytes", func() {
			It("should return 32 bytes", func() {
				pk := Bytes32{}
				copy(pk[:], RandBytes(32))
				Expect(pk.Bytes()).To(HaveLen(32))
			})

			It("should return empty slice when empty", func() {
				pk := Bytes32{}
				Expect(pk.Bytes()).To(HaveLen(0))
			})
		})

		Describe(".Equal", func() {
			It("should return true when equal and false when not", func() {
				src := RandBytes(32)
				pk := Bytes32{}
				copy(pk[:], src)

				pk2 := Bytes32{}
				copy(pk2[:], src)
				Expect(pk.Equal(pk2)).To(BeTrue())

				pk2 = Bytes32{}
				copy(pk2[:], RandBytes(32))
				Expect(pk.Equal(pk2)).To(BeFalse())
			})
		})

		Describe(".String", func() {
			It("should return hex encoding in string format", func() {
				src := RandBytes(32)
				pk := Bytes32{}
				copy(pk[:], src)
				hex := pk.String()
				Expect(hex[:2]).To(Equal("0x"))

				hexDec, err := FromHex(hex)
				Expect(err).To(BeNil())
				Expect(hexDec).To(Equal(src))
			})
		})

		Describe(".Hex", func() {
			It("should return hex encoding in raw bytes", func() {
				src := RandBytes(32)
				pk := Bytes32{}
				copy(pk[:], src)
				hex := pk.Hex()
				Expect(hex).To(HaveLen(64))
			})
		})

		Describe(".IsEmpty", func() {
			It("should return true when empty and false when not", func() {
				pk := Bytes32{}
				Expect(pk.IsEmpty()).To(BeTrue())
				src := RandBytes(32)
				copy(pk[:], src)
				Expect(pk.IsEmpty()).To(BeFalse())
			})
		})

		Describe(".BytesToBytes32", func() {
			It("should convert bytes slice to Bytes32", func() {
				src := RandBytes(32)
				pk := BytesToBytes32(src)
				Expect(pk.Bytes()).To(Equal(src))
			})
		})

		Describe(".StrToBytes32", func() {
			It("should convert bytes slice to Bytes32", func() {
				src := RandString(10)
				pk := StrToBytes32(src)
				Expect(pk.Bytes()).To(HaveLen(32))
			})
		})
	})

	Describe("Bytes64", func() {
		Describe(".Bytes", func() {
			It("should return 32 bytes", func() {
				pk := Bytes64{}
				copy(pk[:], RandBytes(64))
				Expect(pk.Bytes()).To(HaveLen(64))
			})

			It("should return empty slice when empty", func() {
				pk := Bytes64{}
				Expect(pk.Bytes()).To(HaveLen(0))
			})
		})

		Describe(".Equal", func() {
			It("should return true when equal and false when not", func() {
				src := RandBytes(64)
				pk := Bytes64{}
				copy(pk[:], src)

				pk2 := Bytes64{}
				copy(pk2[:], src)
				Expect(pk.Equal(pk2)).To(BeTrue())

				pk2 = Bytes64{}
				copy(pk2[:], RandBytes(64))
				Expect(pk.Equal(pk2)).To(BeFalse())
			})
		})

		Describe(".String", func() {
			It("should return hex encoding in string format", func() {
				src := RandBytes(64)
				pk := Bytes64{}
				copy(pk[:], src)
				hex := pk.String()
				Expect(hex[:2]).To(Equal("0x"))

				hexDec, err := FromHex(hex)
				Expect(err).To(BeNil())
				Expect(hexDec).To(Equal(src))
			})
		})

		Describe(".Hex", func() {
			It("should return hex encoding in raw bytes", func() {
				src := RandBytes(64)
				pk := Bytes64{}
				copy(pk[:], src)
				hex := pk.Hex()
				Expect(hex).To(HaveLen(128))
			})
		})

		Describe(".IsEmpty", func() {
			It("should return true when empty and false when not", func() {
				pk := Bytes64{}
				Expect(pk.IsEmpty()).To(BeTrue())
				src := RandBytes(64)
				copy(pk[:], src)
				Expect(pk.IsEmpty()).To(BeFalse())
			})
		})

		Describe(".BytesToBytes64", func() {
			It("should convert bytes slice to Bytes64", func() {
				src := RandBytes(64)
				pk := BytesToBytes64(src)
				Expect(pk.Bytes()).To(Equal(src))
			})
		})
	})
})
