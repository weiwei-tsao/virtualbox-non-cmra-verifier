package validation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// MailboxRepository defines the interface for database operations.
type MailboxRepository interface {
	// FetchByValidationStatus fetches mailboxes by status and priority
	FetchByValidationStatus(ctx context.Context, status, priority string, limit int) ([]model.Mailbox, error)

	// BatchUpsert updates multiple mailboxes
	BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error

	// UpdateValidationStatus updates a single mailbox validation status
	UpdateValidationStatus(ctx context.Context, id string, status string, priority string, error string, nextRetryAt time.Time, attempts int) error
}

// ValidationRunRepository defines the interface for validation run operations.
type ValidationRunRepository interface {
	CreateRun(ctx context.Context, run *model.ValidationRun) error
	UpdateRun(ctx context.Context, run *model.ValidationRun) error
	GetRun(ctx context.Context, runID string) (*model.ValidationRun, error)
	ListRuns(ctx context.Context, limit int) ([]model.ValidationRun, error)
	UpdateStatus(ctx context.Context, runID string, status string) error
	UpdateStats(ctx context.Context, runID string, stats model.ValidationRunStats) error
}

// ValidationService orchestrates the validation process with quota management and retry logic.
type ValidationService struct {
	validator         ValidationClient
	repository        MailboxRepository
	runRepository     ValidationRunRepository
	quotaManager      *QuotaManager
	batchHandler      *BatchHandler
	config            model.CrawlerConfig
	logFn             func(string)
	currentRunID      string // Track current run ID
	itemsSample       []model.ValidationItem
	errorsSample      []model.ErrorSample
	maxSampleSize     int
	mu                sync.Mutex
	isRunning         bool
}

// NewValidationService creates a new validation service.
func NewValidationService(
	validator ValidationClient,
	repository MailboxRepository,
	runRepository ValidationRunRepository,
	quotaManager *QuotaManager,
	config model.CrawlerConfig,
	logFn func(string),
) *ValidationService {
	backoffConfig := BackoffConfig{
		BaseDelay:  config.RetryBackoffBase,
		Multiplier: config.RetryBackoffMultiplier,
		Jitter:     config.RetryBackoffJitter,
		MaxDelay:   1 * time.Hour,
	}

	batchHandler := NewBatchHandler(
		validator,
		backoffConfig,
		config.MaxRetryAttempts,
		nil, // onProgress callback
		logFn,
	)

	return &ValidationService{
		validator:     validator,
		repository:    repository,
		runRepository: runRepository,
		quotaManager:  quotaManager,
		batchHandler:  batchHandler,
		config:        config,
		logFn:         logFn,
		itemsSample:   []model.ValidationItem{},
		errorsSample:  []model.ErrorSample{},
		maxSampleSize: 50,
	}
}

// ValidationStats tracks validation run statistics.
type ValidationStats struct {
	HighPriorityProcessed   int
	MediumPriorityProcessed int
	LowPriorityProcessed    int
	Succeeded               int
	Failed                  int
	QuotaExhausted          bool
	QuotaRemaining          int
}

