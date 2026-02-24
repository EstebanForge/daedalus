package providers

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

func EncodeEventError(err error) string {
	if err == nil {
		return ""
	}

	var providerErr ProviderError
	if errors.As(err, &providerErr) {
		return string(providerErr.Category) + "|" + providerErr.Error()
	}
	return string(ErrorFatal) + "|" + err.Error()
}

// mapGenericCLIError classifies a CLI subprocess error into a typed ProviderError.
// binary is the CLI binary name used only in the "not found" message.
func mapGenericCLIError(binary string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case errors.Is(err, exec.ErrNotFound):
		return NewConfigurationError(binary+" CLI binary not found in PATH", err)
	}

	lowMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lowMsg, "executable file not found"),
		strings.Contains(lowMsg, "not found in $path"):
		return NewConfigurationError(binary+" CLI binary not found in PATH", err)
	case strings.Contains(lowMsg, "authentication"),
		strings.Contains(lowMsg, "auth"),
		strings.Contains(lowMsg, "token"),
		strings.Contains(lowMsg, "unauthorized"):
		return ProviderError{Category: ErrorAuthentication, Message: err.Error(), Err: err}
	case strings.Contains(lowMsg, "rate limit"),
		strings.Contains(lowMsg, "429"):
		return ProviderError{Category: ErrorRateLimit, Message: err.Error(), Err: err}
	case strings.Contains(lowMsg, "timeout"),
		strings.Contains(lowMsg, "timed out"),
		strings.Contains(lowMsg, "deadline exceeded"):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case strings.Contains(lowMsg, "temporar"),
		strings.Contains(lowMsg, "unavailable"),
		strings.Contains(lowMsg, "try again"):
		return ProviderError{Category: ErrorTransient, Message: err.Error(), Err: err}
	default:
		return ProviderError{Category: ErrorFatal, Message: err.Error(), Err: err}
	}
}

func DecodeEventError(message string) error {
	parts := strings.SplitN(strings.TrimSpace(message), "|", 2)
	if len(parts) != 2 {
		return ProviderError{
			Category: ErrorFatal,
			Message:  strings.TrimSpace(message),
		}
	}

	category := ErrorCategory(strings.TrimSpace(parts[0]))
	text := strings.TrimSpace(parts[1])
	switch category {
	case ErrorConfiguration, ErrorAuthentication, ErrorRateLimit, ErrorTimeout, ErrorTransient, ErrorFatal:
		return ProviderError{Category: category, Message: text}
	default:
		return ProviderError{Category: ErrorFatal, Message: text}
	}
}
