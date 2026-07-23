package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// RevalidationChecker identifies mailboxes that need re-validation based on age.
type RevalidationChecker struct {
	repository MailboxRepository
	config     model.CrawlerConfig
	logFn      func(string)
}

// NewRevalidationChecker creates a new revalidation checker.
func NewRevalidationChecker(repository MailboxRepository, config model.CrawlerConfig, logFn func(string)) *RevalidationChecker {
	return &RevalidationChecker{
		repository: repository,
		config:     config,
		logFn:      logFn,
	}
}

// RevalidationStats tracks revalidation check results.
type RevalidationStats struct {
	Checked             int // Total validated mailboxes checked
	NeedsRevalidation   int // Mailboxes >90 days old (marked needs_revalidation)
	ApproachingThreshold int // Mailboxes 70-89 days old (priority upgraded to medium)
	UpToDate            int // Mailboxes <70 days old (no action)
}

// CheckRevalidationNeeded scans validated mailboxes and marks those needing re-validation.
//
// Logic:
//  - >90 days old: Set status="needs_revalidation", priority="high"
//  - 70-89 days old: Keep status="validated", upgrade priority to "medium"
//  - <70 days old: Keep status="validated", priority="low"
//
// This should run daily (e.g., via cron job or background worker).
func (rc *RevalidationChecker) CheckRevalidationNeeded(ctx context.Context) (RevalidationStats, error) {
	stats := RevalidationStats{}

	if rc.logFn != nil {
		rc.logFn("Starting revalidation check...")
	}

	// Calculate threshold dates
	now := time.Now()
	revalidationThreshold := now.Add(-rc.config.RevalidationInterval)  // 90 days ago
	mediumPriorityThreshold := now.Add(-rc.config.RevalidationThreshold) // 70 days ago

	if rc.logFn != nil {
		rc.logFn(fmt.Sprintf("Revalidation thresholds - Full: %s, Medium: %s",
			revalidationThreshold.Format("2006-01-02"),
			mediumPriorityThreshold.Format("2006-01-02")))
	}

	// Fetch all validated mailboxes
	// Note: In production, this might need to be paginated for very large datasets
	validated, err := rc.repository.FetchByValidationStatus(ctx, "validated", "", 10000)
	if err != nil {
		return stats, fmt.Errorf("fetch validated mailboxes: %w", err)
	}

	stats.Checked = len(validated)

	if stats.Checked == 0 {
		if rc.logFn != nil {
			rc.logFn("No validated mailboxes to check")
		}
		return stats, nil
	}

	// Process each mailbox
	var toUpdate []model.Mailbox

	for _, mb := range validated {
		// Skip if never validated (shouldn't happen, but defensive)
		if mb.LastValidatedAt.IsZero() {
			continue
		}

		if mb.LastValidatedAt.Before(revalidationThreshold) {
			// >90 days old - needs revalidation
			mb.ValidationStatus = "needs_revalidation"
			mb.ValidationPriority = "high"
			toUpdate = append(toUpdate, mb)
			stats.NeedsRevalidation++

		} else if mb.LastValidatedAt.Before(mediumPriorityThreshold) {
			// 70-89 days old - approaching threshold, upgrade priority
			if mb.ValidationPriority != "medium" {
				mb.ValidationPriority = "medium"
				toUpdate = append(toUpdate, mb)
			}
			stats.ApproachingThreshold++

		} else {
			// <70 days old - still fresh
			if mb.ValidationPriority != "low" {
				mb.ValidationPriority = "low"
				toUpdate = append(toUpdate, mb)
			}
			stats.UpToDate++
		}

		// Log progress every 1000 records
		if (stats.NeedsRevalidation+stats.ApproachingThreshold+stats.UpToDate)%1000 == 0 && rc.logFn != nil {
			rc.logFn(fmt.Sprintf("Checked %d/%d mailboxes...",
				stats.NeedsRevalidation+stats.ApproachingThreshold+stats.UpToDate, stats.Checked))
		}
	}

	// Batch update mailboxes that need status/priority changes
	if len(toUpdate) > 0 {
		if rc.logFn != nil {
			rc.logFn(fmt.Sprintf("Updating %d mailboxes with new status/priority", len(toUpdate)))
		}

		if err := rc.repository.BatchUpsert(ctx, toUpdate); err != nil {
			return stats, fmt.Errorf("batch update: %w", err)
		}
	}

	if rc.logFn != nil {
		rc.logFn(fmt.Sprintf("Revalidation check complete - Needs revalidation: %d, Approaching: %d, Up-to-date: %d",
			stats.NeedsRevalidation, stats.ApproachingThreshold, stats.UpToDate))
	}

	return stats, nil
}

// GetRevalidationSummary returns a summary of mailboxes by validation age.
// Useful for monitoring and dashboards.
func (rc *RevalidationChecker) GetRevalidationSummary(ctx context.Context) (string, error) {
	validated, err := rc.repository.FetchByValidationStatus(ctx, "validated", "", 10000)
	if err != nil {
		return "", fmt.Errorf("fetch validated mailboxes: %w", err)
	}

	now := time.Now()
	revalidationThreshold := now.Add(-rc.config.RevalidationInterval)
	mediumPriorityThreshold := now.Add(-rc.config.RevalidationThreshold)

	var oldCount, approachingCount, freshCount int

	for _, mb := range validated {
		if mb.LastValidatedAt.IsZero() {
			continue
		}

		if mb.LastValidatedAt.Before(revalidationThreshold) {
			oldCount++
		} else if mb.LastValidatedAt.Before(mediumPriorityThreshold) {
			approachingCount++
		} else {
			freshCount++
		}
	}

	return fmt.Sprintf(
		"Validation Age Summary:\n"+
			"  >90 days (needs revalidation): %d\n"+
			"  70-89 days (approaching): %d\n"+
			"  <70 days (fresh): %d\n"+
			"  Total validated: %d",
		oldCount, approachingCount, freshCount, len(validated),
	), nil
}
