package providers

import (
	"errors"
	"fmt"
)

type ErrorCategory string

const (
	ErrorConfiguration  ErrorCategory = "configuration_error"
	ErrorAuthentication ErrorCategory = "authentication_error"
	ErrorRateLimit      ErrorCategory = "rate_limit_error"
	ErrorTimeout        ErrorCategory = "timeout_error"
	ErrorTransient      ErrorCategory = "transient_error"
	ErrorFatal          ErrorCategory = "fatal_error"
)

type ProviderError struct {
	Category ErrorCategory
	Message  string
	Err      error
}

func (e ProviderError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "provider error"
}

func (e ProviderError) Unwrap() error {
	return e.Err
}

func IsRetryable(err error) bool {
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}

	switch providerErr.Category {
	case ErrorRateLimit, ErrorTimeout, ErrorTransient:
		return true
	default:
		return false
	}
}

func NewConfigurationError(message string, err error) error {
	return ProviderError{
		Category: ErrorConfiguration,
		Message:  message,
		Err:      err,
	}
}

func NewUnknownProviderError(name string) error {
	return NewConfigurationError(fmt.Sprintf("unknown provider %q", name), nil)
}
