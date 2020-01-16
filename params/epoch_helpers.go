package params

import (
	"math"
)

// IsStartOfEndOfEpochOfHeight checks whether the block at height is the first block
// in the end stage of an epoch that the target block falls in.
// Note: The last 3 blocks of an epoch are the end stage blocks where the epoch
// is prepared to transition to the next.
// Ex: Given a chain: [1]-[2]-[3]-[4]-[5]
// Supposing a epoch is 5 blocks, epoch end stage starts from [3] and ends at
// [5]. So [3] is the beginning of end of the epoch.
func IsStartOfEndOfEpochOfHeight(height int64) bool {
	return GetEndOfEpochOfHeight(height)-int64(NumBlocksToEffectValChange) == height
}

// IsBeforeEndOfEpoch checks whether the block at height is the block next
// to the last block in the end stage of an epoch.
// Note: The last 3 blocks of an epoch are the end stage blocks where the epoch
// is prepared to transition to the next.
// Ex: Given a chain: [1]-[2]-[3]-[4]-[5]
// Supposing a epoch is 5 blocks, epoch end stage starts from [3] and ends at
// [5]. So [4] is the block before of end of the epoch.
func IsBeforeEndOfEpoch(height int64) bool {
	return GetEndOfEpochOfHeight(height)-(int64(NumBlocksToEffectValChange)-1) == height
}

// IsEndOfEpoch checks whether the block at height is the last block of the epoch.
// Note: The last 3 blocks of an epoch are the end stage blocks where the epoch
// is prepared to transition to the next.
// Ex: Given a chain: [1]-[2]-[3]-[4]-[5]
// Supposing a epoch is 5 blocks, epoch end stage starts from [3] and ends at
// [5]. So [5] is the last block of the epoch.
func IsEndOfEpoch(height int64) bool {
	return GetEndOfEpochOfHeight(height) == height
}

// GetEpochOfHeight returns the epoch number where target height falls in
func GetEpochOfHeight(targetHeight int64) int64 {
	return int64(math.Ceil(float64(targetHeight) / float64(NumBlocksPerEpoch)))
}

// GetStartOfEpochOfHeight returns the block height that is the first block in
// the epoch where the target height falls in
func GetStartOfEpochOfHeight(targetHeight int64) int64 {
	epochOfHeight := int64(math.Ceil(float64(targetHeight) / float64(NumBlocksPerEpoch)))
	endOfEpochOfHeight := epochOfHeight * int64(NumBlocksPerEpoch)
	return endOfEpochOfHeight - (int64(NumBlocksPerEpoch) - 1)
}

// GetEndOfEpochOfHeight returns the block height that is the last block in
// the epoch where the target height falls in
func GetEndOfEpochOfHeight(targetHeight int64) int64 {
	epochOfHeight := int64(math.Ceil(float64(targetHeight) / float64(NumBlocksPerEpoch)))
	return epochOfHeight * int64(NumBlocksPerEpoch)
}

// GetSeedHeightInEpochOfHeight returns the block height that contains the seed
// for the epoch where the target height falls in
func GetSeedHeightInEpochOfHeight(targetHeight int64) int64 {
	epochOfHeight := int64(math.Ceil(float64(targetHeight) / float64(NumBlocksPerEpoch)))
	return (epochOfHeight*int64(NumBlocksPerEpoch) - int64(NumBlocksToEffectValChange))
}

// GetEndOfParentEpochOfHeight returns the block height that is the last block in
// the parent epoch of the epoch where the height falls in
func GetEndOfParentEpochOfHeight(targetHeight int64) int64 {
	epochOfHeight := int64(math.Ceil(float64(targetHeight) / float64(NumBlocksPerEpoch)))
	endOfEpochOfHeight := epochOfHeight * int64(NumBlocksPerEpoch)
	return endOfEpochOfHeight - int64(NumBlocksPerEpoch)
}
