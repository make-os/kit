package params

import (
	"github.com/shopspring/decimal"
)

var (
	// MaxBlockSize is the max size of a block
	MaxBlockSize = int64(1000000)

	// FeePerByte is the cost per byte of a transaction
	FeePerByte = decimal.NewFromFloat(0.00001)

	// TxTTL is the number of days a transaction
	// can last for in the pool
	TxTTL = 7

	// TxPoolCap is the number of transactions the tx pool can contain
	TxPoolCap = int64(10000)

	// MinTicketMatDur is the number of blocks that must be created
	// before a ticket is considered matured.
	MinTicketMatDur = 10

	// MaxTicketActiveDur is the number of blocks before a matured
	// ticket is considered spent or decayed.
	MaxTicketActiveDur = 600

	// InitialTicketPrice is the initial price of a ticket (window 0)
	InitialTicketPrice = float64(10)

	// NumBlocksPerPriceWindow is the number of blocks before price is increased
	NumBlocksPerPriceWindow = 100

	// PricePercentIncrease is the percentage increase of ticket price
	PricePercentIncrease = float64(0.2)

	// MaxValTicketsPerBlock is the max number of validators
	// ticket transaction a block can include.
	MaxValTicketsPerBlock = 1

	// NetMaturityHeight is the block height when the network is considered
	// ready for more responsibilities and advanced operations.
	NetMaturityHeight = int64(50)

	// MinBootstrapLiveTickets is the minimum number of live tickets
	// required at bootstrap before the network is considered matured.
	MinBootstrapLiveTickets = 1

	// NumBlocksPerEpoch is the number of blocks in an epoch
	NumBlocksPerEpoch = 120
)
