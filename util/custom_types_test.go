package util

import (
	"github.com/make-os/lobe/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {

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

	Describe("BlockNonce", func() {
		Describe(".EncodeNonce", func() {
			It("should encode to BlockNonce", func() {
				bn := EncodeNonce(1000)
				Expect(bn).To(BeAssignableToTypeOf(BlockNonce{}))
			})
		})

		Describe(".Uint64", func() {
			It("should return uint64 value", func() {
				bn := EncodeNonce(1000)
				Expect(bn.Uint64()).To(Equal(uint64(1000)))
			})
		})
	})

	Describe("String", func() {
		Describe(".Address", func() {
			It("should return Address type", func() {
				Expect(String("addr1").Address()).To(Equal(identifier.Address("addr1")))
			})
		})

		Describe(".Empty", func() {
			It("should return true when empty and false when not", func() {
				Expect(String("").Empty()).To(BeTrue())
				Expect(String("xyz").Empty()).To(BeFalse())
			})
		})

		Describe(".Bytes", func() {
			It("should return expected bytes value", func() {
				s := String("abc")
				Expect(s.Bytes()).To(Equal([]uint8{0x61, 0x62, 0x63}))
			})
		})

		Describe(".Equal", func() {
			It("should equal b", func() {
				a := String("abc")
				b := String("abc")
				Expect(a.Equal(b)).To(BeTrue())
			})

			It("should not equal b", func() {
				a := String("abc")
				b := String("xyz")
				Expect(a.Equal(b)).ToNot(BeTrue())
			})
		})

		Describe(".SS", func() {
			Context("when string is greater than 32 characters", func() {
				It("should return short form", func() {
					s := String("abcdefghijklmnopqrstuvwxyz12345678")
					Expect(s.SS()).To(Equal("abcdefghij...yz12345678"))
				})
			})

			Context("when string is less than 32 characters", func() {
				It("should return unchanged", func() {
					s := String("abcdef")
					Expect(s.SS()).To(Equal("abcdef"))
				})
			})
		})

		Describe(".Decimal", func() {
			It("should return decimal", func() {
				d := String("12.50").Decimal()
				Expect(d.String()).To(Equal("12.5"))
			})

			It("should panic if string is not convertible to decimal", func() {
				Expect(func() {
					String("12a50").Decimal()
				}).To(Panic())
			})
		})

		Describe(".IsDecimal", func() {
			It("should return true if convertible to decimal", func() {
				actual := String("12.50").IsDecimal()
				Expect(actual).To(BeTrue())
			})

			It("should return false if not convertible to decimal", func() {
				actual := String("12a50").IsDecimal()
				Expect(actual).To(BeFalse())
			})
		})

		Describe(".Float", func() {
			It("should panic if unable to convert to float64", func() {
				Expect(func() {
					String("1.0a").Float()
				}).To(Panic())
			})

			It("should return float64 if string is numeric", func() {
				Expect(String("1.3").Float()).To(Equal(1.3))
			})
		})

		Describe(".IsDecimal", func() {
			It("should return true if string contains integer", func() {
				Expect(String("23").IsDecimal()).To(BeTrue())
			})
			It("should return true if string contains float", func() {
				Expect(String("23.726").IsDecimal()).To(BeTrue())
			})
			It("should return false if string is not numerical", func() {
				Expect(String("23a").IsDecimal()).To(BeFalse())
			})
		})
	})

})
