package logic

import (
	"fmt"
	"math"

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
