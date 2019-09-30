package logic

import (
	"math"

	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
)

// System implements types.TxLogic.
// Provides functionalities for executing transactions
type System struct {
	logic types.Logic
}

// GetCurTicketPrice returns the ticket price
func (s *System) GetCurTicketPrice() float64 {

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
