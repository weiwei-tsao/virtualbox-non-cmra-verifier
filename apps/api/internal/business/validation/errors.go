package validation

import (
	"errors"
	"strings"
)

// ErrorType categorizes validation errors for different handling strategies.
type ErrorType int

const (
	// ErrorUnknown is for unclassified errors (default to retry with caution)
	ErrorUnknown ErrorType = iota

	// ErrorTransient indicates temporary failures that should be retried
	// Examples: 429 rate limit, 503 service unavailable, network timeout
	ErrorTransient

	// ErrorPermanent indicates failures that won't succeed on retry
	// Examples: 400 bad request, invalid address format
	ErrorPermanent

	// ErrorQuotaExhausted indicates API quota/credit is exhausted
	// Should pause validation until quota resets
	ErrorQuotaExhausted

	// ErrorPartial indicates some items in batch succeeded, others failed
	// Should retry failed items individually
	ErrorPartial
)

// String returns the string representation of ErrorType.
func (e ErrorType) String() string {
	switch e {
	case ErrorTransient:
		return "TRANSIENT"
	case ErrorPermanent:
		return "PERMANENT"
	case ErrorQuotaExhausted:
		return "QUOTA_EXHAUSTED"
	case ErrorPartial:
		return "PARTIAL"
	default:
		return "UNKNOWN"
	}
}

// CategorizeError determines the error type based on error message/status.
// This enables smart retry logic and quota management.
func CategorizeError(err error) ErrorType {
	if err == nil {
		return ErrorUnknown
	}

	errMsg := strings.ToLower(err.Error())

	// Check for quota exhaustion (402 Payment Required, credit exhausted)
	if strings.Contains(errMsg, "402") ||
		strings.Contains(errMsg, "payment required") ||
		strings.Contains(errMsg, "quota exceeded") ||
		strings.Contains(errMsg, "credit") && strings.Contains(errMsg, "exhausted") {
		return ErrorQuotaExhausted
	}

	// Check for rate limiting (429 Too Many Requests)
	if strings.Contains(errMsg, "429") ||
		strings.Contains(errMsg, "too many requests") ||
		strings.Contains(errMsg, "rate limit") {
		return ErrorTransient
	}

	// Check for service unavailable / timeout (transient)
	if strings.Contains(errMsg, "503") ||
		strings.Contains(errMsg, "service unavailable") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") {
		return ErrorTransient
	}

	// Check for bad request (permanent - won't succeed on retry)
	if strings.Contains(errMsg, "400") ||
		strings.Contains(errMsg, "bad request") ||
		strings.Contains(errMsg, "invalid") && strings.Contains(errMsg, "format") {
		return ErrorPermanent
	}

	// Check for partial batch failure
	if strings.Contains(errMsg, "partial") ||
		strings.Contains(errMsg, "some items failed") {
		return ErrorPartial
	}

	// Default to unknown (retry with caution)
	return ErrorUnknown
}

// ShouldRetry determines if an error should be retried based on its type.
func ShouldRetry(errType ErrorType) bool {
	switch errType {
	case ErrorTransient, ErrorUnknown:
		return true
	case ErrorPermanent, ErrorQuotaExhausted:
		return false
	case ErrorPartial:
		return true // Retry individually
	default:
		return false
	}
}

// ValidationError wraps an error with categorization metadata.
type ValidationError struct {
	Type    ErrorType
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error for errors.Is/As.
func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// NewValidationError creates a categorized validation error.
func NewValidationError(errType ErrorType, message string, cause error) *ValidationError {
	return &ValidationError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// IsQuotaExhausted checks if an error indicates quota exhaustion.
func IsQuotaExhausted(err error) bool {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Type == ErrorQuotaExhausted
	}
	return CategorizeError(err) == ErrorQuotaExhausted
}

// IsTransient checks if an error is transient and should be retried.
func IsTransient(err error) bool {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Type == ErrorTransient
	}
	return CategorizeError(err) == ErrorTransient
}

// IsPermanent checks if an error is permanent and should not be retried.
func IsPermanent(err error) bool {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Type == ErrorPermanent
	}
	return CategorizeError(err) == ErrorPermanent
}
