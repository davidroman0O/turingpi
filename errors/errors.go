// Package errors provides domain-specific error types and error handling utilities
package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents a specific error type
type ErrorCode int

const (
	// Common error codes
	ErrUnknown ErrorCode = iota
	ErrNotFound
	ErrInvalidInput
	ErrPermission
	ErrConfiguration
	ErrConnection
	ErrTimeout
	ErrCancelled

	// Node-specific error codes
	ErrNodeNotFound
	ErrNodeOffline
	ErrNodeBusy
	ErrNodeFailure

	// BMC-specific error codes
	ErrBMCNotFound
	ErrBMCOffline
	ErrBMCFailure
	ErrBMCTimeout

	// Image-specific error codes
	ErrImageNotFound
	ErrImageInvalid
	ErrImageCorrupt
	ErrImageIO

	// Network-specific error codes
	ErrNetworkUnavailable
	ErrNetworkTimeout
	ErrNetworkConfig
	ErrNetworkIO
)

// Error represents a domain-specific error with context
type Error struct {
	// Code identifies the error type
	Code ErrorCode

	// Message provides human-readable error details
	Message string

	// Op describes the operation that failed
	Op string

	// Cause is the underlying error that triggered this one
	Cause error

	// Context holds additional error context
	Context map[string]interface{}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap implements the errors.Unwrap interface
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is implements the errors.Is interface
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithOp adds an operation name to the error
func WithOp(err error, op string) error {
	if err == nil {
		return nil
	}

	e, ok := err.(*Error)
	if !ok {
		return &Error{
			Code:    ErrUnknown,
			Message: err.Error(),
			Op:      op,
			Cause:   err,
		}
	}

	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Op:      op,
		Cause:   e.Cause,
		Context: e.Context,
	}
}

// WithContext adds context to the error
func WithContext(err error, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	e, ok := err.(*Error)
	if !ok {
		return &Error{
			Code:    ErrUnknown,
			Message: err.Error(),
			Cause:   err,
			Context: context,
		}
	}

	// Merge contexts if error already has context
	newContext := make(map[string]interface{})
	for k, v := range e.Context {
		newContext[k] = v
	}
	for k, v := range context {
		newContext[k] = v
	}

	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Op:      e.Op,
		Cause:   e.Cause,
		Context: newContext,
	}
}

// New creates a new Error
func New(code ErrorCode, message string) error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, code ErrorCode, message string) error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// GetCode returns the error code from an error
func GetCode(err error) ErrorCode {
	if err == nil {
		return ErrUnknown
	}

	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ErrUnknown
}

// GetContext returns the error context
func GetContext(err error) map[string]interface{} {
	if err == nil {
		return nil
	}

	var e *Error
	if errors.As(err, &e) {
		return e.Context
	}
	return nil
}

// IsNotFound returns true if the error is a not found error
func IsNotFound(err error) bool {
	return GetCode(err) == ErrNotFound
}

// IsPermission returns true if the error is a permission error
func IsPermission(err error) bool {
	return GetCode(err) == ErrPermission
}

// IsTimeout returns true if the error is a timeout error
func IsTimeout(err error) bool {
	return GetCode(err) == ErrTimeout
}

// IsCancelled returns true if the error is a cancelled error
func IsCancelled(err error) bool {
	return GetCode(err) == ErrCancelled
}

// IsTemporary returns true if the error is temporary
func IsTemporary(err error) bool {
	code := GetCode(err)
	return code == ErrTimeout ||
		code == ErrConnection ||
		code == ErrNetworkUnavailable ||
		code == ErrNetworkTimeout
}

// IsRetryable returns true if the error can be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	code := GetCode(err)
	return code == ErrTimeout ||
		code == ErrConnection ||
		code == ErrNetworkUnavailable ||
		code == ErrNetworkTimeout ||
		code == ErrNodeBusy ||
		code == ErrBMCTimeout
}
