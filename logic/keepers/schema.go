package keepers

import (
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/util"
)

const (
	tagAccount               = "a"
	tagGPGPubKey             = "g"
	tagAddressGPGPkID        = "ag"
	tagRepo                  = "r"
	tagRepoPropVote          = "rpv"
	tagRepoPropEndIndex      = "rei"
	tagNS                    = "ns"
	tagClosedProp            = "cp"
	tagBlockInfo             = "b"
	tagLastRepoSyncherHeight = "rh"
	tagHelmRepo              = "hr"
	tagNetMaturity           = "m"
	tagValidators            = "v"
	tagTx                    = "t"
)

// MakeAccountKey creates a key for accessing an account
func MakeAccountKey(address string) []byte {
	return storage.MakePrefix([]byte(tagAccount), []byte(address))
}

// MakeGPGPubKeyKey creates a key for storing GPG public key
func MakeGPGPubKeyKey(pkID string) []byte {
	return storage.MakePrefix([]byte(tagGPGPubKey), []byte(pkID))
}

// MakeAddrGPGPkIDIndexKey creates a key for address to gpg pub key index
func MakeAddrGPGPkIDIndexKey(address, pkID string) []byte {
	return storage.MakePrefix([]byte(tagAddressGPGPkID), []byte(address), []byte(pkID))
}

// MakeQueryPkIDs creates a key for querying public key ids belonging
// to an address
func MakeQueryPkIDs(address string) []byte {
	return storage.MakePrefix([]byte(tagAddressGPGPkID), []byte(address))
}

// MakeRepoKey creates a key for accessing a repository object
func MakeRepoKey(name string) []byte {
	return storage.MakePrefix([]byte(tagRepo), []byte(name))
}

// MakeRepoProposalVoteKey creates a key as flag for a repo proposal vote
func MakeRepoProposalVoteKey(repoName, proposalID, voterAddr string) []byte {
	return storage.MakePrefix([]byte(tagRepoPropVote), []byte(repoName),
		[]byte(proposalID), []byte(voterAddr))
}

// MakeRepoProposalEndIndexKey creates a key that makes a repo proposal to its
// end height
func MakeRepoProposalEndIndexKey(repoName, proposalID string, endHeight uint64) []byte {
	return storage.MakePrefix([]byte(tagRepoPropEndIndex), util.EncodeNumber(endHeight),
		[]byte(repoName), []byte(proposalID))
}

// MakeQueryKeyRepoProposalAtEndHeight creates a key for finding repo proposals
// ending at the given height
func MakeQueryKeyRepoProposalAtEndHeight(endHeight uint64) []byte {
	return storage.MakePrefix([]byte(tagRepoPropEndIndex), util.EncodeNumber(endHeight))
}

// MakeClosedProposalKey creates a key for marking a proposal as "closed"
func MakeClosedProposalKey(name, propID string) []byte {
	return storage.MakePrefix([]byte(tagClosedProp), []byte(name), []byte(propID))
}

// MakeNamespaceKey creates a key for accessing a namespace
func MakeNamespaceKey(name string) []byte {
	return storage.MakePrefix([]byte(tagNS), []byte(name))
}

// MakeKeyBlockInfo creates a key for accessing/storing committed block data.
func MakeKeyBlockInfo(height int64) []byte {
	return storage.MakeKey(util.EncodeNumber(uint64(height)), []byte(tagBlockInfo))
}

// MakeKeyRepoSyncherHeight creates a key for accessing last height synch-ed by
// the repo syncher
func MakeKeyRepoSyncherHeight() []byte {
	return storage.MakePrefix([]byte(tagLastRepoSyncherHeight))
}

// MakeKeyHelmRepo creates a key for getting/setting the helm repo
func MakeKeyHelmRepo() []byte {
	return storage.MakePrefix([]byte(tagHelmRepo))
}

// MakeQueryKeyBlockInfo creates a key for querying committed block data
func MakeQueryKeyBlockInfo() []byte {
	return storage.MakePrefix([]byte(tagBlockInfo))
}

// MakeNetMaturityKey creates a key indicating the network's maturity status
func MakeNetMaturityKey() []byte {
	return storage.MakePrefix([]byte(tagNetMaturity))
}

// MakeBlockValidatorsKey creates a key for storing validators of blocks
func MakeBlockValidatorsKey(height int64) []byte {
	return storage.MakeKey(util.EncodeNumber(uint64(height)), []byte(tagValidators))
}

// MakeQueryKeyBlockValidators creates a key for querying all block validators
func MakeQueryKeyBlockValidators() []byte {
	return storage.MakePrefix([]byte(tagValidators))
}

// MakeTxKey creates a key for storing validators of blocks
func MakeTxKey(hash []byte) []byte {
	return storage.MakePrefix([]byte(tagTx), hash)
}
