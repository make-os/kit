package identifier

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Address", func() {
	var bech32Addr = "os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8"

	Describe(".GetDomain", func() {
		It("should return resource unique name without prefix", func() {
			Expect(GetDomain("r/repo")).To(Equal("repo"))
		})
	})

	Describe(".IsWholeNativeURI", func() {
		It("should return false if string is not a prefixed address", func() {
			Expect(IsWholeNativeURI("abcdef")).To(BeFalse())
			Expect(IsWholeNativeURI("r/xyz")).To(BeTrue())
		})
	})

	Describe(".IsWholeNativeRepoURI", func() {
		It("should return false when not a repo address", func() {
			Expect(IsWholeNativeRepoURI("abcdef")).To(BeFalse())
		})

		It("should return true when address is a repo address", func() {
			Expect(IsWholeNativeRepoURI("r/repo-name")).To(BeTrue())
		})
	})

	Describe(".IsFullNamespaceURI", func() {
		It("should return expected result", func() {
			Expect(IsFullNamespaceURI("r/domain")).To(BeTrue())
			Expect(IsFullNamespaceURI("ns1/domain")).To(BeTrue())
			Expect(IsFullNamespaceURI("ns1/")).To(BeFalse())
			Expect(IsFullNamespaceURI("ns1")).To(BeFalse())
		})
	})

	Describe(".IsPrefixedAddressBalanceAccount", func() {
		It("should return false when not a prefixed user account address", func() {
			Expect(IsWholeNativeUserAddressURI("abcdef")).To(BeFalse())
		})
		It("should return false when address has the correct prefix but invalid address", func() {
			Expect(IsWholeNativeUserAddressURI("a/invalid")).To(BeFalse())
		})
		It("should return true when address is a valid prefixed balance account address", func() {
			Expect(IsWholeNativeUserAddressURI("a/" + bech32Addr)).To(BeTrue())
		})
	})

	Describe(".IsUserNamespaceURI", func() {
		It("should return false when address is not a non-default namespaced URI", func() {
			Expect(IsUserNamespaceURI("abcde")).To(BeFalse())
			Expect(IsUserNamespaceURI("r/abcde")).To(BeFalse())
			Expect(IsUserNamespaceURI("a/abcde")).To(BeFalse())
			Expect(IsUserNamespaceURI("z/abcde")).To(BeFalse())
			Expect(IsUserNamespaceURI("namespace/abcde")).To(BeTrue())
			Expect(IsUserNamespaceURI("namespace/")).To(BeTrue())
		})
	})

	Describe(".IsNamespaceURI", func() {
		It("should return false when address is a namespaced URI", func() {
			Expect(IsNamespaceURI("abcde")).To(BeFalse())
			Expect(IsNamespaceURI("r/abcde")).To(BeTrue())
			Expect(IsNamespaceURI("a/abcde")).To(BeTrue())
			Expect(IsNamespaceURI("z/abcde")).To(BeFalse())
			Expect(IsNamespaceURI("namespace/abcde")).To(BeTrue())
			Expect(IsNamespaceURI("namespace/")).To(BeTrue())
		})
	})
})
