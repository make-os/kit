package params

import (
	"github.com/shopspring/decimal"
	"time"
)

var (
	// BlockTime is the number of seconds between blocks
	BlockTime = 5

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
	MaxTicketActiveDur = 10

	// NumBlocksInThawPeriod is the number of blocks a decayed ticket will
	// exist for before it can be unbonded
	NumBlocksInThawPeriod = 10

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
	NetMaturityHeight = int64(10)

	// MinBootstrapLiveTickets is the minimum number of live tickets
	// required at bootstrap before the network is considered matured.
	MinBootstrapLiveTickets = 1

	// NumBlocksPerEpoch is the number of blocks in an epoch
	NumBlocksPerEpoch = 6

	// MaxValidatorsPerEpoch is the maximum number validators per epoch
	MaxValidatorsPerEpoch = 1

	// MinDelegatorCommission is the number of percentage delegators pay validators
	MinDelegatorCommission = decimal.NewFromFloat(10)

	// MinStorerStake is the minimum stake for a storer ticket
	MinStorerStake = decimal.NewFromFloat(10)

	// NumBlocksInStorerThawPeriod is the number of blocks before a storer stake
	// is unbonded
	NumBlocksInStorerThawPeriod = 10

	// ValidatorTicketPoolSize is the size of the ticket pool from which validator
	// tickets are selected randomly
	ValidatorTicketPoolSize = 60000

	// PushPoolCap is the pool transaction capacity
	PushPoolCap = 1000

	// PushPoolCleanUpInt is duration between each push pool clean-up operation
	PushPoolCleanUpInt = 30 * time.Minute

	// PushPoolItemTTL is the maximum life time of an item in the push pool
	PushPoolItemTTL = 24 * 3 * time.Hour
)
