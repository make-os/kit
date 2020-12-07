package bech32_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/make-os/kit/pkgs/bech32"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBech32(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bech32 Suite")
}

var _ = Describe("Bech32", func() {
	It("should test encode and decode", func() {
		sum := sha256.Sum256([]byte("hello world\n"))
		ss := "shasum"

		bech, err := bech32.ConvertAndEncode(ss, sum[:])
		Expect(err).To(BeNil())

		hrp, data, err := bech32.DecodeAndConvert(bech)
		Expect(err).To(BeNil())

		Expect(hrp).To(Equal(ss), "Invalid hrp")
		Expect(bytes.Equal(data, sum[:])).To(BeTrue())
	})
})
