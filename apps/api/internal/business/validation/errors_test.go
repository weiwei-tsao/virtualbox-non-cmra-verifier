package validation

import (
	"errors"
	"testing"
)

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: ErrorUnknown,
		},
		{
			name:     "quota exhausted - 402",
			err:      errors.New("HTTP 402: Payment Required"),
			expected: ErrorQuotaExhausted,
		},
		{
			name:     "quota exhausted - credit exhausted",
			err:      errors.New("API credit exhausted"),
			expected: ErrorQuotaExhausted,
		},
		{
			name:     "rate limit - 429",
			err:      errors.New("HTTP 429: Too Many Requests"),
			expected: ErrorTransient,
		},
		{
			name:     "rate limit - text",
			err:      errors.New("rate limit exceeded"),
			expected: ErrorTransient,
		},
		{
			name:     "service unavailable - 503",
			err:      errors.New("HTTP 503: Service Unavailable"),
			expected: ErrorTransient,
		},
		{
			name:     "timeout",
			err:      errors.New("request timeout exceeded"),
			expected: ErrorTransient,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: ErrorTransient,
		},
		{
			name:     "bad request - 400",
			err:      errors.New("HTTP 400: Bad Request"),
			expected: ErrorPermanent,
		},
		{
			name:     "invalid format",
			err:      errors.New("invalid address format"),
			expected: ErrorPermanent,
		},
		{
			name:     "partial failure",
			err:      errors.New("partial batch failure"),
			expected: ErrorPartial,
		},
		{
			name:     "unknown error",
			err:      errors.New("unknown database error"),
			expected: ErrorUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CategorizeError(tt.err)
			if result != tt.expected {
				t.Errorf("CategorizeError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		expected bool
	}{
		{"transient should retry", ErrorTransient, true},
		{"unknown should retry", ErrorUnknown, true},
		{"permanent should not retry", ErrorPermanent, false},
		{"quota exhausted should not retry", ErrorQuotaExhausted, false},
		{"partial should retry", ErrorPartial, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRetry(tt.errType)
			if result != tt.expected {
				t.Errorf("ShouldRetry(%v) = %v, want %v", tt.errType, result, tt.expected)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	cause := errors.New("underlying error")
	ve := NewValidationError(ErrorTransient, "validation failed", cause)

	if ve.Type != ErrorTransient {
		t.Errorf("Type = %v, want %v", ve.Type, ErrorTransient)
	}

	if ve.Message != "validation failed" {
		t.Errorf("Message = %v, want %v", ve.Message, "validation failed")
	}

	expectedError := "validation failed: underlying error"
	if ve.Error() != expectedError {
		t.Errorf("Error() = %v, want %v", ve.Error(), expectedError)
	}

	if !errors.Is(ve, cause) {
		t.Error("ValidationError should wrap underlying cause")
	}
}

func TestIsQuotaExhausted(t *testing.T) {
	quotaErr := NewValidationError(ErrorQuotaExhausted, "quota exhausted", nil)
	otherErr := NewValidationError(ErrorTransient, "transient error", nil)
	plainErr := errors.New("plain error")
	quotaPlainErr := errors.New("HTTP 402: Payment Required")

	if !IsQuotaExhausted(quotaErr) {
		t.Error("IsQuotaExhausted should return true for quota exhausted error")
	}

	if IsQuotaExhausted(otherErr) {
		t.Error("IsQuotaExhausted should return false for non-quota error")
	}

	if IsQuotaExhausted(plainErr) {
		t.Error("IsQuotaExhausted should return false for plain error")
	}

	if !IsQuotaExhausted(quotaPlainErr) {
		t.Error("IsQuotaExhausted should check plain errors via CategorizeError")
	}
}

func TestErrorTypeString(t *testing.T) {
	tests := []struct {
		errType  ErrorType
		expected string
	}{
		{ErrorTransient, "TRANSIENT"},
		{ErrorPermanent, "PERMANENT"},
		{ErrorQuotaExhausted, "QUOTA_EXHAUSTED"},
		{ErrorPartial, "PARTIAL"},
		{ErrorUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.errType.String()
			if result != tt.expected {
				t.Errorf("String() = %v, want %v", result, tt.expected)
			}
		})
	}
}
