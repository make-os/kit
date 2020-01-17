package logic

import (
	"fmt"
	"math"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto/vrf"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/logic/keepers"
	"github.com/pkg/errors"

	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
)

// ErrNoSeedFound means no secret was found
var ErrNoSeedFound = fmt.Errorf("no secret found")

// System implements types.TxLogic.
// Provides functionalities for executing transactions
type System struct {
	logic types.Logic
}

// GetCurValidatorTicketPrice returns the ticket price.
// Ticket price increases by x percent after every n blocks
func (s *System) GetCurValidatorTicketPrice() float64 {

	bi, err := s.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(err)
	}

	price := decimal.NewFromFloat(params.InitialTicketPrice)
	epoch := math.Ceil(float64(bi.Height) / float64(params.NumBlocksPerPriceWindow))
	for i := 0; i < int(epoch); i++ {
		if i == 0 {
			continue
		}
		inc := price.Mul(decimal.NewFromFloat(params.PricePercentIncrease))
		price = price.Add(inc)
	}

	p, _ := price.Float64()
	return p
}

// CheckSetNetMaturity checks whether the network has reached a matured period.
// If it has not, we return error. However, if it just met the maturity
// condition in this call, we mark the network as mature
func (s *System) CheckSetNetMaturity() error {

	// Check whether the network has already been flagged as matured.
	// If yes, return nil
	isMatured, err := s.logic.SysKeeper().IsMarkedAsMature()
	if err != nil {
		return errors.Wrap(err, "failed to determine network maturity status")
	} else if isMatured {
		return nil
	}

	// Get the most recently committed block
	bi, err := s.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		if err == keepers.ErrBlockInfoNotFound {
			return fmt.Errorf("no committed block yet")
		}
		return err
	}

	// Ensure the network has reached the maturity height
	if bi.Height < params.NetMaturityHeight {
		return fmt.Errorf("network maturity period has not been reached (%d blocks left)",
			params.NetMaturityHeight-bi.Height)
	}

	// Ensure there are enough live tickets
	numLiveTickets, err := s.logic.GetTicketManager().CountActiveValidatorTickets()
	if err != nil {
		return errors.Wrap(err, "failed to count live tickets")
	}

	// Ensure there are currently enough bootstrap tickets
	if numLiveTickets < params.MinBootstrapLiveTickets {
		return fmt.Errorf("insufficient live bootstrap tickets")
	}

	// Set the network maturity flag so that we won't have
	// to perform this entire check next time.
	if err := s.logic.SysKeeper().MarkAsMatured(uint64(bi.Height)); err != nil {
		return errors.Wrap(err, "failed to set network maturity flag")
	}

	return nil
}

// GetEpoch return the epoch in which the given block height belongs in.
// Also, it returns the epoch of the following block height.
// PANICS: if unable to determine network's maturity height.
func (s *System) GetEpoch(curBlockHeight uint64) (int, int) {

	// The first epoch start height refers the the block height where
	// the network gain maturity.
	firstEpochStartHeight, err := s.logic.SysKeeper().GetNetMaturityHeight()
	if err != nil {
		panic(err)
	}

	// Calculate the current epoch
	numBlocksSinceEpochStart := curBlockHeight - firstEpochStartHeight
	curEpoch := float64(numBlocksSinceEpochStart) / float64(params.NumBlocksPerEpoch)

	// Calculate the next epoch
	numBlocksSinceEpochStart = (curBlockHeight + 1) - firstEpochStartHeight
	nextEpoch := float64(numBlocksSinceEpochStart) / float64(params.NumBlocksPerEpoch)

	return int(math.Ceil(curEpoch)), int(math.Ceil(nextEpoch))
}

// GetLastEpochSeed get the seed of the last epoch
func (s *System) GetLastEpochSeed(curBlockHeight int64) (util.Bytes32, error) {

	// At epoch 1, there is not last epoch so we use the hash of the genesis
	// data file as the seed.
	curEpoch := params.GetEpochOfHeight(curBlockHeight)
	if curEpoch == 1 {
		return config.GenesisFileHash(), nil
	}

	// Get block height where the last epoch seed is stored
	lastEpochSeedHeight := params.GetSeedHeightInEpochOfHeight(
		params.GetEndOfParentEpochOfHeight(curBlockHeight))
	lastSeedBlock, err := s.logic.SysKeeper().GetBlockInfo(lastEpochSeedHeight)
	if err != nil {
		return util.EmptyBytes32, err
	}

	// Get the preceding block
	blockBefore, err := s.logic.SysKeeper().GetBlockInfo(lastEpochSeedHeight - 1)
	if err != nil {
		return util.EmptyBytes32, errors.Wrap(err, "failed to get preceding block of seed block")
	}

	// If the last epoch has a seed, mix it with the preceding
	// block hash and return a hash of the mix...
	if !lastSeedBlock.EpochSeedOutput.IsEmpty() {
		mix := append(blockBefore.Hash, lastSeedBlock.EpochSeedOutput.Bytes()...)
		return util.BytesToBytes32(util.Blake2b256(mix)), nil
	}

	// ..otherwise, return only the preceding block hash
	return util.BytesToBytes32(blockBefore.Hash), nil
}

// MakeEpochSeedTx generates and returns a TxTypeEpochSeed transaction.
// This function is used by the mempool only.
func (s *System) MakeEpochSeedTx() (types.BaseTx, error) {

	// Get last committed block information
	bi, err := s.logic.SysKeeper().GetLastBlockInfo()
	if err != nil { // Returning error will cause the mempool to panic at height 0
		return nil, nil
	}

	// Determine if the next block is the first block in the end stage of the current epoch
	if !params.IsStartOfEndOfEpochOfHeight(bi.Height + 1) {
		return nil, nil
	}

	// Get the seed of the last epoch
	lastEpochSeed, err := s.GetLastEpochSeed(bi.Height)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get last epoch seed")
	}

	pvk, _ := s.logic.Cfg().G().PrivVal.GetKey()
	vrfKey, _ := vrf.GenerateKeyFromPrivateKey(pvk.PrivKey().MustBytes())
	output, proof := vrfKey.Prove(lastEpochSeed.Bytes())
	seedTx := types.NewBareTxEpochSeed()
	seedTx.Output = util.BytesToBytes32(output)
	seedTx.Proof = proof

	return seedTx, nil
}

// MakeSecret generates a 32 bytes secret for validator selection by xor-ing the
// last 32 valid epoch seeds. The most recent epoch seeds will be selected
// starting from the given height down to genesis.
// It returns ErrNoSeedFound if no seed was found
func (s *System) MakeSecret(height int64) ([]byte, error) {
	secrets, err := s.logic.SysKeeper().GetEpochSeeds(height, 32)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secrets")
	}

	if len(secrets) == 0 {
		return nil, ErrNoSeedFound
	}

	final := secrets[0]
	for _, s := range secrets[1:] {
		final = util.XorBytes(final, s)
	}

	return final, nil
}
