package util

import (
	"fmt"

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
		It("should return false when not a prefixed user account address", func() {
			Expect(IsPrefixedAddressUserAccount("abcdef")).To(BeFalse())
		})
		It("should return false when address has the correct prefix but invalid address", func() {
			Expect(IsPrefixedAddressUserAccount("a/invalid")).To(BeFalse())
		})
		It("should return true when address is a valid prefixed balance account address", func() {
			Expect(IsPrefixedAddressUserAccount("a/" + bech32Addr)).To(BeTrue())
		})
	})

	Describe(".IsNonDefaultNamespaceURI", func() {
		It("should return false when address is not a non-default namespaced URI", func() {
			Expect(IsNonDefaultNamespaceURI("abcde")).To(BeFalse())
			Expect(IsNonDefaultNamespaceURI("r/abcde")).To(BeFalse())
			Expect(IsNonDefaultNamespaceURI("a/abcde")).To(BeFalse())
			Expect(IsNonDefaultNamespaceURI("z/abcde")).To(BeFalse())
			Expect(IsNonDefaultNamespaceURI("namespace/abcde")).To(BeTrue())
			Expect(IsNonDefaultNamespaceURI("namespace/")).To(BeTrue())
		})
	})

	Describe(".IsNamespaceURI", func() {
		It("should return false when address is a namespaced URI", func() {
			Expect(IsNamespaceURI("abcde")).To(BeFalse())
			Expect(IsNamespaceURI("r/abcde")).To(BeTrue())
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
				Expect(Address("").IsEmpty()).To(BeTrue())
				Expect(Address("abcdef").IsEmpty()).To(BeFalse())
			})
		})

		Describe(".IsNonDefaultNamespaceURI", func() {
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

	Describe("IsValidAddr", func() {
		It("should return if address is unset", func() {
			Expect(IsValidAddr("")).To(Equal(fmt.Errorf("empty address")))
		})

		It("should return if address is not valid", func() {
			err := IsValidAddr("abc")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid bech32 string"))
		})

		It("should return nil when address is valid", func() {
			err := IsValidAddr("push1k75ztyqr2dq7pc3nlpdfzj2ry58sfzm7l803nz")
			Expect(err).ToNot(BeNil())
			err = IsValidAddr("maker1dhlnq5dt488huxs8nyzd7mu20ujw6zddjv3w4w")
			Expect(err).To(BeNil())
		})
	})
})
