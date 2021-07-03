package keepers

import (
	"github.com/make-os/kit/storage/common"
	"github.com/make-os/kit/util"
)

const (
	TagAccount                 = "a"
	TagPushKey                 = "g"
	TagAddressPushKeyID        = "ag"
	TagRepo                    = "r"
	TagRepoPropVote            = "rpv"
	TagRepoPropEndIndex        = "rei"
	TagNS                      = "ns"
	TagClosedProp              = "cp"
	TagBlockInfo               = "b"
	TagHelmRepo                = "hr"
	TagValidators              = "v"
	TagTrackedRepo             = "tr"
	TagAnnouncementScheduleKey = "ak"
	TagRepoRefLastSyncHeight   = "rrh"
	TagPoWNonce                = "pwn"
	TagNodeWork                = "nw"
	TagTotalEpochGasReward     = "egr"
	TagDifficulty              = "d"
)

// MakeRepoRefLastSyncHeightKey creates a key for storing a repo's reference last successful synchronized height.
func MakeRepoRefLastSyncHeightKey(repo, reference string) []byte {
	return common.MakePrefix([]byte(TagRepoRefLastSyncHeight), []byte(repo), []byte(reference))
}

// MakeTrackedRepoKey creates a key for accessing a tracked repo.
func MakeTrackedRepoKey(name string) []byte {
	return common.MakePrefix([]byte(TagTrackedRepo), []byte(name))
}

// MakeQueryTrackedRepoKey creates a key for accessing all tracked repo.
func MakeQueryTrackedRepoKey() []byte {
	return common.MakePrefix([]byte(TagTrackedRepo))
}

// MakeAccountKey creates a key for accessing an account
func MakeAccountKey(address string) []byte {
	return common.MakePrefix([]byte(TagAccount), []byte(address))
}

// MakePushKeyKey creates a key for storing push key
func MakePushKeyKey(pushKeyID string) []byte {
	return common.MakePrefix([]byte(TagPushKey), []byte(pushKeyID))
}

// MakeAddrPushKeyIDIndexKey creates a key for address to push key index
func MakeAddrPushKeyIDIndexKey(address, pushKeyID string) []byte {
	return common.MakePrefix([]byte(TagAddressPushKeyID), []byte(address), []byte(pushKeyID))
}

// MakeQueryPushKeyIDsOfAddress creates a key for querying push key ids belonging to an address
func MakeQueryPushKeyIDsOfAddress(address string) []byte {
	return common.MakePrefix([]byte(TagAddressPushKeyID), []byte(address))
}

// MakeRepoKey creates a key for accessing a repository object
func MakeRepoKey(name string) []byte {
	return common.MakePrefix([]byte(TagRepo), []byte(name))
}

// MakeRepoProposalVoteKey creates a key as flag for a repo proposal vote
func MakeRepoProposalVoteKey(repoName, proposalID, voterAddr string) []byte {
	return common.MakePrefix([]byte(TagRepoPropVote), []byte(repoName),
		[]byte(proposalID), []byte(voterAddr))
}

// MakeRepoProposalEndIndexKey creates a key that makes a repo proposal to its
// end height
func MakeRepoProposalEndIndexKey(repoName, proposalID string, endHeight uint64) []byte {
	return common.MakePrefix([]byte(TagRepoPropEndIndex), util.EncodeNumber(endHeight),
		[]byte(repoName), []byte(proposalID))
}

// MakeQueryKeyRepoProposalAtEndHeight creates a key for finding repo proposals
// ending at the given height
func MakeQueryKeyRepoProposalAtEndHeight(endHeight uint64) []byte {
	return common.MakePrefix([]byte(TagRepoPropEndIndex), util.EncodeNumber(endHeight))
}

// MakeClosedProposalKey creates a key for marking a proposal as "closed"
func MakeClosedProposalKey(name, propID string) []byte {
	return common.MakePrefix([]byte(TagClosedProp), []byte(name), []byte(propID))
}

// MakeNamespaceKey creates a key for accessing a namespace
func MakeNamespaceKey(name string) []byte {
	return common.MakePrefix([]byte(TagNS), []byte(name))
}

// MakeKeyBlockInfo creates a key for accessing/storing committed block data.
func MakeKeyBlockInfo(height int64) []byte {
	return common.MakeKey(util.EncodeNumber(uint64(height)), []byte(TagBlockInfo))
}

// MakeKeyHelmRepo creates a key for getting/setting the helm repo
func MakeKeyHelmRepo() []byte {
	return common.MakePrefix([]byte(TagHelmRepo))
}

// MakeQueryKeyBlockInfo creates a key for querying committed block data
func MakeQueryKeyBlockInfo() []byte {
	return common.MakePrefix([]byte(TagBlockInfo))
}

// MakeBlockValidatorsKey creates a key for storing validators of blocks
func MakeBlockValidatorsKey(height int64) []byte {
	return common.MakeKey(util.EncodeNumber(uint64(height)), []byte(TagValidators))
}

// MakeQueryKeyBlockValidators creates a key for querying all block validators
func MakeQueryKeyBlockValidators() []byte {
	return common.MakePrefix([]byte(TagValidators))
}

// MakeAnnounceListKey creates a key for adding DHT key announcement entry
func MakeAnnounceListKey(key []byte, announceTime int64) []byte {
	return common.MakeKey(util.EncodeNumber(uint64(announceTime)), []byte(TagAnnouncementScheduleKey), key)
}

// MakeQueryAnnounceListKey creates a key for accessing all DHT key announcements entries.
func MakeQueryAnnounceListKey() []byte {
	return common.MakePrefix([]byte(TagAnnouncementScheduleKey))
}

func MakePoWEpochKey(epoch int64) []byte {
	return common.MakePrefix([]byte(TagPoWNonce), util.EncodeNumber(uint64(epoch)))
}

// MakeNodeWorkKey creates a key for storing proof of work nonces found by the node
func MakeNodeWorkKey() []byte {
	return common.MakePrefix([]byte(TagNodeWork))
}

// MakeEpochTotalGasRewardKey creates a key for storing total gas reward in an epoch
func MakeEpochTotalGasRewardKey(epoch int64) []byte {
	return common.MakePrefix([]byte(TagTotalEpochGasReward), util.EncodeNumber(uint64(epoch)))
}

// MakeDifficultyKey creates a key for storing difficulty
func MakeDifficultyKey() []byte {
	return common.MakePrefix([]byte(TagDifficulty))
}
