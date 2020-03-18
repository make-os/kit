package util

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Address", func() {
	var bech32Addr = "maker1xshwn62y6074qdvqyldkevdqgnht9lh4jvs4lc"

	Describe(".GetPrefixedAddressValue", func() {
		It("should return resource unique name without prefix", func() {
			Expect(GetPrefixedAddressValue("r/repo")).To(Equal("repo"))
		})
	})

	Describe(".IsPrefixedAddr", func() {
		It("should return false if string is not a prefixed address", func() {
			Expect(IsPrefixedAddr("abcdef")).To(BeFalse())
			Expect(IsPrefixedAddr("r/xyz")).To(BeTrue())
		})
	})

	Describe(".IsPrefixedAddressRepo", func() {
		It("should return false when not a repo address", func() {
			Expect(IsPrefixedAddressRepo("abcdef")).To(BeFalse())
		})
		It("should return true when address is a repo address", func() {
			Expect(IsPrefixedAddressRepo("r/repo-name")).To(BeTrue())
		})
	})

	Describe(".IsPrefixedAddressBalanceAccount", func() {
		It("should return false when not a prefixed user keystore address", func() {
			Expect(IsPrefixedAddressUserAccount("abcdef")).To(BeFalse())
		})
		It("should return false when address has the correct prefix but invalid address", func() {
			Expect(IsPrefixedAddressUserAccount("a/invalid")).To(BeFalse())
		})
		It("should return true when address is a valid prefixed balance keystore address", func() {
			Expect(IsPrefixedAddressUserAccount("a/" + bech32Addr)).To(BeTrue())
		})
	})

	Describe(".IsNamespaceURI", func() {
		It("should return false when address is not a namespaced URI", func() {
			Expect(IsNamespaceURI("abcde")).To(BeFalse())
			Expect(IsNamespaceURI("r/abcde")).To(BeFalse())
			Expect(IsNamespaceURI("a/abcde")).To(BeFalse())
			Expect(IsNamespaceURI("z/abcde")).To(BeFalse())
			Expect(IsNamespaceURI("namespace/abcde")).To(BeTrue())
			Expect(IsNamespaceURI("namespace/")).To(BeTrue())
		})
	})

	Describe("Address", func() {
		Describe(".String", func() {
			It("should return the string equivalent", func() {
				Expect(Address("ns/abcd").String()).To(Equal("ns/abcd"))
			})
		})

		Describe(".Empty", func() {
			It("should return true when not set and false when set", func() {
				Expect(Address("").Empty()).To(BeTrue())
				Expect(Address("abcdef").Empty()).To(BeFalse())
			})
		})

		Describe(".IsNamespaceURI", func() {
			It("should return true when address is a namespace URI and false when not", func() {
				Expect(Address("ns1/abcdef").IsNamespaceURI()).To(BeTrue())
				Expect(Address("abcdef").IsNamespaceURI()).To(BeFalse())
			})
		})

		Describe(".IsPrefixed", func() {
			It("should return true when address is a prefixed Address and false when not", func() {
				Expect(Address("r/abcdef").IsPrefixed()).To(BeTrue())
				Expect(Address("s/abcdef").IsPrefixed()).To(BeFalse())
				Expect(Address("abcdef").IsPrefixed()).To(BeFalse())
			})
		})

		Describe(".IsPrefixedRepoAddress", func() {
			It("should return true when address is a prefixed repo Address and false when not", func() {
				Expect(Address("r/abcdef").IsPrefixedRepoAddress()).To(BeTrue())
				Expect(Address("a/abcdef").IsPrefixedRepoAddress()).To(BeFalse())
				Expect(Address("abcdef").IsPrefixedRepoAddress()).To(BeFalse())
			})
		})

		Describe(".IsPrefixedUserAddress", func() {
			It("should return true when address is a prefixed user Address and false when not", func() {
				Expect(Address("a/abcdef").IsPrefixedUserAddress()).To(BeTrue())
				Expect(Address("r/abcdef").IsPrefixedUserAddress()).To(BeFalse())
				Expect(Address("abcdef").IsPrefixedUserAddress()).To(BeFalse())
			})
		})

		Describe(".IsBech32MakerAddress", func() {
			It("should return true when address is a prefixed user Address and false when not", func() {
				Expect(Address("r/abcdef").IsBech32MakerAddress()).To(BeFalse())
				Expect(Address(bech32Addr).IsBech32MakerAddress()).To(BeTrue())
			})
		})
	})
})
