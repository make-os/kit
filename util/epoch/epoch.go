package epoch

import (
	"fmt"
	"math"

	"github.com/make-os/kit/params"
)

// IsThirdToLastInEpochOfHeight checks whether the block at given height
// is the third to the last block its epoch.
//  - Ex: Given a chain: [1]-[2]-[3]-[4]-[5]
//  Supposing a epoch is 5 blocks, epoch end stage starts from [3] and ends at
//  [5]. So [3] is the third to the last.
func IsThirdToLastInEpochOfHeight(height int64) bool {
	return IsLastInEpochOfHeight(height)-int64(params.NumBlocksToEffectValChange) == height
}

// IsBeforeEndOfEpochOfHeight checks whether the block at the given height is the block next
// to the last block in the end stage of an epoch.
// Note: The last 3 blocks of an epoch are the end stage blocks where the epoch
// is prepared to transition to the next.
// Ex: Given a chain: [1]-[2]-[3]-[4]-[5]
// Supposing a epoch is 5 blocks, epoch end stage starts from [3] and ends at
// [5]. So [4] is the block before of end of the epoch.
func IsBeforeEndOfEpochOfHeight(height int64) bool {
	return IsLastInEpochOfHeight(height)-(int64(params.NumBlocksToEffectValChange)-1) == height
}

// IsEndOfEpochOfHeight checks whether the block at height is the last block of the epoch.
// Note: The last 3 blocks of an epoch are the end stage blocks where the epoch
// is prepared to transition to the next.
// Ex: Given a chain: [1]-[2]-[3]-[4]-[5]
// Supposing a epoch is 5 blocks, epoch end stage starts from [3] and ends at
// [5]. So [5] is the last block of the epoch.
func IsEndOfEpochOfHeight(height int64) bool {
	return IsLastInEpochOfHeight(height) == height
}

// GetEpochAt returns the epoch number where target height falls in
func GetEpochAt(height int64) int64 {
	return int64(math.Ceil(float64(height) / float64(params.NumBlocksPerEpoch)))
}

// GetFirstInEpochOfHeight returns the block height that is the first
// block in the epoch where the target height falls in.
func GetFirstInEpochOfHeight(height int64) int64 {
	epochOfHeight := GetEpochAt(height)
	endOfEpochOfHeight := epochOfHeight * int64(params.NumBlocksPerEpoch)
	return endOfEpochOfHeight - (int64(params.NumBlocksPerEpoch) - 1)
}

// GetFirstInEpoch returns the block height that is the first in a given epoch.
func GetFirstInEpoch(epoch int64) int64 {
	if epoch == 0 {
		panic(fmt.Errorf("invalid epoch"))
	}
	return epoch*int64(params.NumBlocksPerEpoch) - int64(params.NumBlocksPerEpoch) + 1
}

// IsLastInEpochOfHeight returns the height of the last block in the epoch
// where the target height falls in.
func IsLastInEpochOfHeight(height int64) int64 {
	epochOfHeight := GetEpochAt(height)
	return epochOfHeight * int64(params.NumBlocksPerEpoch)
}

// GetSeedInEpochOfHeight returns the block height that contains the seed
// for the epoch where the target height falls in
func GetSeedInEpochOfHeight(height int64) int64 {
	epochOfHeight := GetEpochAt(height)
	return epochOfHeight*int64(params.NumBlocksPerEpoch) - int64(params.NumBlocksToEffectValChange)
}

// GetLastInParentOfEpochOfHeight returns the block height that is the last block in
// the parent epoch of the epoch where the given block height falls in
func GetLastInParentOfEpochOfHeight(height int64) int64 {
	epochOfHeight := GetEpochAt(height)
	endOfEpochOfHeight := epochOfHeight * int64(params.NumBlocksPerEpoch)
	return endOfEpochOfHeight - int64(params.NumBlocksPerEpoch)
}
