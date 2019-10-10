package types

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Util", func() {
	Describe(".ErrStaleSecretRound", func() {
		It("should return expected error", func() {
			err := ErrStaleSecretRound(1)
			Expect(err.Error()).To(Equal("index:1, field:secretRound, error:must be greater than the previous round"))
		})
	})

	Describe(".IsStaleSecretRoundErr", func() {
		It("should return true if err is from ErrStaleSecretRound", func() {
			err := ErrStaleSecretRound(1)
			res := IsStaleSecretRoundErr(err)
			Expect(res).To(BeTrue())
		})
	})

	Describe(".ErrEarlySecretRound", func() {
		It("should return expected error", func() {
			err := ErrEarlySecretRound(1)
			Expect(err.Error()).To(Equal("index:1, field:secretRound, error:round was generated too early"))
		})
	})

	Describe(".IsEarlySecretRoundErr", func() {
		It("should return true if err is from ErrStaleSecretRound", func() {
			err := ErrEarlySecretRound(1)
			res := IsEarlySecretRoundErr(err)
			Expect(res).To(BeTrue())
		})
	})

	Describe("HexBytes", func() {
		Describe(".String", func() {
			It("should return expected hex=616263 for byte=[0x61, 0x62, 0x63]", func() {
				bz := []byte("abc")
				hexBz := HexBytes(bz)
				Expect(hexBz.String()).To(Equal("616263"))
			})
		})

		Describe(".HexBytesFromHex", func() {
			It("should return byte=[0x61, 0x62, 0x63] for hex=616263", func() {
				hexBz := HexBytesFromHex("616263")
				Expect(hexBz).To(Equal(HexBytes([]byte{0x61, 0x62, 0x63})))
			})

			It("should panic if hex string is invalid", func() {
				Expect(func() {
					HexBytesFromHex("6&&^63")
				}).To(Panic())
			})
		})
	})
})
