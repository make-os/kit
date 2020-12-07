package identifier

import (
	"fmt"

	"github.com/btcsuite/btcutil/bech32"
	"github.com/make-os/kit/types/constants"
)

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

// IsUserURI checks whether the address is a user
func (a Address) IsUserNamespace() bool {
	return IsUserURI(string(a))
}

// IsNamespace checks whether the given address is a valid native or user namespace
func (a Address) IsNamespace() bool {
	return IsNamespace(string(a))
}

// IsWholeURI checks whether the address is a full namespace path.
func (a Address) IsFullNamespace() bool {
	return IsWholeURI(string(a))
}

// IsFullNativeNamespace checks whether the address is prefixed with a/ or /r which
// indicates a repo and account address respectively
func (a Address) IsFullNativeNamespace() bool {
	return IsWholeNativeURI(string(a))
}

// IsNativeNamespace checks whether the given address is a native namespace
func (a Address) IsNativeNamespace() bool {
	return IsNativeURI(string(a))
}

// IsUserAddressURI checks whether the address is a native namespace address for users
func (a Address) IsNativeNamespaceUserAddress() bool {
	return IsUserAddressURI(string(a))
}

// IsNativeRepoURI checks whether the address is a native namespace address for repositories
func (a Address) IsNativeNamespaceRepo() bool {
	return IsNativeRepoURI(string(a))
}

// IsWholeNativeRepoURI checks if the address is native repo address
func (a Address) IsNativeRepoAddress() bool {
	if !a.IsFullNativeNamespace() {
		return false
	}
	return string(a)[:2] == NativeNamespaceRepo
}

// IsWholeNativeUserAddressURI checks if the address is native user address
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
	return IsValidNativeWholeNamespaceURI(string(a))
}

// IsValidUserAddr checks whether a bech32 user address is valid
func IsValidUserAddr(str string) error {
	if str == "" {
		return fmt.Errorf("empty address")
	}

	hrp, _, err := bech32.Decode(str)
	if err != nil {
		return err
	}

	if hrp != constants.AddrHRP {
		return fmt.Errorf("invalid hrp")
	}

	return nil
}
