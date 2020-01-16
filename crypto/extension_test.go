package crypto

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WrappedPV", func() {
	Describe(".GetKey", func() {
		It("should return expected key and no error", func() {
			wpv := GenerateWrappedPV([]byte("abc"))
			key, err := wpv.GetKey()
			Expect(err).To(BeNil())
			Expect(key.Addr().String()).To(Equal("eCDUbWW9prPkFL1aMTvJSAmYicpxHUkB21"))
		})
	})
})
