package identifier

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Address", func() {
	var bech32Addr = "maker1xshwn62y6074qdvqyldkevdqgnht9lh4jvs4lc"

	Describe(".GetNativeNamespaceTarget", func() {
		It("should return resource unique name without prefix", func() {
			Expect(GetNativeNamespaceTarget("r/repo")).To(Equal("repo"))
		})
	})

	Describe(".IsFullNativeNamespace", func() {
		It("should return false if string is not a prefixed address", func() {
			Expect(IsFullNativeNamespace("abcdef")).To(BeFalse())
			Expect(IsFullNativeNamespace("r/xyz")).To(BeTrue())
		})
	})

	Describe(".IsFullNativeNamespaceRepo", func() {
		It("should return false when not a repo address", func() {
			Expect(IsFullNativeNamespaceRepo("abcdef")).To(BeFalse())
		})
		It("should return true when address is a repo address", func() {
			Expect(IsFullNativeNamespaceRepo("r/repo-name")).To(BeTrue())
		})
	})

	Describe(".IsPrefixedAddressBalanceAccount", func() {
		It("should return false when not a prefixed user account address", func() {
			Expect(IsFullNativeNamespaceUserAddress("abcdef")).To(BeFalse())
		})
		It("should return false when address has the correct prefix but invalid address", func() {
			Expect(IsFullNativeNamespaceUserAddress("a/invalid")).To(BeFalse())
		})
		It("should return true when address is a valid prefixed balance account address", func() {
			Expect(IsFullNativeNamespaceUserAddress("a/" + bech32Addr)).To(BeTrue())
		})
	})

	Describe(".IsUserNamespace", func() {
		It("should return false when address is not a non-default namespaced URI", func() {
			Expect(IsUserNamespace("abcde")).To(BeFalse())
			Expect(IsUserNamespace("r/abcde")).To(BeFalse())
			Expect(IsUserNamespace("a/abcde")).To(BeFalse())
			Expect(IsUserNamespace("z/abcde")).To(BeFalse())
			Expect(IsUserNamespace("namespace/abcde")).To(BeTrue())
			Expect(IsUserNamespace("namespace/")).To(BeTrue())
		})
	})

	Describe(".IsNamespace", func() {
		It("should return false when address is a namespaced URI", func() {
			Expect(IsNamespace("abcde")).To(BeFalse())
			Expect(IsNamespace("r/abcde")).To(BeTrue())
			Expect(IsNamespace("a/abcde")).To(BeTrue())
			Expect(IsNamespace("z/abcde")).To(BeFalse())
			Expect(IsNamespace("namespace/abcde")).To(BeTrue())
			Expect(IsNamespace("namespace/")).To(BeTrue())
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

		Describe(".IsUserNamespace", func() {
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
			err := IsValidUserAddr("push1k75ztyqr2dq7pc3nlpdfzj2ry58sfzm7l803nz")
			Expect(err).ToNot(BeNil())
			err = IsValidUserAddr("maker1dhlnq5dt488huxs8nyzd7mu20ujw6zddjv3w4w")
			Expect(err).To(BeNil())
		})
	})
})
