package types

import "fmt"

// RPC package error codes
const ()

// RPC error codes
const (
	// Account package error codes
	RPCErrCodeAccountNotFound = 30000
	RPCErrCodeGPGKeyNotFound  = 30001

	// Authentication error codes
	ErrCodeInvalidAuthHeader      = 40000
	ErrCodeInvalidAuthCredentials = 40001

	// Implementation error codes
	RPCErrCodeInvalidParamType  = 60000
	RPCErrCodeInvalidParamValue = 60001
	RPCErrCodeUnexpected        = 60002
)

// Account package errors
var (
	// ErrAccountUnknown indicates an unknown/missing account
	ErrAccountUnknown = fmt.Errorf("account not found")
)

// Crypto package errors
var (
	// ErrInvalidPrivKey indicates an invalid private key
	ErrInvalidPrivKey = fmt.Errorf("private key is invalid")
)

// Decode/Cast Error
var (
	// ErrArgDecode means a parameter could not be decoded
	ErrArgDecode = func(castType string, index int) error {
		return fmt.Errorf("Failed to decode argument.%d to %s", index, castType)
	}
	// ErrParamDecode means a parameter could not be decoded
	ErrParamDecode = func(castType string) error {
		return fmt.Errorf("Failed to decode parameter to %s", castType)
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
