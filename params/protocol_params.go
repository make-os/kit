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
)