// ProcessPendingValidations processes the validation queue by priority.
// Returns statistics about what was processed.
//
// Processing order:
//  1. HIGH priority (new addresses, data changed, >90 days old)
//  2. MEDIUM priority (approaching revalidation threshold: 70-89 days)
//  3. LOW priority (recently validated: <70 days)
//
// Quota allocation:
//  - High: 50% of daily budget
//  - Medium: 30% of daily budget
//  - Low: 20% of daily budget
func (s *ValidationService) ProcessPendingValidations(ctx context.Context, triggerType string) (ValidationStats, error) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return ValidationStats{}, errors.New("a validation run is already in progress")
	}
	s.isRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	stats := ValidationStats{}

	// Default trigger type if not provided
	if triggerType == "" {
		triggerType = "automatic"
	}

	// Generate run ID
	runID := fmt.Sprintf("validation_%d", time.Now().Unix())
	s.currentRunID = runID
	s.itemsSample = []model.ValidationItem{}
	s.errorsSample = []model.ErrorSample{}

	// Create validation run record
	run := &model.ValidationRun{
		RunID:       runID,
		Status:      "running",
		StartedAt:   time.Now(),
		TriggerType: triggerType,
		Stats:       model.ValidationRunStats{},
	}

	if err := s.runRepository.CreateRun(ctx, run); err != nil {
		if s.logFn != nil {
			s.logFn(fmt.Sprintf("Failed to create validation run: %v", err))
		}
		// Continue even if run creation fails
	}

	if s.logFn != nil {
		s.logFn(fmt.Sprintf("Starting validation processing (run: %s)...", runID))
	}

	// Check available quota
	highQuota, mediumQuota, lowQuota, err := s.quotaManager.GetAvailableQuota(ctx)
	if err != nil {
		s.finalizeRun(ctx, "failed", stats)
		return stats, fmt.Errorf("get quota: %w", err)
	}

	if highQuota+mediumQuota+lowQuota == 0 {
		stats.QuotaExhausted = true
		if s.logFn != nil {
			s.logFn("Daily quota exhausted, skipping validation")
		}
		s.finalizeRun(ctx, "success", stats)
		return stats, nil
	}

	if s.logFn != nil {
		s.logFn(fmt.Sprintf("Available quota - High: %d, Medium: %d, Low: %d",
			highQuota, mediumQuota, lowQuota))
	}

	// PHASE 1: Process high priority (up to high quota limit)
	if highQuota > 0 {
		processed, succeeded, failed, quotaHit := s.processPriority(ctx, "high", highQuota)
		stats.HighPriorityProcessed = processed
		stats.Succeeded += succeeded
		stats.Failed += failed

		// Update progress
		s.updateRunStats(ctx, stats)

		if quotaHit {
			stats.QuotaExhausted = true
			s.finalizeRun(ctx, "partial", stats)
			return stats, nil
		}
	}

	// PHASE 2: Process medium priority (up to medium quota limit)
	if mediumQuota > 0 {
		processed, succeeded, failed, quotaHit := s.processPriority(ctx, "medium", mediumQuota)
		stats.MediumPriorityProcessed = processed
		stats.Succeeded += succeeded
		stats.Failed += failed

		// Update progress
		s.updateRunStats(ctx, stats)

		if quotaHit {
			stats.QuotaExhausted = true
			s.finalizeRun(ctx, "partial", stats)
			return stats, nil
		}
	}

	// PHASE 3: Process low priority (up to low quota limit)
	if lowQuota > 0 {
		processed, succeeded, failed, quotaHit := s.processPriority(ctx, "low", lowQuota)
		stats.LowPriorityProcessed = processed
		stats.Succeeded += succeeded
		stats.Failed += failed

		// Update progress
		s.updateRunStats(ctx, stats)

		if quotaHit {
			stats.QuotaExhausted = true
		}
	}

	// Get remaining quota
	high, medium, low, _ := s.quotaManager.GetAvailableQuota(ctx)
	stats.QuotaRemaining = high + medium + low

	// Finalize run
	status := "success"
	if stats.QuotaExhausted {
		status = "partial"
	}
	s.finalizeRun(ctx, status, stats)

	return stats, nil
}

