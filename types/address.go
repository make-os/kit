package types

import (
	"github.com/makeos/mosdef/crypto"
	"regexp"
)

// Address constants
const (
	AddressPrefixRepo                = "r/"
	AddressPrefixAddress             = "a/"
	AddressPrefixedIdentifierRegexp  = "^[ar]{1}/[a-zA-Z0-9_-]+$"        // e.g r/abc-xyz
	AddressNamespaceDomainNameRegexp = "^[a-zA-Z0-9_-]+$"                // e.g r/abc-xyz
	AddressNamespaceRegexp           = "^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$" // e.g r/abc-xyz
)

func isPrefixedAddr(addr string) bool {
	return regexp.MustCompile(AddressPrefixedIdentifierRegexp).MatchString(addr)
}

func isNamespacedAddr(addr string) bool {
	return regexp.MustCompile(AddressNamespaceRegexp).MatchString(addr)
}

// Address represents an identifier for a resource
type Address string

func (a *Address) String() string {
	return string(*a)
}

// Empty checks whether the address is empty
func (a *Address) Empty() bool {
	return a.String() == ""
}

// IsNamespaceURI checks whether the address is a namespace URI
func (a *Address) IsNamespaceURI() bool {
	return isNamespacedAddr(string(*a))
}

// IsPrefixed checks whether the address is prefixed with a/ or /r which
// indicates a repo and account address respectively
func (a *Address) IsPrefixed() bool {
	return isPrefixedAddr(string(*a))
}

// IsPrefixedRepoAddress checks if the address is prefixed by `r/` which is used to
// identity a repo address
func (a *Address) IsPrefixedRepoAddress() bool {
	if !a.IsPrefixed() {
		return false
	}
	return string(*a)[:2] == AddressPrefixRepo
}

// IsPrefixedAccountAddress checks if the address is prefixed by `a/` which is used to
// identity an account address
func (a *Address) IsPrefixedAccountAddress() bool {
	if !a.IsPrefixed() {
		return false
	}
	return string(*a)[:2] == AddressPrefixAddress
}

// IsBase58Address checks if the address is prefixed by `a/` which is used to
// identity an account address
func (a *Address) IsBase58Address() bool {
	return crypto.IsValidAddr(string(*a)) == nil
}
