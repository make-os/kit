package types

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
