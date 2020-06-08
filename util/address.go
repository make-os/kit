package util

import (
	"fmt"
	"regexp"

	"github.com/btcsuite/btcutil/bech32"
	"gitlab.com/makeos/mosdef/types/constants"
)

// Address constants
const (
	RepoIDPrefix                     = "r/"
	AddressIDPrefix                  = "a/"
	addressPrefixedIdentifierRegexp  = "^[ar]{1}/[a-zA-Z0-9_-]+$"                  // e.g r/abc-xyz or a/abc-xyz
	AddressNamespaceDomainNameRegexp = "^[a-zA-Z0-9_-]+$"                          // e.g abc-xyz_
	addressNonDefaultNamespaceRegexp = "^[a-zA-Z0-9_-]{3,}/[a-zA-Z0-9_-]{0,}$"     // e.g namespace/abc-xyz_ (excluding: r/abc-xyz)
	addressNamespaceRegexp           = "^([a-zA-Z0-9_-]{3,}|r)/[a-zA-Z0-9_-]{0,}$" // e.g namespace/abc-xyz_ or r/abc-xyz
)

// GetPrefixedAddressValue returns the address without the prefix
func GetPrefixedAddressValue(prefixedAddr string) string {
	return prefixedAddr[2:]
}

// IsPrefixedAddressRepo checks whether the given address is a prefixed repo address
func IsPrefixedAddressRepo(addr string) bool {
	if !IsPrefixedAddr(addr) {
		return false
	}
	return addr[:2] == RepoIDPrefix
}

// IsPrefixedAddressUserAccount checks whether the given address is a prefixed repo address
func IsPrefixedAddressUserAccount(addr string) bool {
	if !IsPrefixedAddr(addr) {
		return false
	}
	return addr[:2] == AddressIDPrefix &&
		IsValidAddr(addr[2:]) == nil
}

// IsPrefixedAddr checks whether the given address matches a prefixed address
func IsPrefixedAddr(addr string) bool {
	return regexp.MustCompile(addressPrefixedIdentifierRegexp).MatchString(addr)
}

// IsNonDefaultNamespaceURI checks whether the given address is a non-default namespace URI
func IsNonDefaultNamespaceURI(addr string) bool {
	return regexp.MustCompile(addressNonDefaultNamespaceRegexp).MatchString(addr)
}

// IsNamespaceURI checks whether the given address is a namespace URI (including default or custom namespaces).
func IsNamespaceURI(addr string) bool {
	return regexp.MustCompile(addressNamespaceRegexp).MatchString(addr)
}

// Address represents an identifier for a resource
type Address string

func (a Address) String() string {
	return string(a)
}

// Equals checks whether a is equal to addr
func (a Address) Equal(addr Address) bool {
	return a == addr
}

// Empty checks whether the address is empty
func (a Address) IsEmpty() bool {
	return a.String() == ""
}

// IsNonDefaultNamespaceURI checks whether the address is a namespace URI
func (a Address) IsNamespaceURI() bool {
	return IsNonDefaultNamespaceURI(string(a))
}

// IsPrefixed checks whether the address is prefixed with a/ or /r which
// indicates a repo and account address respectively
func (a Address) IsPrefixed() bool {
	return IsPrefixedAddr(string(a))
}

// IsPrefixedRepoAddress checks if the address is prefixed by `r/` which is used to
// identity a repo address
func (a Address) IsPrefixedRepoAddress() bool {
	if !a.IsPrefixed() {
		return false
	}
	return string(a)[:2] == RepoIDPrefix
}

// IsPrefixedUserAddress checks if the address is prefixed by
// `a/` which is used to identity an account's address
func (a Address) IsPrefixedUserAddress() bool {
	if !a.IsPrefixed() {
		return false
	}
	return string(a)[:2] == AddressIDPrefix
}

// IsBech32MakerAddress checks whether the address is a
// bech32 address with the general HRP
func (a Address) IsBech32MakerAddress() bool {
	return IsValidAddr(string(a)) == nil
}

// IsValidAddr checks whether an address is valid
func IsValidAddr(addr string) error {
	if addr == "" {
		return fmt.Errorf("empty address")
	}

	hrp, _, err := bech32.Decode(addr)
	if err != nil {
		return err
	}

	if hrp != constants.AddrHRP {
		return fmt.Errorf("invalid hrp")
	}

	return nil
}
