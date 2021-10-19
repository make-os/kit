package types

import "fmt"

// RPC error codes
const (
	ErrCodeInvalidAuthHeader      = 40000
	ErrCodeInvalidAuthCredentials = 40001
	ErrRPCServerError             = 50000
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
		sliceIndexStr := ""
		if sliceIndex > -1 {
			sliceIndexStr = fmt.Sprintf("[%d]", sliceIndex)
		}
		return fmt.Errorf("failed to decode argument.%d%s to %s", index, sliceIndexStr, castType)
	}
)

// ABI App Error Codes
var (
	ErrCodeTxBadEncode        uint32 = 20000
	ErrCodeTxFailedValidation uint32 = 20001
)

// Transaction processing errors
const (
	ErrCodeFailedDecode     = uint32(1)
	ErrCodeExecFailure      = uint32(2)
	ErrCodeMaxTxTypeReached = 3
)

var ErrExit = fmt.Errorf("exit")
var ErrSkipped = fmt.Errorf("skipped")
