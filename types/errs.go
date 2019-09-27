package types

import "fmt"

// RPC package error codes
const (
	// ErrCodeInvalidAuthParams for invalid authorization parameter
	ErrCodeInvalidAuthParams = 40000
	// ErrCodeInvalidAuthCredentials for invalid authorization credentials
	ErrCodeInvalidAuthCredentials = 40001
)

// General error codes
const (
	// ErrCodeUnexpectedArgType for when an argument type is invalid
	ErrCodeUnexpectedArgType = 60000
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
	// ErrCodeListAccountFailed for failure to list account
	ErrCodeListAccountFailed = 30000
	// ErrCodeAccountNotFound for missing account
	ErrCodeAccountNotFound = 30001
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