// processPriority processes validations for a specific priority level.
// Returns: (processed count, succeeded count, failed count, quota hit flag)
func (s *ValidationService) processPriority(ctx context.Context, priority string, quotaLimit int) (int, int, int, bool) {
	if quotaLimit <= 0 {
		return 0, 0, 0, false
	}

	// Fetch mailboxes for this priority (both "pending" and "needs_revalidation")
	// Split quota between the two statuses
	pendingLimit := quotaLimit / 2
	revalidationLimit := quotaLimit - pendingLimit

	pendingMailboxes, err := s.repository.FetchByValidationStatus(ctx, "pending", priority, pendingLimit)
	if err != nil {
		if s.logFn != nil {
			s.logFn(fmt.Sprintf("Error fetching pending %s priority mailboxes: %v", priority, err))
		}
		return 0, 0, 0, false
	}

	revalidationMailboxes, err := s.repository.FetchByValidationStatus(ctx, "needs_revalidation", priority, revalidationLimit)
	if err != nil {
		if s.logFn != nil {
			s.logFn(fmt.Sprintf("Error fetching revalidation %s priority mailboxes: %v", priority, err))
		}
		return 0, 0, 0, false
	}

	// Combine both lists
	mailboxes := append(pendingMailboxes, revalidationMailboxes...)

	if len(mailboxes) == 0 {
		if s.logFn != nil {
			s.logFn(fmt.Sprintf("No pending or revalidation %s priority mailboxes", priority))
		}
		return 0, 0, 0, false
	}

	if s.logFn != nil {
		s.logFn(fmt.Sprintf("Processing %d %s priority mailboxes", len(mailboxes), priority))
	}

	// Process in batches (Smarty API limit: 100 per request)
	batchSize := s.config.ValidationBatchSize
	totalProcessed := 0
	totalSucceeded := 0
	totalFailed := 0

	for i := 0; i < len(mailboxes); i += batchSize {
		end := i + batchSize
		if end > len(mailboxes) {
			end = len(mailboxes)
		}
		batch := mailboxes[i:end]

		// Validate batch
		result := s.batchHandler.ValidateBatch(ctx, batch)

		// Update successful validations
		if len(result.Succeeded) > 0 {
			for j := range result.Succeeded {
				result.Succeeded[j].ValidationAttempts = 0
				result.Succeeded[j].LastValidationError = ""

				// Record sample
				s.recordValidationItem(
					result.Succeeded[j],
					"validated",
					result.Succeeded[j].CMRA,
					result.Succeeded[j].RDI,
					"",
				)
			}

			if err := s.repository.BatchUpsert(ctx, result.Succeeded); err != nil {
				if s.logFn != nil {
					s.logFn(fmt.Sprintf("Error saving successful validations: %v", err))
				}
			} else {
				totalSucceeded += len(result.Succeeded)

				// Record quota usage
				if err := s.quotaManager.RecordUsage(ctx, priority, len(result.Succeeded)); err != nil {
					if s.logFn != nil {
						s.logFn(fmt.Sprintf("Error recording quota usage: %v", err))
					}
				}
			}
		}

		// Handle failed validations
		if len(result.Failed) > 0 {
			s.handleFailedValidations(ctx, result.Failed)
			totalFailed += len(result.Failed)
		}

		totalProcessed += len(batch)

		// Check if quota was hit
		if result.QuotaHit {
			if s.logFn != nil {
				s.logFn("Quota exhausted during validation")
			}
			return totalProcessed, totalSucceeded, totalFailed, true
		}
	}

	return totalProcessed, totalSucceeded, totalFailed, false
}

// handleFailedValidations processes failed validation items and schedules retries.
func (s *ValidationService) handleFailedValidations(ctx context.Context, failed []FailedItem) {
	backoffConfig := BackoffConfig{
		BaseDelay:  s.config.RetryBackoffBase,
		Multiplier: s.config.RetryBackoffMultiplier,
		Jitter:     s.config.RetryBackoffJitter,
		MaxDelay:   1 * time.Hour,
	}

	for _, item := range failed {
		mb := item.Mailbox
		attempts := mb.ValidationAttempts + 1

		var status string
		var nextRetryAt time.Time

		// Determine next status based on error type and attempt count
		switch {
		case item.ErrorType == ErrorPermanent:
			// Permanent error - mark as failed, no retry
			status = "failed"
			nextRetryAt = time.Time{} // Zero time means no retry

		case item.ErrorType == ErrorQuotaExhausted:
			// Quota exhausted - keep as pending for next run
			status = "pending"
			nextRetryAt = time.Now().Add(24 * time.Hour) // Try again tomorrow
			attempts = mb.ValidationAttempts // Don't increment attempts

		case attempts >= s.config.MaxRetryAttempts:
			// Max retries exceeded - mark for manual review
			status = "manual_review"
			nextRetryAt = time.Time{} // Zero time means no auto-retry

		default:
			// Transient error - schedule retry with backoff
			status = "retry_scheduled"
			nextRetryAt = CalculateNextRetryTime(backoffConfig, attempts)
		}

		// Record error sample
		s.recordError(mb.Link, item.Error.Error())

		// Record validation item
		s.recordValidationItem(mb, status, "", "", item.Error.Error())

		// Update validation status
		err := s.repository.UpdateValidationStatus(
			ctx,
			mb.ID,
			status,
			mb.ValidationPriority,
			item.Error.Error(),
			nextRetryAt,
			attempts,
		)

		if err != nil && s.logFn != nil {
			s.logFn(fmt.Sprintf("Error updating validation status for %s: %v", mb.Link, err))
		}
	}
}

