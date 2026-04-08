package validation

import (
	"math"
	"math/rand"
	"time"
)

// BackoffConfig configures exponential backoff with jitter.
type BackoffConfig struct {
	BaseDelay  time.Duration // Initial delay (e.g., 1 second)
	Multiplier float64       // Exponential multiplier (e.g., 2.0 for doubling)
	Jitter     float64       // Random jitter as percentage (e.g., 0.1 for ±10%)
	MaxDelay   time.Duration // Maximum delay cap (e.g., 1 hour)
}

// DefaultBackoffConfig returns sensible defaults for retry backoff.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		BaseDelay:  1 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1, // ±10%
		MaxDelay:   1 * time.Hour,
	}
}

// CalculateBackoff computes the backoff delay for a given attempt number.
// Uses exponential backoff with random jitter to prevent thundering herd.
//
// Formula: delay = min(baseDelay * (multiplier ^ attempt), maxDelay) ± jitter
//
// Examples with default config (base=1s, multiplier=2.0, jitter=0.1):
//   - Attempt 0: 1s ± 10% = 0.9s - 1.1s
//   - Attempt 1: 2s ± 10% = 1.8s - 2.2s
//   - Attempt 2: 4s ± 10% = 3.6s - 4.4s
//   - Attempt 3: 8s ± 10% = 7.2s - 8.8s
//   - Attempt 4: 16s ± 10% = 14.4s - 17.6s
func CalculateBackoff(config BackoffConfig, attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential delay
	delay := float64(config.BaseDelay) * math.Pow(config.Multiplier, float64(attempt))

	// Apply maximum cap
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Apply random jitter (±jitter%)
	jitterAmount := delay * config.Jitter
	jitter := (rand.Float64()*2 - 1) * jitterAmount // Random value in [-jitter, +jitter]
	delay += jitter

	// Ensure delay is never negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// CalculateNextRetryTime returns the timestamp when the next retry should occur.
func CalculateNextRetryTime(config BackoffConfig, attempt int) time.Time {
	delay := CalculateBackoff(config, attempt)
	return time.Now().Add(delay)
}

// ShouldRetryNow checks if enough time has passed for a retry.
func ShouldRetryNow(nextRetryAt time.Time) bool {
	return time.Now().After(nextRetryAt) || nextRetryAt.IsZero()
}

// BackoffSchedule generates a schedule of retry times for visualization/testing.
// Returns a slice of delays for each attempt up to maxAttempts.
func BackoffSchedule(config BackoffConfig, maxAttempts int) []time.Duration {
	schedule := make([]time.Duration, maxAttempts)
	for i := 0; i < maxAttempts; i++ {
		schedule[i] = CalculateBackoff(config, i)
	}
	return schedule
}
