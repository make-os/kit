package types

import "fmt"

// RPC error codes
const (
	// Authentication error codes
	ErrCodeInvalidAuthHeader      = 40000
	ErrCodeInvalidAuthCredentials = 40001

	// Implementation error codes
	RPCErrCodeInvalidParamType  = 60000
	RPCErrCodeInvalidParamValue = 60001
)

// General
var (
	ErrKeyUnknown        = fmt.Errorf("key not found")
	ErrAccountUnknown    = fmt.Errorf("account not found")
	ErrPushKeyUnknown    = fmt.Errorf("push key not found")
	ErrInvalidPrivKey    = fmt.Errorf("private key is invalid")
	ErrRepoNotFound      = fmt.Errorf("repo not found")
	ErrTxNotFound        = fmt.Errorf("transaction not found")
	ErrInvalidPassphrase = fmt.Errorf("invalid passphrase")
)

// Decode/Cast Error
var (
	// ErrIntSliceArgDecode means an interface slice parameter could not be decoded
	ErrIntSliceArgDecode = func(castType string, index, sliceIndex int) error {
		return fmt.Errorf("failed to decode argument.%d[%d] to %s", index, sliceIndex, castType)
	}
)

// ABI App Error Codes
var (
	ErrCodeTxBadEncode        uint32 = 20000
	ErrCodeTxFailedValidation uint32 = 20001
	ErrCodeTxPoolReject       uint32 = 20002
)

// Transaction processing errors
const (
	ErrCodeFailedDecode     = uint32(1)
	ErrCodeExecFailure      = uint32(2)
	ErrCodeMaxTxTypeReached = 3
)

var ErrExit = fmt.Errorf("exit")
