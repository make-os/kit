package identifier

import (
	"fmt"
	"regexp"

	"github.com/btcsuite/btcutil/bech32"
	"gitlab.com/makeos/mosdef/types/constants"
)

const (
	NativeNamespaceRepo          = "r/"
	NativeNamespaceUserAddress   = "a/"
	IdentifierRegexp             = "^[a-zA-Z0-9_-]+$"                      // e.g abc-xyz_
	NativeNamespaceRegexp        = "^[ar]{1}/[a-zA-Z0-9_-]{0,}$"           // e.g r/abc-xyz, a/abc-xyz, r/, a/
	NativeNamespaceRepoRegexp    = "^[r]{1}/[a-zA-Z0-9_-]{0,}$"            // e.g r/abc-xyz, a/abc-xyz, r/, a/
	NativeNamespaceAddressRegexp = "^[a]{1}/[a-zA-Z0-9_-]{0,}$"            // e.g r/abc-xyz, a/abc-xyz, r/, a/
	FullNativeNamespaceRegexp    = "^[ar]{1}/[a-zA-Z0-9_-]+$"              // e.g r/abc-xyz or a/abc-xyz
	UserNamespaceRegexp          = "^[a-zA-Z0-9_-]{3,}/[a-zA-Z0-9_-]{0,}$" // e.g namespace/abc-xyz_ (excluding: r/abc-xyz)
	NamespaceRegexp              = "^([a-zA-Z0-9_-]+)/[a-zA-Z0-9_-]{0,}$"  // e.g namespace/abc-xyz_, r/abc-xyz, r/, namespace/
	FullNamespaceRegexp          = "^([a-zA-Z0-9_-]+)/[a-zA-Z0-9_-]+$"     // e.g namespace/abc-xyz_, r/abc-xyz
	// ScopePathRegexp              = "^(([a-zA-Z0-9_-]{3,})/[a-zA-Z0-9_-]{0,})|(r/[a-zA-Z0-9_-]+)$" // e.g namespace/abc-xyz_ or r/abc-xyz
)

// GetNativeNamespaceTarget returns the target part of a native namespace
func GetNativeNamespaceTarget(str string) string {
	return str[2:]
}

// IsFullNativeNamespaceRepo checks whether the given address is a native namespace for a repo
func IsFullNativeNamespaceRepo(addr string) bool {
	if !IsFullNativeNamespace(addr) {
		return false
	}
	return addr[:2] == NativeNamespaceRepo
}

// IsFullNativeNamespaceUserAddress checks whether the given address is a native namespace for user address
func IsFullNativeNamespaceUserAddress(addr string) bool {
	if !IsFullNativeNamespace(addr) {
		return false
	}
	return addr[:2] == NativeNamespaceUserAddress && IsValidUserAddr(addr[2:]) == nil
}

// IsFullNativeNamespace checks whether the given address is a full native namespace
func IsFullNativeNamespace(addr string) bool {
	return regexp.MustCompile(FullNativeNamespaceRegexp).MatchString(addr)
}

// IsNativeNamespace checks whether the given address is a native namespace
func IsNativeNamespace(addr string) bool {
	return regexp.MustCompile(NativeNamespaceRegexp).MatchString(addr)
}

// IsNativeNamespaceUserAddress checks whether address is a native namespace address for users
func IsNativeNamespaceUserAddress(addr string) bool {
	return regexp.MustCompile(NativeNamespaceAddressRegexp).MatchString(addr)
}

// IsNativeNamespaceRepo checks whether address is a native namespace address for repositories
func IsNativeNamespaceRepo(addr string) bool {
	return regexp.MustCompile(NativeNamespaceRepoRegexp).MatchString(addr)
}

// IsValidScope checks whether and address can be used as a scope
func IsValidScope(addr string) bool {
	return IsUserNamespace(addr) || IsFullNativeNamespaceRepo(addr) || IsValidResourceName(addr) == nil
}

// IsUserNamespace checks whether the given address is a user-defined namespace
func IsUserNamespace(addr string) bool {
	return regexp.MustCompile(UserNamespaceRegexp).MatchString(addr)
}

// IsFullNamespace checks whether the given address is a full namespace path
func IsFullNamespace(addr string) bool {
	return regexp.MustCompile(FullNamespaceRegexp).MatchString(addr)
}

// IsNamespace checks whether the given address is a valid native or user namespace
func IsNamespace(addr string) bool {
	return IsUserNamespace(addr) || IsNativeNamespaceRepo(addr) || IsNativeNamespaceUserAddress(addr)
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

// IsUserNamespace checks whether the address is a user namespace.
func (a Address) IsUserNamespace() bool {
	return IsUserNamespace(string(a))
}

// IsNamespace checks whether the given address is a valid native or user namespace
func (a Address) IsNamespace() bool {
	return IsNamespace(string(a))
}

// IsFullNamespace checks whether the address is a full namespace path.
func (a Address) IsFullNamespace() bool {
	return IsFullNamespace(string(a))
}

// IsFullNativeNamespace checks whether the address is prefixed with a/ or /r which
// indicates a repo and account address respectively
func (a Address) IsFullNativeNamespace() bool {
	return IsFullNativeNamespace(string(a))
}

// IsNativeNamespace checks whether the given address is a native namespace
func (a Address) IsNativeNamespace() bool {
	return IsNativeNamespace(string(a))
}

// IsNativeNamespaceUserAddress checks whether address is a native namespace address for users
func (a Address) IsNativeNamespaceUserAddress() bool {
	return IsNativeNamespaceUserAddress(string(a))
}

// IsNativeNamespaceRepo checks whether address is a native namespace address for repositories
func (a Address) IsNativeNamespaceRepo() bool {
	return IsNativeNamespaceRepo(string(a))
}

// IsFullNativeNamespaceRepo checks if the address is native repo address namespace.
func (a Address) IsNativeRepoAddress() bool {
	if !a.IsFullNativeNamespace() {
		return false
	}
	return string(a)[:2] == NativeNamespaceRepo
}

// IsFullNativeNamespaceUserAddress checks if the address is native user address namespace.
func (a Address) IsNativeUserAddress() bool {
	if !a.IsFullNativeNamespace() {
		return false
	}
	return string(a)[:2] == NativeNamespaceUserAddress
}

// IsUserAddress checks whether the address is a bech32 user address.
func (a Address) IsUserAddress() bool {
	return IsValidUserAddr(string(a)) == nil
}

// IsValidNativeAddress checks whether the address is a valid native namespace address
func (a Address) IsValidNativeAddress() bool {
	return IsValidNativeAddress(string(a))
}

// IsValidUserAddr checks whether a bech32 user address is valid
func IsValidUserAddr(addr string) error {
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

// IsValidNativeAddress checks whether an address is a valid native namespaced address.
func IsValidNativeAddress(addr string) bool {
	if IsFullNativeNamespaceUserAddress(addr) {
		return IsValidUserAddr(GetNativeNamespaceTarget(addr)) == nil
	}
	return IsFullNativeNamespaceRepo(addr)
}
