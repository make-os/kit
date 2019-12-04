package keepers

import (
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/util"
)

const (
	// TagAccount is the prefix for account data
	TagAccount = "a"
	// TagGPGPubKey is the prefix for storing gpg public key data
	TagGPGPubKey = "g"
	// TagAddressGPGPkID is the prefix for indexing address -> gpg pkID mapping
	TagAddressGPGPkID = "ag"
	// TagRepo is the prefix for account data
	TagRepo = "r"
	// TagBlockInfo is the prefix for last block data
	TagBlockInfo = "b"
	// TagNetMaturity is the prefix for account data
	TagNetMaturity = "m"
	// TagHighestDrandRound is the prefix for highest drand round
	TagHighestDrandRound = "dr"
	// TagValidators is the prefix for block validators
	TagValidators = "v"
	// TagTx is the prefix for storing/accessing transactions
	TagTx = "t"
)

// MakeAccountKey creates a key for accessing/store an account
func MakeAccountKey(address string) []byte {
	return storage.MakePrefix([]byte(TagAccount), []byte(address))
}

// MakeGPGPubKeyKey creates a key for storing GPG public key
func MakeGPGPubKeyKey(pkID string) []byte {
	return storage.MakePrefix([]byte(TagGPGPubKey), []byte(pkID))
}

// MakeAddrGPGPkIDIndexKey creates a key for address to gpg pub key index
func MakeAddrGPGPkIDIndexKey(address, pkID string) []byte {
	return storage.MakePrefix([]byte(TagAddressGPGPkID), []byte(address), []byte(pkID))
}

// MakeQueryPkIDs creates a key for querying public key ids belonging
// to an address
func MakeQueryPkIDs(address string) []byte {
	return storage.MakePrefix([]byte(TagAddressGPGPkID), []byte(address))
}

// MakeRepoKey creates a key for accessing/store a repository object
func MakeRepoKey(name string) []byte {
	return storage.MakePrefix([]byte(TagRepo), []byte(name))
}

// MakeKeyBlockInfo creates a key for accessing/storing committed block data.
func MakeKeyBlockInfo(height int64) []byte {
	return storage.MakeKey(util.EncodeNumber(uint64(height)), []byte(TagBlockInfo))
}

// MakeQueryKeyBlockInfo creates a key for querying committed block data
func MakeQueryKeyBlockInfo() []byte {
	return storage.MakePrefix([]byte(TagBlockInfo))
}

// MakeNetMaturityKey creates a key indicating the network's maturity status
func MakeNetMaturityKey() []byte {
	return storage.MakePrefix([]byte(TagNetMaturity))
}

// MakeHighestDrandRoundKey creates a key for storing the highest know drand round
func MakeHighestDrandRoundKey() []byte {
	return storage.MakePrefix([]byte(TagHighestDrandRound))
}

// MakeBlockValidatorsKey creates a key for storing validators of blocks
func MakeBlockValidatorsKey(height int64) []byte {
	return storage.MakeKey(util.EncodeNumber(uint64(height)), []byte(TagValidators))
}

// MakeQueryKeyBlockValidators creates a key for querying all block validators
func MakeQueryKeyBlockValidators() []byte {
	return storage.MakePrefix([]byte(TagValidators))
}

// MakeTxKey creates a key for storing validators of blocks
func MakeTxKey(hash []byte) []byte {
	return storage.MakePrefix([]byte(TagTx), hash)
}
