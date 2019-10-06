package keepers

import (
	"fmt"

	"github.com/makeos/mosdef/util"
)

const (
	// Separator separates prefixes
	Separator = ":"
	// TagAccount is the prefix for account data
	TagAccount = "a"
	// TagBlockInfo is the prefix for last block data
	TagBlockInfo = "b"
	// TagNetMaturity is the prefix for account data
	TagNetMaturity = "nm"
	// TagHighestDrandRound is the prefix for highest drand round
	TagHighestDrandRound = "dr"
)

// MakeAccountKey creates a key for accessing/store an account
func MakeAccountKey(address string) []byte {
	return []byte(fmt.Sprintf("%s%s%s", TagAccount, Separator, address))
}

// MakeKeyBlockInfo creates a key for accessing/storing committed block data.
func MakeKeyBlockInfo(height int64) []byte {
	return append([]byte(TagBlockInfo+Separator), util.EncodeNumber(uint64(height))...)
}

// MakeQueryKeyBlockInfo creates a key for querying committed block data
func MakeQueryKeyBlockInfo() []byte {
	return []byte(fmt.Sprintf("%s%s", TagBlockInfo, Separator))
}

// MakeNetMaturityKey creates a key indicating the network's maturity status
func MakeNetMaturityKey() []byte {
	return []byte(fmt.Sprintf("%s", TagNetMaturity))
}

// MakeHighestDrandRoundKey creates a key for storing the highest know drand round
func MakeHighestDrandRoundKey() []byte {
	return []byte(fmt.Sprintf("%s", TagHighestDrandRound))
}
