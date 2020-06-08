package types

import "fmt"

// RPC error codes
const (
	// Authentication error codes
	ErrCodeInvalidAuthHeader      = 40000
	ErrCodeInvalidAuthCredentials = 40001

	// Implementation error codes
	RPCErrCodeInvalidParamValue = 60000
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
	// ErrArgDecode means a parameter could not be decoded
	ErrArgDecode = func(castType string, index int) error {
		return fmt.Errorf("failed to decode argument.%d to %s", index, castType)
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
