package state

import (
	"github.com/make-os/lobe/crypto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PushKey", func() {
	var pushPubKey *PushKey

	Describe(".Bytes", func() {
		It("should return byte slice", func() {
			pushPubKey = &PushKey{PubKey: crypto.StrToPublicKey("abc")}
			Expect(pushPubKey.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".NewPushKeyFromBytes", func() {
		It("should deserialize successfully", func() {
			pushPubKey = &PushKey{PubKey: crypto.StrToPublicKey("abc"), Address: "abc"}
			bz := pushPubKey.Bytes()
			obj, err := NewPushKeyFromBytes(bz)
			Expect(err).To(BeNil())
			Expect(obj.Bytes()).To(Equal(pushPubKey.Bytes()))
		})
	})
})
