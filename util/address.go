package util

import (
	"regexp"
)

// Address constants
const (
	addressPrefixRepo                = "r/"
	addressPrefixAddress             = "a/"
	addressPrefixedIdentifierRegexp  = "^[ar]{1}/[a-zA-Z0-9_-]+$"           // e.g r/abc-xyz
	AddressNamespaceDomainNameRegexp = "^[a-zA-Z0-9_-]+$"                   // e.g r/abc-xyz
	addressNamespaceRegexp           = "^[a-zA-Z0-9_-]{2,}/[a-zA-Z0-9_-]+$" // e.g r/abc-xyz
)

// IsPrefixedAddr checks whether the given string matches a prefixed address
func IsPrefixedAddr(addr string) bool {
	return regexp.MustCompile(addressPrefixedIdentifierRegexp).MatchString(addr)
}

func isNamespacedAddr(addr string) bool {
	return regexp.MustCompile(addressNamespaceRegexp).MatchString(addr)
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
	return IsPrefixedAddr(string(*a))
}

// IsPrefixedRepoAddress checks if the address is prefixed by `r/` which is used to
// identity a repo address
func (a *Address) IsPrefixedRepoAddress() bool {
	if !a.IsPrefixed() {
		return false
	}
	return string(*a)[:2] == addressPrefixRepo
}

// IsPrefixedAccountAddress checks if the address is prefixed by `a/` which is used to
// identity an account address
func (a *Address) IsPrefixedAccountAddress() bool {
	if !a.IsPrefixed() {
		return false
	}
	return string(*a)[:2] == addressPrefixAddress
}

// IsBase58Address checks if the address is prefixed by `a/` which is used to
// identity an account address
func (a *Address) IsBase58Address() bool {
	return IsValidAddr(string(*a)) == nil
}
