package dht

import (
	"bytes"
	"testing"

	"github.com/make-os/lobe/remote/plumbing"
	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
)

func TestDHT(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DHT Suite")
}

var _ = Describe("Common", func() {
	Describe(".MakeWantMsg", func() {
		It("should return expected format", func() {
			msg := MakeWantMsg("repo1", plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521"))
			bytes.HasPrefix(msg, []byte(MsgTypeWant))
			parts := bytes.SplitN(msg, []byte(" "), 3)
			Expect(parts).To(HaveLen(3))
			Expect(parts[1]).To(Equal([]byte("repo1")))
		})
	})

	Describe(".MakeNopeMsg", func() {
		It("should make expected message", func() {
			Expect(MakeNopeMsg()).To(Equal([]byte("NOPE")))
		})
	})

	Describe(".MakeHaveMsg()", func() {
		It("should make expected message", func() {
			Expect(MakeHaveMsg()).To(Equal([]byte("HAVE")))
		})
	})

	Describe(".ParseWantOrSendMsg", func() {
		It("should parse want message", func() {
			hashBz := plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521")
			msg := MakeWantMsg("repo1", hashBz)
			typ, repoName, hash, err := ParseWantOrSendMsg(msg)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal("WANT"))
			Expect(repoName).To(Equal("repo1"))
			Expect(hash).To(Equal(hashBz))
		})

		When("hash part is malformed by having an unexpected space", func() {
			It("should parse correctly", func() {
				hashBz := plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521")
				hashBz[15] = ' '
				msg := MakeWantMsg("repo1", hashBz)
				typ, repoName, hash, err := ParseWantOrSendMsg(msg)
				Expect(err).To(BeNil())
				Expect(typ).To(Equal("WANT"))
				Expect(repoName).To(Equal("repo1"))
				Expect(hash[15]).To(Equal(uint8(0x20)))
			})
		})
	})

	Describe(".ReadWantOrSendMsg", func() {
		It("should read WANT message correctly", func() {
			hashBz := plumbing.HashToBytes("d9dbe0e59248c7f0505dd5d80ed470fb43f82521")
			msg := MakeWantMsg("repo1", hashBz)
			typ, repoName, hash, err := ReadWantOrSendMsg(bytes.NewReader(msg))
			Expect(err).To(BeNil())
			Expect(typ).To(Equal("WANT"))
			Expect(repoName).To(Equal("repo1"))
			Expect(hash).To(Equal(hashBz))
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
