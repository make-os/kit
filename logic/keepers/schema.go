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
	// TagValidators is the prefix for block validators
	TagValidators = "v"
	// TagTx is the prefix for storing/accessing transactions
	TagTx = "t"
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

// MakeBlockValidatorsKey creates a key for storing validators of blocks
func MakeBlockValidatorsKey(height int64) []byte {
	return append([]byte(TagValidators+Separator), util.EncodeNumber(uint64(height))...)
}

// MakeQueryKeyBlockValidators creates a key for querying all block validators
func MakeQueryKeyBlockValidators() []byte {
	return []byte(fmt.Sprintf("%s%s", TagValidators, Separator))
}

// MakeTxKey creates a key for storing validators of blocks
func MakeTxKey(hash []byte) []byte {
	return append([]byte(TagTx+Separator), hash...)
}
