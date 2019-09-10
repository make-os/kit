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

// Service Errors
var (
	// ErrServiceMethodUnknown means a requested method is unknown
	ErrServiceMethodUnknown = fmt.Errorf("service method unknown")
)
