package identifier

import (
	"regexp"
	"strings"
)

const (
	NativeNamespaceRepo          = "r/"
	NativeNamespaceUserAddress   = "a/"
	NativeNamespaceRegexp        = "^[ar]{1}/[a-zA-Z0-9_-]{0,}$"           // e.g r/abc-xyz, a/abc-xyz, r/, a/
	NativeNamespaceRepoRegexp    = "^[r]{1}/[a-zA-Z0-9_-]{0,}$"            // e.g r/abc-xyz, r/
	NativeNamespaceAddressRegexp = "^[a]{1}/[a-zA-Z0-9_-]{0,}$"            // e.g a/abc-xyz, a/
	FullNativeNamespaceRegexp    = "^[ar]{1}/[a-zA-Z0-9_-]+$"              // e.g r/abc-xyz or a/abc-xyz
	UserNamespaceRegexp          = "^[a-zA-Z0-9_-]{3,}/[a-zA-Z0-9_-]{0,}$" // e.g namespace/abc-xyz_ (excluding: r/abc-xyz)
	WholeNamespaceRegexp         = "^([a-zA-Z0-9_-]+)/[a-zA-Z0-9_-]+$"     // e.g namespace/abc-xyz_, r/abc-xyz
)

// GetDomain returns the domain part of a whole namespace. URI
//
// Example: r/abc => abc , a/os1 => os1 , ns1/abc => abc
func GetDomain(str string) string {
	return strings.Split(str, "/")[1]
}

// IsWholeNativeRepoURI checks whether the given string is a whole native repo namespace URI.
//
// Example: r/repo
func IsWholeNativeRepoURI(str string) bool {
	if !IsWholeNativeURI(str) {
		return false
	}
	return str[:2] == NativeNamespaceRepo
}

// IsWholeNativeUserAddressURI checks whether the given string is a whole native user address namespace URI.
//
// Example: a/os1abc
func IsWholeNativeUserAddressURI(str string) bool {
	if !IsWholeNativeURI(str) {
		return false
	}
	return str[:2] == NativeNamespaceUserAddress && IsValidUserAddr(str[2:]) == nil
}

// IsWholeNativeURI checks whether the given string is a full native namespace URI.
//
// Example: r/repo, a/os1abc
func IsWholeNativeURI(str string) bool {
	return regexp.MustCompile(FullNativeNamespaceRegexp).MatchString(str)
}

// IsNativeURI checks whether the given string is a whole or partial native namespace URI.
//
// Example: r/repo, a/os1abc, r/, a/
func IsNativeURI(str string) bool {
	return regexp.MustCompile(NativeNamespaceRegexp).MatchString(str)
}

// IsUserAddressURI checks whether the address is a whole or partial user address namespace URI.
//
// Example: a/os1abc, a/
func IsUserAddressURI(str string) bool {
	return regexp.MustCompile(NativeNamespaceAddressRegexp).MatchString(str)
}

// IsNativeRepoURI checks whether the address is a whole or partial native repo namespace URI.
//
// Example: r/repo, r/
func IsNativeRepoURI(str string) bool {
	return regexp.MustCompile(NativeNamespaceRepoRegexp).MatchString(str)
}

// IsUserURI checks whether the given string is a whole or partial user-defined namespace URI.
//
// Example: ns1/domain, ns1/
func IsUserURI(str string) bool {
	return regexp.MustCompile(UserNamespaceRegexp).MatchString(str)
}

// IsWholeURI checks whether the given string is a whole namespace URI.
//
// Example: r/domain, ns1/domain, a/os1abc
func IsWholeURI(str string) bool {
	return regexp.MustCompile(WholeNamespaceRegexp).MatchString(str)
}

// IsNamespace checks whether the given string is a valid namespace.
//
// Example: user-ns/domain, user-ns/,  r/repo, r/,
func IsNamespace(str string) bool {
	return IsUserURI(str) || IsNativeRepoURI(str) || IsUserAddressURI(str)
}

// IsValidNativeWholeNamespaceURI checks whether the given string is a whole native user address or repo namespace URI.
//
// Example: a/os1abc , r/repo
func IsValidNativeWholeNamespaceURI(str string) bool {
	if IsWholeNativeUserAddressURI(str) {
		return IsValidUserAddr(GetDomain(str)) == nil
	}
	return IsWholeNativeRepoURI(str)
}

// IsValidScope checks whether and address can be used as a scope
func IsValidScope(addr string) bool {
	return IsUserURI(addr) || IsWholeNativeRepoURI(addr) || IsValidResourceName(addr) == nil
}
