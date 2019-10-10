package logic

import (
	"fmt"
	"math"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/logic/keepers"
	"github.com/pkg/errors"

	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
)

// ErrNoSecretFound means no secret was found
var ErrNoSecretFound = fmt.Errorf("no secret found")

// System implements types.TxLogic.
// Provides functionalities for executing transactions
type System struct {
	logic types.Logic
}

// GetCurValidatorTicketPrice returns the ticket price
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

// CheckSetNetMaturity checks whether the network
// has reached a matured period. If it has not,
// we return error. However, if it is just
// met the maturity condition in this call, we
// mark the network as mature
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
	numLiveTickets, err := s.logic.GetTicketManager().CountLiveTickets()
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

// GetCurretEpochSecretTx generates and returns a TxTypeEpochSecret
// transaction only if the next block height is the last in the
// current epoch.
func (s *System) GetCurretEpochSecretTx() (types.Tx, error) {

	// Get last committed block information
	bi, err := s.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, nil
	}

	// Simple calculation to determine if the next block is
	// the last in the current epoch
	if (bi.Height+1)%int64(params.NumBlocksPerEpoch) != 0 {
		return nil, nil
	}

	// At the point, the next block is the last in the epoch,
	// so we need to generate a 64 bytes random value wrapped
	// in a TxTypeEpochSecret transaction
	randVal := s.logic.GetDRand().Get(0)
	if randVal == nil {
		return nil, fmt.Errorf("failed to get random value from drand")
	}

	secretTx := types.NewBareTx(types.TxTypeEpochSecret)
	secretTx.Secret = randVal.Randomness.Point
	secretTx.PreviousSecret = randVal.Previous
	secretTx.SecretRound = randVal.Round

	return secretTx, nil
}

// MakeSecret generates a 64 bytes secret for validator
// selection by xoring the last 32 valid epoch secrets.
// The most recent secrets will be selected starting from
// the given height down to genesis.
// It returns ErrNoSecretFound if no error was found
func (s *System) MakeSecret(height int64) ([]byte, error) {
	secrets, err := s.logic.SysKeeper().GetSecrets(height, 32, int64(params.NumBlocksPerEpoch))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secrets")
	}

	if len(secrets) == 0 {
		return nil, ErrNoSecretFound
	}

	final := secrets[0]
	for _, s := range secrets[1:] {
		final = util.XorBytes(final, s)
	}

	return final, nil
}
