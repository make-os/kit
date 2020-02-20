package logic

import (
	"fmt"
	types2 "gitlab.com/makeos/mosdef/logic/types"

	"gitlab.com/makeos/mosdef/params"
)

// ErrNoSeedFound means no secret was found
var ErrNoSeedFound = fmt.Errorf("no secret found")

// System implements types.TxLogic.
// Provides functionalities for executing transactions
type System struct {
	logic types2.Logic
}

// GetCurValidatorTicketPrice returns the ticket price.
// Ticket price increases by x percent after every n blocks
func (s *System) GetCurValidatorTicketPrice() float64 {
	return params.MinValidatorsTicketPrice
}
