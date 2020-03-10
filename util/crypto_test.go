package util

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tendermint/tendermint/libs/bech32"
)

var _ = Describe("Crypto", func() {

	Describe(".Encrypt", func() {
		It("should return err='crypto/aes: invalid key size 12' when key size is less than 32 bytes", func() {
			msg := []byte("hello")
			key := []byte("not-32-bytes")
			_, err := Encrypt(msg, key)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("crypto/aes: invalid key size 12"))
		})

		It("should successfully encrypt", func() {
			msg := []byte("hello")
			key := []byte("abcdefghijklmnopqrstuvwxyzabcdef")
			enc, err := Encrypt(msg, key)
			Expect(err).To(BeNil())
			Expect(enc).ToNot(BeNil())
		})
	})

	Describe(".Unlock", func() {
		It("should successfully decrypt", func() {
			msg := []byte("hello")
			key := []byte("abcdefghijklmnopqrstuvwxyzabcdef")
			enc, err := Encrypt(msg, key)
			Expect(err).To(BeNil())
			Expect(enc).ToNot(BeNil())

			dec, err := Decrypt(enc, key)
			Expect(err).To(BeNil())
			Expect(dec).To(Equal(msg))
		})
	})

	Describe(".HexToBytes32", func() {
		It("", func() {
			hash := StrToBytes32("something")
			hex := hash.HexStr()
			result, err := HexToBytes32(hex)
			Expect(err).To(BeNil())
			Expect(result.Equal(hash)).To(BeTrue())
		})
	})

	Describe(".Blake2b256", func() {
		It("should compute expected hash", func() {
			var bs = []byte("hello")
			var expected = []byte{50, 77, 207, 2, 125, 212, 163, 10, 147, 44, 68, 31, 54, 90, 37, 232, 107, 23, 61, 239, 164, 184, 229, 137, 72, 37, 52, 113, 184, 27, 114, 207}
			Expect(Blake2b256(bs)).To(Equal(expected))
		})
	})

	Describe(".RIPEMD160", func() {
		It("should return 20 bytes output", func() {
			var bz = []byte("hello")
			out := RIPEMD160(bz)
			Expect(out).To(HaveLen(20))
		})
	})

	Describe(".CreateGPGIDFromRSA", func() {
		It("should return a 42 character string", func() {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).To(BeNil())
			out := CreateGPGIDFromRSA(key.Public().(*rsa.PublicKey))
			Expect(len(out)).To(Equal(42))
			Expect(out[:3]).To(Equal(GPGAddrHRP))
		})
	})

	Describe("#Bytes32", func() {

		var hash Bytes32
		var bs []byte

		BeforeEach(func() {
			bs = []byte{136, 225, 82, 38, 62, 228, 83, 58, 208, 206, 112, 72, 56, 67, 33, 237, 116, 123, 76, 149, 110, 48, 200, 21, 66, 213, 60, 114, 21, 246, 127, 211}
			hash = BytesToBytes32(bs)
		})

		Describe(".Bytes", func() {
			It("should return expected bytes", func() {
				Expect(hash.Bytes()).To(Equal(bs))
			})
		})

		Describe(".Big", func() {
			It("should return expected big.Int value", func() {
				res := hash.Big()
				Expect(res.Int64()).To(Equal(int64(4815821837235027923)))
			})
		})

		Describe(".Equal", func() {
			It("should return true when equal", func() {
				Expect(hash.Equal(hash)).To(BeTrue())
			})

			It("should return false when not equal", func() {
				hash2 := BytesToBytes32([]byte{23, 45})
				Expect(hash.Equal(hash2)).To(BeFalse())
			})
		})

		Describe(".HexStr", func() {
			It("should return expected hex string prefixed with '0x'", func() {
				str := hash.HexStr()
				Expect(str).To(Equal("0x88e152263ee4533ad0ce7048384321ed747b4c956e30c81542d53c7215f67fd3"))
				Expect(str[0:2]).To(Equal("0x"))
			})
		})

		Describe(".Hex", func() {
			It("should return expected byte slice", func() {
				hexBs := hash.Hex()
				expected := make([]byte, hex.EncodedLen(len(hash)))
				hex.Encode(expected, hash[:])
				Expect(hexBs).To(Equal(expected))
			})
		})

		Describe(".IsEmpty", func() {
			It("should return true if empty", func() {
				hash := BytesToBytes32([]byte{})
				Expect(hash.IsEmpty()).To(BeTrue())
			})
		})
	})

	Describe(".Hash20", func() {
		It("should return 20 bytes", func() {
			res := Hash20([]byte("data"))
			Expect(res).To(HaveLen(20))
		})
	})

	Describe(".CreateGPGIDFromRSA", func() {
		It("should return gpg id", func() {
			sk, err := rsa.GenerateKey(rand.Reader, 1024)
			Expect(err).To(BeNil())
			id := CreateGPGIDFromRSA(&sk.PublicKey)
			Expect(id).To(HaveLen(42))
		})
	})

	Describe(".MustDecodeGPGIDToRSAHash", func() {
		It("should return a 20 bytes slice when successful", func() {
			bz := MustDecodeGPGIDToRSAHash("gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			Expect(bz).To(HaveLen(20))
		})

		It("should panic when not successful", func() {
			Expect(func() { MustDecodeGPGIDToRSAHash("ql277nsqpczpfd") }).To(Panic())
		})
	})

	Describe(".IsValidGPGID", func() {
		It("should return false id could not be decoded", func() {
			// id := bech32.ConvertAndEncode("abc", []byte("abc"))
			id := "bad_id"
			Expect(IsValidGPGID(id)).To(BeFalse())
		})

		It("should return false id has wrong hrp", func() {
			id, _ := bech32.ConvertAndEncode("abc", []byte("abc"))
			Expect(IsValidGPGID(id)).To(BeFalse())
		})

		It("should return false id actual data is not 20-bytes", func() {
			id, _ := bech32.ConvertAndEncode(GPGAddrHRP, []byte("abc"))
			Expect(IsValidGPGID(id)).To(BeFalse())
		})
	})

	Describe(".Hash20Hex", func() {
		It("should return 40 characters", func() {
			Expect(Hash20Hex([]byte("xyz"))).To(HaveLen(40))
		})
	})

	Describe(".HashNamespace", func() {
		It("should produce a 40 byte string", func() {
			Expect(HashNamespace("name1")).To(HaveLen(40))
		})
	})

	Describe(".MustCreateGPGID", func() {
		It("should create a 42 bytes ID from a 20 bytes input", func() {
			bz := []uint8{
				0x6c, 0x73, 0x45, 0x6f, 0x66, 0x4f, 0x73, 0x75, 0x67, 0x42,
				0x57, 0x68, 0x47, 0x6e, 0x41, 0x72, 0x73, 0x75, 0x45, 0x76,
			}
			id := MustCreateGPGID(bz)
			Expect(id).To(Equal("gpg1d3e52mmxfaeh2e6z2a5ywmjpwfeh23tkyp89t4"))
			Expect(len(id)).To(Equal(42))
		})
	})
})
