package identifier

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Address", func() {
	var bech32Addr = "os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8"

	Describe(".String", func() {
		It("should return the string equivalent", func() {
			Expect(Address("ns/abcd").String()).To(Equal("ns/abcd"))
		})
	})

	Describe(".Empty", func() {
		It("should return true when not set and false when set", func() {
			Expect(Address("").IsEmpty()).To(BeTrue())
			Expect(Address("abcdef").IsEmpty()).To(BeFalse())
		})
	})

	Describe(".IsUserNamespaceURI", func() {
		It("should return true when address is a namespace path and false when not", func() {
			Expect(Address("ns1/abcdef").IsNamespace()).To(BeTrue())
			Expect(Address("abcdef").IsNamespace()).To(BeFalse())
		})
	})

	Describe(".IsFullNativeNamespace", func() {
		It("should return true when address is a prefixed Address and false when not", func() {
			Expect(Address("r/abcdef").IsFullNativeNamespace()).To(BeTrue())
			Expect(Address("s/abcdef").IsFullNativeNamespace()).To(BeFalse())
			Expect(Address("abcdef").IsFullNativeNamespace()).To(BeFalse())
		})
	})

	Describe(".IsFullNativeNamespaceRepo", func() {
		It("should return true when address is a prefixed repo Address and false when not", func() {
			Expect(Address("r/abcdef").IsNativeRepoAddress()).To(BeTrue())
			Expect(Address("a/abcdef").IsNativeRepoAddress()).To(BeFalse())
			Expect(Address("abcdef").IsNativeRepoAddress()).To(BeFalse())
		})
	})

	Describe(".IsFullNativeNamespaceUserAddress", func() {
		It("should return true when address is a prefixed user Address and false when not", func() {
			Expect(Address("a/abcdef").IsNativeUserAddress()).To(BeTrue())
			Expect(Address("r/abcdef").IsNativeUserAddress()).To(BeFalse())
			Expect(Address("abcdef").IsNativeUserAddress()).To(BeFalse())
		})
	})

	Describe(".IsUserAddress", func() {
		It("should return true when address is a prefixed user Address and false when not", func() {
			Expect(Address("r/abcdef").IsUserAddress()).To(BeFalse())
			Expect(Address(bech32Addr).IsUserAddress()).To(BeTrue())
		})
	})

	Describe("IsValidUserAddr", func() {
		It("should return if address is unset", func() {
			Expect(IsValidUserAddr("")).To(Equal(fmt.Errorf("empty address")))
		})

		It("should return if address is not valid", func() {
			err := IsValidUserAddr("abc")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid bech32 string"))
		})

		It("should return nil when address is valid", func() {
			err := IsValidUserAddr("pk1k75ztyqr2dq7pc3nlpdfzj2ry58sfzm7l803nz")
			Expect(err).ToNot(BeNil())
			err = IsValidUserAddr("os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8")
			Expect(err).To(BeNil())
		})
	})
})
