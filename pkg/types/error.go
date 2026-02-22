package types

import (
	"encoding/json"
	"fmt"
)

// ErrorCode represents the type of error.
type ErrorCode string

const (
	ErrAuthentication        ErrorCode = "authentication_error"
	ErrRateLimit             ErrorCode = "rate_limit_error"
	ErrInvalidRequest        ErrorCode = "invalid_request_error"
	ErrModelNotFound         ErrorCode = "model_not_found"
	ErrProviderNotFound      ErrorCode = "provider_not_found"
	ErrProviderError         ErrorCode = "provider_error"
	ErrTimeout               ErrorCode = "timeout_error"
	ErrCapabilityUnavailable ErrorCode = "capability_unavailable"
	ErrQuotaExceeded         ErrorCode = "quota_exceeded"
	ErrCircuitOpen           ErrorCode = "circuit_open"
	ErrCooldown              ErrorCode = "cooldown"
	ErrInternalError         ErrorCode = "internal_error"
)

// ProviderError represents an error from an LLM provider.
type ProviderError struct {
	Code       ErrorCode       `json:"code"`
	Message    string          `json:"message"`
	Provider   string          `json:"provider,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
	Retryable  bool            `json:"retryable"`
	Raw        json.RawMessage `json:"raw,omitempty"` // Original error response
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Provider != "" {
		return fmt.Sprintf("[%s] %s: %s (status=%d)", e.Provider, e.Code, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// IsTransient returns true if the error is potentially recoverable through retry.
func (e *ProviderError) IsTransient() bool {
	return e.Retryable
}

// NewProviderError creates a new ProviderError.
func NewProviderError(code ErrorCode, message, provider string, statusCode int, retryable bool) *ProviderError {
	return &ProviderError{
		Code:       code,
		Message:    message,
		Provider:   provider,
		StatusCode: statusCode,
		Retryable:  retryable,
	}
}

// NewAuthenticationError creates an authentication error.
func NewAuthenticationError(provider, message string) *ProviderError {
	return NewProviderError(ErrAuthentication, message, provider, 401, false)
}

// NewRateLimitError creates a rate limit error.
func NewRateLimitError(provider, message string, retryable bool) *ProviderError {
	return NewProviderError(ErrRateLimit, message, provider, 429, retryable)
}

// NewInvalidRequestError creates an invalid request error.
func NewInvalidRequestError(message string) *ProviderError {
	return NewProviderError(ErrInvalidRequest, message, "", 400, false)
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(provider, message string) *ProviderError {
	return NewProviderError(ErrTimeout, message, provider, 0, true)
}

// ErrorAction represents the action to take when an error occurs.
// Four-level error classification (inspired by LLM-API-Key-Proxy).
type ErrorAction int

const (
	// ActionRetry indicates the request can be retried (5xx, timeout).
	ActionRetry ErrorAction = iota
	// ActionRotateKey indicates the API key should be rotated (401, 403, 429 key-level).
	ActionRotateKey
	// ActionFallback indicates should switch to another provider (model not supported, etc.).
	ActionFallback
	// ActionAbort indicates the error is not recoverable (400 invalid request, insufficient balance).
	ActionAbort
)

// String returns a string representation of the ErrorAction.
func (a ErrorAction) String() string {
	switch a {
	case ActionRetry:
		return "retry"
	case ActionRotateKey:
		return "rotate_key"
	case ActionFallback:
		return "fallback"
	case ActionAbort:
		return "abort"
	default:
		return "unknown"
	}
}

// ClassifyError determines the appropriate ErrorAction for a given error.
// This is a basic classification; more sophisticated logic is in manager/retry.go.
func ClassifyError(err error) ErrorAction {
	pe, ok := err.(*ProviderError)
	if !ok {
		return ActionAbort
	}

	switch pe.StatusCode {
	case 429:
		// Rate limit - could be key-level or global
		// For now, treat all as retryable
		return ActionRetry
	case 500, 502, 503, 504:
		// Server errors - retryable
		return ActionRetry
	case 401, 403:
		// Authentication errors - try rotating key
		return ActionRotateKey
	case 400, 422:
		// Client errors - not recoverable
		return ActionAbort
	case 404:
		// Not found - might need fallback to another provider
		return ActionFallback
	default:
		if pe.Retryable {
			return ActionRetry
		}
		return ActionAbort
	}
}

// Common sentinel errors for use with errors.Is().
var (
	ErrModelNotFoundSentinel    = &ProviderError{Code: ErrModelNotFound, Message: "model not found"}
	ErrProviderNotFoundSentinel = &ProviderError{Code: ErrProviderNotFound, Message: "provider not found"}
	ErrCircuitOpenSentinel      = &ProviderError{Code: ErrCircuitOpen, Message: "circuit breaker is open", Retryable: true}
	ErrQuotaExceededSentinel    = &ProviderError{Code: ErrQuotaExceeded, Message: "quota exceeded"}
)
