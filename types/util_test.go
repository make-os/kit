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
})
