package keepers

import (
	"fmt"

	"github.com/makeos/mosdef/util"
)

const (
	// Separator separates prefixes
	Separator = ":"
	// AccountTag is the prefix for account data
	AccountTag = "a"
	// BlockInfoTag is the prefix for last block data
	BlockInfoTag = "b"
)

// MakeAccountKey creates a key for accessing/store an account
func MakeAccountKey(address string) []byte {
	return []byte(fmt.Sprintf("%s%s%s", AccountTag, Separator, address))
}

// MakeKeyBlockInfo creates a key for accessing/storing committed block data.
func MakeKeyBlockInfo(height int64) []byte {
	return append([]byte(BlockInfoTag+Separator), util.EncodeNumber(uint64(height))...)
}

// MakeQueryKeyBlockInfo creates a key for querying committed block data
func MakeQueryKeyBlockInfo() []byte {
	return []byte(fmt.Sprintf("%s%s", BlockInfoTag, Separator))
}
