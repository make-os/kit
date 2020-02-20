package types

import (
	"gitlab.com/makeos/mosdef/types/msgs"
	"gitlab.com/makeos/mosdef/util"
)

// Service provides an interface for exposing functionalities.
// It is meant to be used by packages that offer operations
// that other packages or processes might need
type Service interface {
	SendTx(tx msgs.BaseTx) (util.Bytes32, error)
	GetBlock(height int64) (map[string]interface{}, error)
	GetCurrentHeight() (int64, error)
	GetNonce(address util.String) (uint64, error)
}
