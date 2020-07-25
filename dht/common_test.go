package dht

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/lobe/remote/plumbing"

	. "github.com/onsi/gomega"
)

var _ = Describe("Common", func() {
	var hashToKeys = map[string][]byte{
		"d9dbe0e59248c7f0505dd5d80ed470fb43f82521": {0x2f, 0x6f, 0x2f, 0xd9, 0xdb, 0xe0, 0xe5, 0x92, 0x48, 0xc7, 0xf0, 0x50, 0x5d, 0xd5, 0xd8, 0x0e,
			0xd4, 0x70, 0xfb, 0x43, 0xf8, 0x25, 0x21},
	}

	Describe(".MakeObjectKey", func() {
		It("should return expected key", func() {
			for hash, expected := range hashToKeys {
				bz := plumbing.HashToBytes(hash)
				actual := MakeObjectKey(bz)
				Expect(expected).To(Equal(actual))
			}
		})
	})

	Describe(".ParseObjectKey", func() {
		It("should return expected key", func() {
			for hash, key := range hashToKeys {
				expected := plumbing.HashToBytes(hash)
				actual, err := ParseObjectKey(key)
				Expect(err).To(BeNil())
				Expect(bytes.Equal(expected, actual)).To(BeTrue())
			}
		})
	})

	Describe(".MakeWantMsg", func() {
		It("should return expected format", func() {
			msg := MakeWantMsg("repo1", plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521"))
			bytes.HasPrefix(msg, []byte(MsgTypeWant))
			parts := bytes.SplitN(msg, []byte(" "), 3)
			Expect(parts).To(HaveLen(3))
			Expect(parts[1]).To(Equal([]byte("repo1")))
		})
	})

	Describe(".ParseWantOrSendMsg", func() {
		It("should parse want message", func() {
			hashBz := plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521")
			msg := MakeWantMsg("repo1", hashBz)
			repoName, hash, err := ParseWantOrSendMsg(msg)
			Expect(err).To(BeNil())
			Expect(repoName).To(Equal("repo1"))
			Expect(hash).To(Equal(hashBz))
		})

		When("hash part is malformed by having an unexpected space", func() {
			It("should parse correctly", func() {
				hashBz := plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521")
				hashBz[15] = ' '
				msg := MakeWantMsg("repo1", hashBz)
				repoName, hash, err := ParseWantOrSendMsg(msg)
				Expect(err).To(BeNil())
				Expect(repoName).To(Equal("repo1"))
				Expect(hash[15]).To(Equal(uint8(0x20)))
			})
		})
	})

	Describe(".MakeSendMsg", func() {
		It("should return expected format", func() {
			msg := MakeSendMsg("repo1", plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521"))
			bytes.HasPrefix(msg, []byte(MsgTypeWant))
			parts := bytes.SplitN(msg, []byte(" "), 3)
			Expect(parts).To(HaveLen(3))
			Expect(parts[1]).To(Equal([]byte("repo1")))
		})
	})
})
