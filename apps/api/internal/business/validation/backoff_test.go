package validation

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	config := BackoffConfig{
		BaseDelay:  1 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1, // ±10%
		MaxDelay:   1 * time.Hour,
	}

	tests := []struct {
		name       string
		attempt    int
		minDelay   time.Duration // Min expected (base - jitter)
		maxDelay   time.Duration // Max expected (base + jitter)
		exactDelay time.Duration // Expected base without jitter
	}{
		{
			name:       "attempt 0",
			attempt:    0,
			minDelay:   900 * time.Millisecond,
			maxDelay:   1100 * time.Millisecond,
			exactDelay: 1 * time.Second,
		},
		{
			name:       "attempt 1",
			attempt:    1,
			minDelay:   1800 * time.Millisecond,
			maxDelay:   2200 * time.Millisecond,
			exactDelay: 2 * time.Second,
		},
		{
			name:       "attempt 2",
			attempt:    2,
			minDelay:   3600 * time.Millisecond,
			maxDelay:   4400 * time.Millisecond,
			exactDelay: 4 * time.Second,
		},
		{
			name:       "attempt 3",
			attempt:    3,
			minDelay:   7200 * time.Millisecond,
			maxDelay:   8800 * time.Millisecond,
			exactDelay: 8 * time.Second,
		},
		{
			name:       "negative attempt",
			attempt:    -1,
			minDelay:   900 * time.Millisecond,
			maxDelay:   1100 * time.Millisecond,
			exactDelay: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for jitter randomness
			for i := 0; i < 10; i++ {
				delay := CalculateBackoff(config, tt.attempt)

				if delay < tt.minDelay || delay > tt.maxDelay {
					t.Errorf("CalculateBackoff(%d) = %v, want between %v and %v",
						tt.attempt, delay, tt.minDelay, tt.maxDelay)
				}
			}
		})
	}
}

func TestCalculateBackoffMaxCap(t *testing.T) {
	config := BackoffConfig{
		BaseDelay:  1 * time.Minute,
		Multiplier: 10.0, // Very aggressive multiplier
		Jitter:     0.0,  // No jitter for predictable test
		MaxDelay:   5 * time.Minute,
	}

	// Attempt that would exceed max without cap
	// 1min * 10^10 = huge number, but should be capped at 5min
	delay := CalculateBackoff(config, 10)

	if delay > config.MaxDelay {
		t.Errorf("CalculateBackoff should cap at MaxDelay, got %v, want <= %v", delay, config.MaxDelay)
	}
}

func TestCalculateBackoffNoJitter(t *testing.T) {
	config := BackoffConfig{
		BaseDelay:  1 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.0, // No jitter
		MaxDelay:   1 * time.Hour,
	}

	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}

	for attempt, want := range expected {
		got := CalculateBackoff(config, attempt)
		if got != want {
			t.Errorf("CalculateBackoff(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestDefaultBackoffConfig(t *testing.T) {
	config := DefaultBackoffConfig()

	if config.BaseDelay != 1*time.Second {
		t.Errorf("BaseDelay = %v, want 1s", config.BaseDelay)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", config.Multiplier)
	}

	if config.Jitter != 0.1 {
		t.Errorf("Jitter = %v, want 0.1", config.Jitter)
	}

	if config.MaxDelay != 1*time.Hour {
		t.Errorf("MaxDelay = %v, want 1h", config.MaxDelay)
	}
}

func TestCalculateNextRetryTime(t *testing.T) {
	config := DefaultBackoffConfig()
	before := time.Now()

	nextRetry := CalculateNextRetryTime(config, 1)

	after := time.Now()

	// Should be approximately 2 seconds from now (attempt 1 = 2s delay)
	if nextRetry.Before(before.Add(1800 * time.Millisecond)) {
		t.Error("NextRetryTime is too soon")
	}

	if nextRetry.After(after.Add(2200 * time.Millisecond)) {
		t.Error("NextRetryTime is too late")
	}
}

func TestShouldRetryNow(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	future := time.Now().Add(1 * time.Minute)
	zero := time.Time{}

	if !ShouldRetryNow(past) {
		t.Error("ShouldRetryNow should return true for past time")
	}

	if ShouldRetryNow(future) {
		t.Error("ShouldRetryNow should return false for future time")
	}

	if !ShouldRetryNow(zero) {
		t.Error("ShouldRetryNow should return true for zero time")
	}
}

func TestBackoffSchedule(t *testing.T) {
	config := BackoffConfig{
		BaseDelay:  1 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.0, // No jitter for predictable test
		MaxDelay:   1 * time.Hour,
	}

	schedule := BackoffSchedule(config, 5)

	if len(schedule) != 5 {
		t.Errorf("BackoffSchedule length = %d, want 5", len(schedule))
	}

	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}

	for i, want := range expected {
		if schedule[i] != want {
			t.Errorf("schedule[%d] = %v, want %v", i, schedule[i], want)
		}
	}
}
