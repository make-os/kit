package types

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GPGPubKey", func() {
	var gpgPubKey *GPGPubKey

	Describe(".Bytes", func() {
		It("should return byte slice", func() {
			gpgPubKey = &GPGPubKey{PubKey: "abc"}
			Expect(gpgPubKey.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".NewGPGPubKeyFromBytes", func() {
		It("should deserialize successfully", func() {
			gpgPubKey = &GPGPubKey{PubKey: "abc", Address: "abc"}
			bz := gpgPubKey.Bytes()
			obj, err := NewGPGPubKeyFromBytes(bz)
			Expect(err).To(BeNil())
			Expect(obj).To(Equal(gpgPubKey))
		})
	})
})
