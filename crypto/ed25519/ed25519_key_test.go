package ed25519

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/make-os/kit/pkgs/bech32"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

var _ = Describe("Key", func() {

	Describe(".PrivKeyFromBytes", func() {
		It("should convert bytes to PrivKey successfully", func() {
			b64 := []byte{
				0x2b, 0xb8, 0x0d, 0x53, 0x7b, 0x1d, 0xa3, 0xe3, 0x8b, 0xd3, 0x03, 0x61, 0xaa, 0x85, 0x56, 0x86,
				0xbd, 0xe0, 0xea, 0xcd, 0x71, 0x62, 0xfe, 0xf6, 0xa2, 0x5f, 0xe9, 0x7b, 0xf5, 0x27, 0xa2, 0x5b,
				0x5d, 0x03, 0x6a, 0x85, 0x8c, 0xe8, 0x9f, 0x84, 0x44, 0x91, 0x76, 0x2e, 0xb8, 0x9e, 0x2b, 0xfb,
				0xd5, 0x0a, 0x4a, 0x0a, 0x0d, 0xa6, 0x58, 0xe4, 0xb2, 0x62, 0x8b, 0x25, 0xb1, 0x17, 0xae, 0x09,
			}
			pk, err := PrivKeyFromBytes(b64)
			Expect(err).To(BeNil())
			rawBz, _ := pk.privKey.Raw()
			Expect(b64[:]).To(Equal(rawBz))
		})

	})

	Describe(".PubKeyFromBytes", func() {
		var key = NewKeyFromIntSeed(22)
		var bz = []uint8{
			0x5e, 0xed, 0xb1, 0x26, 0x49, 0x16, 0x15, 0xab, 0x16, 0xda, 0x11, 0xa4, 0x0a, 0x21, 0xff, 0x89,
			0x25, 0x3a, 0x4c, 0x43, 0x46, 0xfc, 0xbb, 0x38, 0x82, 0xa0, 0x61, 0xac, 0xdf, 0xc7, 0xb3, 0x9b,
		}

		It("should convert bytes to PubKey successfully", func() {
			pubKey, err := PubKeyFromBytes(bz)
			Expect(err).To(BeNil())
			Expect(pubKey.Addr()).To(Equal(key.Addr()))
		})
	})

	Describe(".NewKey", func() {
		When("seeds are '1'", func() {
			It("multiple calls should return same private keys", func() {
				seed := int64(1)
				a1, err := NewKey(&seed)
				Expect(err).To(BeNil())
				a2, err := NewKey(&seed)
				Expect(a1).To(Equal(a2))
			})
		})

		When("seeds are nil", func() {
			It("should return random key on each call", func() {
				a1, err := NewKey(nil)
				Expect(err).To(BeNil())
				a2, err := NewKey(nil)
				Expect(err).To(BeNil())
				Expect(a1).NotTo(Equal(a2))
			})
		})

		When("seeds are different", func() {
			It("multiple calls should not return same private keys", func() {
				seed := int64(1)
				a1, err := NewKey(&seed)
				Expect(err).To(BeNil())
				seed = int64(2)
				a2, err := NewKey(&seed)
				Expect(a1).NotTo(Equal(a2))
			})
		})
	})

	Describe(".idFromPublicKey", func() {

		var cases = [][]interface{}{
			{int64(1), "12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNzbm5akpqu"},
			{int64(2), "12D3KooWKRyzVWW6ChFjQjK4miCty85Niy49tpPV95XdKu1BcvMA"},
			{int64(3), "12D3KooWB1b3qZxWJanuhtseF3DmPggHCtG36KZ9ixkqHtdKH9fh"},
		}

		for _, i := range cases {
			var c = i
			It(fmt.Sprintf("should return id for seed=%d as id=%s", c[0], c[1]), func() {
				seed1 := c[0].(int64)
				k1, _ := NewKey(&seed1)
				id, err := idFromPublicKey(k1.PubKey().pubKey)
				Expect(err).To(BeNil())
				Expect(id).To(Equal(c[1]))
			})
		}
	})

	Describe(".PeerID", func() {
		It("should return 12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNzbm5akpqu", func() {
			seed := int64(1)
			a1, err := NewKey(&seed)
			Expect(err).To(BeNil())
			Expect(a1.PeerID()).To(Equal("12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNzbm5akpqu"))
		})
	})

	Describe(".Addr", func() {
		It("should return 'os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8'", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			addr := a.Addr()
			Expect(addr).To(Equal(identifier.Address("os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8")))
		})
	})

	Describe(".PushAddr", func() {
		It("should return 'pk1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7w8nsw'", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			addr := a.PushAddr()
			Expect(addr).To(Equal(identifier.Address("pk1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7w8nsw")))
		})
	})

	Describe("PubKey.Bytes", func() {
		It("should return err.Error('public key is nil')", func() {
			a := PubKey{}
			_, err := a.Bytes()
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("public key is nil"))
		})

		It("should return []byte{111, 21, 129, 112, 155, 183, 177, 239, 3, 13, 33, 13, 177, 142, 59, 11, 161, 199, 118, 251, 166, 93, 140, 218, 173, 5, 65, 81, 66, 209, 137, 248}", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			bs, err := a.PubKey().Bytes()
			Expect(err).To(BeNil())
			expected := []byte{111, 21, 129, 112, 155, 183, 177, 239, 3, 13, 33, 13, 177, 142, 59, 11, 161, 199, 118, 251, 166, 93, 140, 218, 173, 5, 65, 81, 66, 209, 137, 248}
			Expect(bs).To(Equal(expected))
		})
	})

	Describe("PubKey.Base58", func() {
		It("should return 48d9u6L7tWpSVYmTE4zBDChMUasjP5pvoXE7kPw5HbJnXRnZBNC", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			hx := a.PubKey().Base58()
			Expect(hx).To(Equal("48d9u6L7tWpSVYmTE4zBDChMUasjP5pvoXE7kPw5HbJnXRnZBNC"))
		})
	})

	Describe("Priv.Bytes", func() {
		It("should return err.Error('private key is nil')", func() {
			a := PrivKey{}
			_, err := a.Bytes()
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("private key is nil"))
		})

		It(`should return []byte{82, 253, 252, 7, 33, 130, 101, 79, 22, 63, 95, 15, 154, 98, 29, 
			114, 149, 102, 199, 77, 16, 3, 124, 77, 123, 187, 4, 7, 209, 226, 198, 73, 111, 21, 
			129, 112, 155, 183, 177, 239, 3, 13, 33, 13, 177, 142, 59, 11, 161, 199, 118, 
			251, 166, 93, 140, 218, 173, 5, 65, 81, 66, 209, 137, 248}`, func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			bs, err := a.PrivKey().Bytes()
			Expect(err).To(BeNil())
			expected := []byte{82, 253, 252, 7, 33, 130, 101, 79, 22, 63, 95, 15, 154, 98, 29,
				114, 149, 102, 199, 77, 16, 3, 124, 77, 123, 187, 4, 7, 209, 226, 198, 73, 111,
				21, 129, 112, 155, 183, 177, 239, 3, 13, 33, 13, 177, 142, 59, 11, 161, 199,
				118, 251, 166, 93, 140, 218, 173, 5, 65, 81, 66, 209, 137, 248}
			Expect(bs).To(Equal(expected))
		})
	})

	Describe("Priv.Base58", func() {
		It("should return wU7ckbRBWevtkoT9QoET1adGCsABPRtyDx5T9EHZ4paP78EQ1w5sFM2sZg87fm1N2Np586c98GkYwywvtgy9d2gEpWbsbU", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			hx := a.PrivKey().Base58()
			Expect(hx).To(Equal("wU7ckbRBWevtkoT9QoET1adGCsABPRtyDx5T9EHZ4paP78EQ1w5sFM2sZg87fm1N2Np586c98GkYwywvtgy9d2gEpWbsbU"))
		})
	})

	Describe("Priv.Sign", func() {
		It("should sign the word 'hello'", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			sig, err := a.PrivKey().Sign([]byte("hello"))
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())
			Expect(sig).To(Equal([]byte{158, 13, 68, 26, 41, 83, 26, 181, 43, 77, 192, 150, 115,
				117, 175, 47, 207, 26, 118, 217, 101, 179, 49, 206, 126, 203, 37, 152, 3, 68, 75,
				1, 141, 65, 141, 7, 87, 247, 160, 35, 94, 34, 137, 101, 185, 75, 228, 85, 240,
				182, 166, 71, 94, 88, 208, 108, 189, 55, 174, 220, 119, 184, 128, 15}))
		})
	})

	Describe("Priv.Marshal", func() {
		It("should marshal and unmarshal correctly", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())

			bs, err := a.PrivKey().Marshal()
			Expect(err).To(BeNil())
			Expect(bs).ToNot(BeEmpty())

			_, err = crypto.UnmarshalPrivateKey(bs)
			Expect(err).To(BeNil())
		})
	})

	Describe("Priv.BLSKey", func() {
		Specify("that same key seed should return same bls key always", func() {
			k1 := NewKeyFromIntSeed(1)
			k2 := NewKeyFromIntSeed(1)
			bls1 := k1.PrivKey().BLSKey()
			bls2 := k2.PrivKey().BLSKey()
			Expect(bls1.Bytes()).To(Equal(bls2.Bytes()))
		})
	})

	Describe("Priv.VRFKey", func() {
		Specify("that a 64 bytes slice is returned", func() {
			k1 := NewKeyFromIntSeed(1)
			vrfKey := k1.PrivKey().VRFKey()
			Expect(vrfKey).To(HaveLen(64))
		})
	})

	Describe("Pub.Verify", func() {

		It("should return false when signature is incorrect", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			sig, err := a.PrivKey().Sign([]byte("hello"))
			Expect(err).To(BeNil())

			valid, err := a.PubKey().Verify([]byte("hello friend"), sig)
			Expect(err).To(BeNil())
			Expect(valid).To(BeFalse())
		})

		It("should return true when the signature is correct", func() {
			seed := int64(1)
			a, err := NewKey(&seed)
			Expect(err).To(BeNil())
			sig, err := a.PrivKey().Sign([]byte("hello"))
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())
			Expect(sig).To(Equal([]byte{158, 13, 68, 26, 41, 83, 26, 181, 43, 77, 192, 150, 115,
				117, 175, 47, 207, 26, 118, 217, 101, 179, 49, 206, 126, 203, 37, 152, 3, 68, 75,
				1, 141, 65, 141, 7, 87, 247, 160, 35, 94, 34, 137, 101, 185, 75, 228, 85, 240,
				182, 166, 71, 94, 88, 208, 108, 189, 55, 174, 220, 119, 184, 128, 15}))

			valid, err := a.PubKey().Verify([]byte("hello"), sig)
			Expect(err).To(BeNil())
			Expect(valid).To(BeTrue())
		})
	})

	Describe(".IsValidUserAddr", func() {
		It("should return err when address is unset", func() {
			err := IsValidUserAddr("")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("empty address"))
		})

		It("should return checksum error if address could not be decoded", func() {
			err := IsValidUserAddr("hh23887dhhw88su")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("decoding bech32 failed"))
		})

		It("should return err when address could not be bech32 decoded", func() {
			err := IsValidUserAddr("E1juuqo9XEfKhGHSwExMxGry54h4JzoRkr")
			Expect(err).ToNot(BeNil())
		})

		It("should return nil when address is ok", func() {
			err := IsValidUserAddr("os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8")
			Expect(err).To(BeNil())
		})

		It("should return err when address has invalid hrp", func() {
			addr, _ := bech32.ConvertAndEncode("xyz", []byte("address"))
			err := IsValidUserAddr(addr)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid hrp"))
		})

		It("should return err when address raw data is not 20 bytes", func() {
			addr, _ := bech32.ConvertAndEncode(constants.AddrHRP, []byte("address"))
			err := IsValidUserAddr(addr)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid raw address length"))
		})
	})

	Describe(".IsValidPushAddr()", func() {
		It("should return err when address is unset", func() {
			err := IsValidPushAddr("")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("empty address"))
		})

		It("should return checksum error if address could not be decoded", func() {
			err := IsValidPushAddr("hh23887dhhw88su")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("decoding bech32 failed"))
		})

		It("should return err when address could not be bech32 decoded", func() {
			err := IsValidPushAddr("E1juuqo9XEfKhGHSwExMxGry54h4JzoRkr")
			Expect(err).ToNot(BeNil())
		})

		It("should return nil when address is ok", func() {
			err := IsValidPushAddr("pk1yydtesdlq6p5smejz2gpzlmsxyx2um9rd9qqvp")
			Expect(err).To(BeNil())
		})

		It("should return err when address has invalid hrp", func() {
			addr, _ := bech32.ConvertAndEncode("xyz", []byte("address"))
			err := IsValidPushAddr(addr)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid hrp"))
		})

		It("should return err when address raw data is not 20 bytes", func() {
			addr, _ := bech32.ConvertAndEncode(constants.PushAddrHRP, []byte("address"))
			err := IsValidPushAddr(addr)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid raw address length"))
		})
	})

	Describe(".DecodeAddr", func() {
		It("should return err when address is unset", func() {
			_, err := DecodeAddr("")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("empty address"))
		})

		It("should return 20 bytes address", func() {
			key := NewKeyFromIntSeed(1)
			addrBs, err := DecodeAddr("os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8")
			Expect(err).To(BeNil())
			Expect(addrBs).To(HaveLen(20))
			Expect(addrBs[:]).To(Equal(key.PubKey().AddrRaw()))
		})
	})

	Describe(".DecodeAddrOnly", func() {
		It("should return 20 bytes address", func() {
			addrBs, err := DecodeAddrOnly("eDFPdimzRqfFKetEMSmsSLTLHCLSniZQwD")
			Expect(err).To(BeNil())
			Expect(addrBs).To(HaveLen(20))
			Expect(addrBs).To(Equal([20]uint8{
				0x7a, 0x58, 0x28, 0x74, 0x48, 0xab, 0x42, 0x94, 0x98, 0x5b, 0x71, 0x8e, 0x3d, 0x6b, 0xe6, 0xa7,
				0x81, 0x82, 0x4e, 0xfc,
			}))
		})
	})

	Describe(".RIPEMD160ToAddr", func() {
		It("should return expected address", func() {
			addr := "eDFPdimzRqfFKetEMSmsSLTLHCLSniZQwD"
			addrBs, err := DecodeAddrOnly(addr)
			Expect(err).To(BeNil())
			Expect(addrBs).To(HaveLen(20))
			Expect(addrBs).To(Equal([20]uint8{
				0x7a, 0x58, 0x28, 0x74, 0x48, 0xab, 0x42, 0x94, 0x98, 0x5b, 0x71, 0x8e, 0x3d, 0x6b, 0xe6, 0xa7,
				0x81, 0x82, 0x4e, 0xfc,
			}))

			res := RIPEMD160ToAddr(addrBs)
			Expect(res.String()).To(Equal(addr))
		})
	})

	Describe(".IsValidPubKey", func() {
		It("should return error.Error(empty pub key)", func() {
			err := IsValidPubKey("")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("empty pub key"))
		})

		It("should return err.Error(decoding bech32 failed)", func() {
			err := IsValidPubKey("hh23887dhhw88su")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("checksum error"))
		})

		It("should return err.Error(invalid version)", func() {
			err := IsValidPubKey("E1juuqo9XEfKhGHSwExMxGry54h4JzoRkr")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("invalid version"))
		})

		It("should return nil", func() {
			err := IsValidPubKey("48s9G48LD5eo5YMjJWmRjPaoDZJRNTuiscHMov6zDGMEqUg4vbG")
			Expect(err).To(BeNil())
		})
	})

	Describe(".FromBase58PubKey", func() {
		It("should return err.Error(decoding bech32 failed)", func() {
			_, err := PubKeyFromBase58("hh23887dhhw88su")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("checksum error"))
		})

		It("should return err.Error(invalid version)", func() {
			_, err := PubKeyFromBase58("E1juuqo9XEfKhGHSwExMxGry54h4JzoRkr")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("invalid version"))
		})

		It("should return err = nil", func() {
			pk, err := PubKeyFromBase58("48s9G48LD5eo5YMjJWmRjPaoDZJRNTuiscHMov6zDGMEqUg4vbG")
			Expect(err).To(BeNil())
			Expect(pk).ToNot(BeNil())
		})
	})

	Describe(".IsValidPrivKey", func() {
		It("should return error.Error(empty priv key)", func() {
			err := IsValidPrivKey("")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("empty priv key"))
		})

		It("should return checksum error if address could not be decoded", func() {
			err := IsValidPrivKey("hh23887dhhw88su")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("checksum error"))
		})

		It("should return err.Error(invalid version)", func() {
			err := IsValidPrivKey("E1juuqo9XEfKhGHSwExMxGry54h4JzoRkr")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("invalid version"))
		})

		It("should return nil", func() {
			err := IsValidPrivKey("waS1jBBgdyYgpNtTjKbt6MZbDTYweLtzkNxueyyEc6ss33kPG58VcJNmp" +
				"DK82BwuX8LAoqZuBCdaoXbxHPM99k8HFvqueW")
			Expect(err).To(BeNil())
		})
	})

	Describe(".PrivKeyFromBase58", func() {
		It("should return checksum error if address could not be decoded", func() {
			_, err := PrivKeyFromBase58("hh23887dhhw88su")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("checksum error"))
		})

		It("should return err.Error(invalid version)", func() {
			_, err := PrivKeyFromBase58("E1juuqo9XEfKhGHSwExMxGry54h4JzoRkr")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("invalid version"))
		})

		It("should return err = nil", func() {
			pk, err := PrivKeyFromBase58("waS1jBBgdyYgpNtTjKbt6MZbDTYweLtzkNxueyyEc6ss33kPG58VcJNmpDK82BwuX8LAoqZuBCdaoXbxHPM99k8HFvqueW")
			Expect(err).To(BeNil())
			Expect(pk).ToNot(BeNil())
		})
	})

	Describe(".PrivKeyFromTMPrivateKey", func() {
		It("should encode tendermint's private key to PrivKey", func() {
			sk := ed25519.GenPrivKey()
			nativeSk, err := PrivKeyFromTMPrivateKey(sk)
			Expect(err).To(BeNil())
			Expect(nativeSk).ToNot(BeNil())
		})
	})

	Describe(".ConvertBase58PubKeyToTMPubKey", func() {
		It("should decode base58 public key to tendermint's ed25519.PubKey", func() {
			sk := ed25519.GenPrivKey()
			nativeSk, err := PrivKeyFromTMPrivateKey(sk)
			Expect(err).To(BeNil())

			pubBase58 := NewKeyFromPrivKey(nativeSk).PubKey().Base58()
			tmPubKey, err := ConvertBase58PubKeyToTMPubKey(pubBase58)
			Expect(err).To(BeNil())

			Expect(tmPubKey).To(Equal(sk.PubKey()))
		})
	})

	Describe(".ConvertBase58PrivKeyToTMPrivKey", func() {
		It("should decode base58 private key to tendermint's ed25519.PrivKey", func() {
			sk := ed25519.GenPrivKey()
			nativeSk, err := PrivKeyFromTMPrivateKey(sk)
			Expect(err).To(BeNil())

			privKeyBase58 := NewKeyFromPrivKey(nativeSk).PrivKey().Base58()
			tmPrivKey, err := ConvertBase58PrivKeyToTMPrivKey(privKeyBase58)
			Expect(err).To(BeNil())

			Expect(tmPrivKey).To(Equal(sk))
		})
	})

	Describe(".NewKeyFromPrivKey", func() {
		It("should return nil if nil is passed", func() {
			k := NewKeyFromPrivKey(nil)
			Expect(k).To(BeNil())
		})

		It("should wrap PrivKey in Key", func() {
			sk, err := PrivKeyFromBase58("waS1jBBgdyYgpNtTjKbt6MZbDTYweLtzkNxueyyEc6ss33kPG58VcJNmpDK82BwuX8LAoqZuBCdaoXbxHPM99k8HFvqueW")
			Expect(err).To(BeNil())
			k := NewKeyFromPrivKey(sk)
			Expect(k).ToNot(BeNil())
			Expect(k.PrivKey()).To(Equal(sk))
		})
	})
})