// ProcessRetries processes mailboxes scheduled for retry.
// Returns count of mailboxes moved back to pending status.
func (s *ValidationService) ProcessRetries(ctx context.Context) (int, error) {
	// Fetch mailboxes with status="retry_scheduled" where nextRetryAt <= now
	retryReady, err := s.repository.FetchByValidationStatus(ctx, "retry_scheduled", "", 1000)
	if err != nil {
		return 0, fmt.Errorf("fetch retry mailboxes: %w", err)
	}

	count := 0
	for _, mb := range retryReady {
		// Check if retry time has arrived
		if ShouldRetryNow(mb.NextRetryAt) {
			// Move back to pending for next validation run
			err := s.repository.UpdateValidationStatus(
				ctx,
				mb.ID,
				"pending",
				mb.ValidationPriority,
				mb.LastValidationError, // Keep error history
				time.Time{},            // Clear next retry time
				mb.ValidationAttempts,  // Keep attempt count
			)

			if err != nil && s.logFn != nil {
				s.logFn(fmt.Sprintf("Error moving %s back to pending: %v", mb.Link, err))
			} else {
				count++
			}
		}
	}

	if s.logFn != nil && count > 0 {
		s.logFn(fmt.Sprintf("Moved %d mailboxes from retry_scheduled to pending", count))
	}

	return count, nil
}

// updateRunStats updates the validation run with current stats.
func (s *ValidationService) updateRunStats(ctx context.Context, stats ValidationStats) {
	if s.currentRunID == "" {
		return
	}

	runStats := model.ValidationRunStats{
		HighPriorityProcessed:   stats.HighPriorityProcessed,
		MediumPriorityProcessed: stats.MediumPriorityProcessed,
		LowPriorityProcessed:    stats.LowPriorityProcessed,
		Succeeded:               stats.Succeeded,
		Failed:                  stats.Failed,
		QuotaExhausted:          stats.QuotaExhausted,
		QuotaRemaining:          stats.QuotaRemaining,
	}

	if err := s.runRepository.UpdateStats(ctx, s.currentRunID, runStats); err != nil {
		if s.logFn != nil {
			s.logFn(fmt.Sprintf("Failed to update run stats: %v", err))
		}
	}
}

// finalizeRun marks the validation run as complete.
func (s *ValidationService) finalizeRun(ctx context.Context, status string, stats ValidationStats) {
	if s.currentRunID == "" {
		return
	}

	// Get remaining quota
	high, medium, low, _ := s.quotaManager.GetAvailableQuota(ctx)
	stats.QuotaRemaining = high + medium + low

	// Update final stats
	s.updateRunStats(ctx, stats)

	// Update status
	if err := s.runRepository.UpdateStatus(ctx, s.currentRunID, status); err != nil {
		if s.logFn != nil {
			s.logFn(fmt.Sprintf("Failed to finalize run: %v", err))
		}
	}

	// Update run with samples
	run, err := s.runRepository.GetRun(ctx, s.currentRunID)
	if err == nil {
		run.ItemsSample = s.itemsSample
		run.ErrorsSample = s.errorsSample
		s.runRepository.UpdateRun(ctx, run)
	}

	if s.logFn != nil {
		s.logFn(fmt.Sprintf("Validation run %s completed with status: %s", s.currentRunID, status))
	}

	// Reset tracking
	s.currentRunID = ""
	s.itemsSample = []model.ValidationItem{}
	s.errorsSample = []model.ErrorSample{}
}

// recordValidationItem adds a validation item to the sample.
func (s *ValidationService) recordValidationItem(mb model.Mailbox, status string, cmra string, rdi string, errMsg string) {
	if len(s.itemsSample) >= s.maxSampleSize {
		return
	}

	item := model.ValidationItem{
		MailboxID: mb.ID,
		Name:      mb.Name,
		Address:   fmt.Sprintf("%s, %s, %s %s", mb.AddressRaw.Street, mb.AddressRaw.City, mb.AddressRaw.State, mb.AddressRaw.Zip),
		Status:    status,
		Error:     errMsg,
		CMRA:      cmra,
		RDI:       rdi,
	}

	s.itemsSample = append(s.itemsSample, item)
}

// recordError adds an error to the sample.
func (s *ValidationService) recordError(link string, reason string) {
	if len(s.errorsSample) >= s.maxSampleSize {
		return
	}

	errorSample := model.ErrorSample{
		Link:   link,
		Reason: reason,
	}

	s.errorsSample = append(s.errorsSample, errorSample)
}
