package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// ValidationClient defines the interface for address validation (Smarty API).
type ValidationClient interface {
	ValidateMailbox(ctx context.Context, mb model.Mailbox) (model.Mailbox, error)
	ValidateMailboxBatch(ctx context.Context, mailboxes []model.Mailbox) ([]model.Mailbox, error)
}

// BatchValidationResult contains the outcome of a batch validation attempt.
type BatchValidationResult struct {
	Succeeded []model.Mailbox // Successfully validated mailboxes
	Failed    []FailedItem    // Failed validations with error info
	QuotaHit  bool            // True if quota was exhausted during validation
}

// FailedItem represents a mailbox that failed validation with error details.
type FailedItem struct {
	Mailbox   model.Mailbox
	Error     error
	ErrorType ErrorType
	Attempt   int
}

// BatchHandler handles batch validation with partial retry logic.
// If a batch fails, it retries failed items individually to save successful validations.
type BatchHandler struct {
	validator      ValidationClient
	backoffConfig  BackoffConfig
	maxAttempts    int
	onProgress     func(succeeded, failed int)
	logFn          func(string)
}

// NewBatchHandler creates a new batch validation handler.
func NewBatchHandler(
	validator ValidationClient,
	backoffConfig BackoffConfig,
	maxAttempts int,
	onProgress func(succeeded, failed int),
	logFn func(string),
) *BatchHandler {
	return &BatchHandler{
		validator:     validator,
		backoffConfig: backoffConfig,
		maxAttempts:   maxAttempts,
		onProgress:    onProgress,
		logFn:         logFn,
	}
}

// ValidateBatch validates a batch of mailboxes with partial retry logic.
//
// Strategy:
//  1. Try batch validation (up to 100 addresses per Smarty API request)
//  2. If batch succeeds → return all results
//  3. If batch fails with transient error → retry individually
//  4. If batch fails with permanent error → mark all as failed
//  5. If batch fails with quota exhaustion → stop and return partial results
func (h *BatchHandler) ValidateBatch(ctx context.Context, mailboxes []model.Mailbox) BatchValidationResult {
	result := BatchValidationResult{
		Succeeded: []model.Mailbox{},
		Failed:    []FailedItem{},
	}

	if len(mailboxes) == 0 {
		return result
	}

	// PHASE 1: Try batch validation
	validated, err := h.validator.ValidateMailboxBatch(ctx, mailboxes)
	if err == nil {
		// Batch succeeded - update all with validation timestamps
		for i := range validated {
			validated[i].LastValidatedAt = time.Now()
			validated[i].ValidationStatus = "validated"
			validated[i].LastValidationError = ""
		}
		result.Succeeded = validated
		if h.onProgress != nil {
			h.onProgress(len(validated), 0)
		}
		return result
	}

	// PHASE 2: Batch failed - categorize error
	errType := CategorizeError(err)

	if h.logFn != nil {
		h.logFn(fmt.Sprintf("batch validation failed (%s): %v, retrying %d items individually",
			errType.String(), err, len(mailboxes)))
	}

	// Check for quota exhaustion - stop immediately
	if errType == ErrorQuotaExhausted {
		result.QuotaHit = true
		for _, mb := range mailboxes {
			result.Failed = append(result.Failed, FailedItem{
				Mailbox:   mb,
				Error:     err,
				ErrorType: errType,
				Attempt:   0,
			})
		}
		return result
	}

	// Check for permanent error - don't retry
	if errType == ErrorPermanent {
		for _, mb := range mailboxes {
			result.Failed = append(result.Failed, FailedItem{
				Mailbox:   mb,
				Error:     err,
				ErrorType: errType,
				Attempt:   0,
			})
		}
		if h.onProgress != nil {
			h.onProgress(0, len(mailboxes))
		}
		return result
	}

	// PHASE 3: Retry individually (transient or unknown error)
	for _, mb := range mailboxes {
		select {
		case <-ctx.Done():
			// Context cancelled - mark remaining as failed
			result.Failed = append(result.Failed, FailedItem{
				Mailbox:   mb,
				Error:     ctx.Err(),
				ErrorType: ErrorTransient,
				Attempt:   0,
			})
			continue
		default:
		}

		validated, err := h.validateWithRetry(ctx, mb)
		if err != nil {
			errType := CategorizeError(err)

			// Check for quota exhaustion
			if errType == ErrorQuotaExhausted {
				result.QuotaHit = true
				result.Failed = append(result.Failed, FailedItem{
					Mailbox:   mb,
					Error:     err,
					ErrorType: errType,
					Attempt:   0,
				})
				// Stop processing remaining items
				break
			}

			result.Failed = append(result.Failed, FailedItem{
				Mailbox:   mb,
				Error:     err,
				ErrorType: errType,
				Attempt:   h.maxAttempts,
			})
		} else {
			validated.LastValidatedAt = time.Now()
			validated.ValidationStatus = "validated"
			validated.LastValidationError = ""
			result.Succeeded = append(result.Succeeded, validated)
		}
	}

	if h.onProgress != nil {
		h.onProgress(len(result.Succeeded), len(result.Failed))
	}

	return result
}

// validateWithRetry validates a single mailbox with exponential backoff retry.
func (h *BatchHandler) validateWithRetry(ctx context.Context, mb model.Mailbox) (model.Mailbox, error) {
	var lastErr error

	for attempt := 0; attempt < h.maxAttempts; attempt++ {
		// Apply backoff delay (except for first attempt)
		if attempt > 0 {
			delay := CalculateBackoff(h.backoffConfig, attempt-1)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return mb, ctx.Err()
			}
		}

		validated, err := h.validator.ValidateMailbox(ctx, mb)
		if err == nil {
			return validated, nil
		}

		lastErr = err
		errType := CategorizeError(err)

		// Stop immediately on permanent errors or quota exhaustion
		if errType == ErrorPermanent || errType == ErrorQuotaExhausted {
			return mb, err
		}

		// Continue retrying transient errors
		if h.logFn != nil && attempt < h.maxAttempts-1 {
			h.logFn(fmt.Sprintf("validation attempt %d/%d failed for %s (%s), retrying...",
				attempt+1, h.maxAttempts, mb.Link, errType.String()))
		}
	}

	return mb, lastErr
}
