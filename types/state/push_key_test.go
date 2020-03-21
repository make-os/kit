package state

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/crypto"
)

var _ = Describe("PushKey", func() {
	var gpgPubKey *PushKey

	Describe(".Bytes", func() {
		It("should return byte slice", func() {
			gpgPubKey = &PushKey{PubKey: crypto.StrToPublicKey("abc")}
			Expect(gpgPubKey.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".NewPushKeyFromBytes", func() {
		It("should deserialize successfully", func() {
			gpgPubKey = &PushKey{PubKey: crypto.StrToPublicKey("abc"), Address: "abc"}
			bz := gpgPubKey.Bytes()
			obj, err := NewPushKeyFromBytes(bz)
			Expect(err).To(BeNil())
			Expect(obj).To(Equal(gpgPubKey))
		})
	})
})
