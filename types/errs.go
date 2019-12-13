package types

import "fmt"

// RPC package error codes
const (
	// ErrCodeInvalidAuthHeader for invalid authorization parameter
	ErrCodeInvalidAuthHeader = 40000
	// ErrCodeInvalidAuthCredentials for invalid authorization credentials
	ErrCodeInvalidAuthCredentials = 40001
)

// General error codes
const (
	// ErrCodeInvalidParamType for when an parameter type is invalid
	ErrCodeInvalidParamType = 60000
	// ErrCodeCallParamError for when a call parameter is invalid
	ErrCodeCallParamError = 60001
	// ErrValueDecodeFailed for when decoding a value failed
	ErrValueDecodeFailed = 60002
	// ErrCodeUnexpected for when an unexpected error occurs
	ErrCodeUnexpected = 60003
	// ErrCodeCallParamTypeError for when a call parameter type is invalid
	ErrCodeCallParamTypeError = 60004
)

// Account package error codes
const (
	// ErrCodeAccountNotFound for missing account
	ErrCodeAccountNotFound = 30000
	// ErrCodeGPGKeyNotFound for missing gpg key
	ErrCodeGPGKeyNotFound = 30001
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

// Service Errors
var (
	// ErrServiceMethodUnknown means a requested method is unknown
	ErrServiceMethodUnknown = fmt.Errorf("service method unknown")
)

// Validation Errors
var (
	// ErrTxVerificationFailed means signature verification failed
	ErrTxVerificationFailed = fmt.Errorf("transaction verification failed")
)

// ABI App Error Codes
var (
	ErrCodeTxBadEncode        uint32 = 20000
	ErrCodeTxFailedValidation uint32 = 20001
	ErrCodeTxPoolReject       uint32 = 20002
)

// Transaction errors
var (
	//ErrTxTypeUnknown means transaction type is unknown
	ErrTxTypeUnknown = fmt.Errorf("unknown transaction type")
)

// Transaction processing errors
const (
	ErrCodeFailedDecode     = uint32(1)
	ErrCodeExecFailure      = 2
	ErrCodeMaxTxTypeReached = 3
	ErrCodeTxTypeUnexpected = 4
	ErrCodeTxInvalidValue   = 5
)

// Network errors
var (
	ErrImmatureNetwork = fmt.Errorf("network is immature")
)

// Ticket errors
var (
	ErrTicketNotFound = fmt.Errorf("ticket not found")
)
