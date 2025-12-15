package crawler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/util"
)

// ReprocessOptions configures how reprocessing is performed.
type ReprocessOptions struct {
	TargetVersion   string    // Target parser version (defaults to CurrentParserVersion)
	OnlyOutdated    bool      // Only reprocess records with different parser version
	ForceRevalidate bool      // Force Smarty re-validation even if DataHash unchanged (useful when switching from mock to real API)
	SinceTime       time.Time // Only reprocess records updated after this time
	BatchSize       int       // Number of records to process per batch (defaults to 100)
}

// ReprocessStats tracks progress of reprocessing operation.
type ReprocessStats struct {
	Total      int // Total records found
	Processed  int // Records successfully reprocessed
	Skipped    int // Records skipped (no HTML or version match)
	Failed     int // Records that failed parsing
	NoHTML     int // Records without RawHTML field
	UpToDate   int // Records already at target version
}

// ReprocessFromDB re-parses mailboxes from stored RawHTML without re-fetching.
// This is useful when parser logic changes and you need to update all records.
// Uses batch validation to reduce API calls by up to 99%.
func ReprocessFromDB(
	ctx context.Context,
	store MailboxStore,
	smarty ValidationClient,
	opts ReprocessOptions,
	logFn func(string),
	onProgress func(ReprocessStats),
) (ReprocessStats, error) {
	// Set defaults
	if opts.TargetVersion == "" {
		opts.TargetVersion = CurrentParserVersion
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}

	stats := ReprocessStats{}

	// Fetch all mailboxes from DB
	existing, err := store.FetchAllMap(ctx)
	if err != nil {
		return stats, fmt.Errorf("fetch mailboxes: %w", err)
	}

	stats.Total = len(existing)
	if logFn != nil {
		logFn(fmt.Sprintf("reprocessing %d mailboxes to version %s", stats.Total, opts.TargetVersion))
	}

	var toUpdate []model.Mailbox
	var toValidateIndices []int // Track indices that need validation
	const incrementalWriteThreshold = 20 // Write to DB every 20 items (reduced due to RawHTML size)

	for link, mb := range existing {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		// Skip if no raw HTML available
		if mb.RawHTML == "" {
			stats.NoHTML++
			stats.Skipped++
			if logFn != nil && stats.NoHTML <= 3 {
				logFn(fmt.Sprintf("skipping %s (no raw HTML)", link))
			}
			continue
		}

		// Skip if already at target version and OnlyOutdated is true
		if opts.OnlyOutdated && mb.ParserVersion == opts.TargetVersion {
			stats.UpToDate++
			stats.Skipped++
			continue
		}

		// Skip if updated before SinceTime
		if !opts.SinceTime.IsZero() && mb.LastValidatedAt.Before(opts.SinceTime) {
			stats.Skipped++
			continue
		}

		// Re-parse from stored HTML
		reparsed, err := ParseMailboxHTML(strings.NewReader(mb.RawHTML), link)
		if err != nil {
			stats.Failed++
			if logFn != nil {
				logFn(fmt.Sprintf("parse error for %s: %v", link, err))
			}
			continue
		}

		// Preserve original metadata and update parsed fields
		reparsed.ID = mb.ID
		reparsed.Source = mb.Source // Preserve original source (ATMB or iPost1)
		reparsed.Link = link
		reparsed.RawHTML = mb.RawHTML // Keep original HTML
		reparsed.CrawlRunID = mb.CrawlRunID
		reparsed.Active = mb.Active
		reparsed.DataHash = util.HashMailboxKey(reparsed.Name, reparsed.AddressRaw)
		reparsed.ParserVersion = opts.TargetVersion
		reparsed.LastParsedAt = time.Now()

		// Re-validate with Smarty if:
		// 1. Data changed (DataHash different), OR
		// 2. ForceRevalidate option is enabled (useful when switching from mock to real API)
		needsRevalidation := reparsed.DataHash != mb.DataHash || opts.ForceRevalidate

		if !needsRevalidation {
			// Keep existing validation if data unchanged and not forcing revalidation
			reparsed.CMRA = mb.CMRA
			reparsed.RDI = mb.RDI
			reparsed.StandardizedAddress = mb.StandardizedAddress
			reparsed.LastValidatedAt = mb.LastValidatedAt
		}

		toUpdate = append(toUpdate, reparsed)
		if needsRevalidation && smarty != nil {
			toValidateIndices = append(toValidateIndices, len(toUpdate)-1)
		}
		stats.Processed++

		// Incremental write with batch validation: flush to DB every N items
		if len(toUpdate) >= incrementalWriteThreshold {
			// Batch validate before writing
			if len(toValidateIndices) > 0 && smarty != nil {
				toUpdate = reprocessBatchValidate(ctx, smarty, toUpdate, toValidateIndices, logFn)
				toValidateIndices = toValidateIndices[:0]
			}

			if err := store.BatchUpsert(ctx, toUpdate); err != nil {
				if logFn != nil {
					logFn(fmt.Sprintf("batch upsert error: %v", err))
				}
				return stats, fmt.Errorf("batch upsert: %w", err)
			}
			if logFn != nil {
				logFn(fmt.Sprintf("wrote %d items to DB (incremental)", len(toUpdate)))
			}
			toUpdate = toUpdate[:0] // Clear slice

			if onProgress != nil {
				onProgress(stats)
			}
		}
	}

	// Final write with batch validation: flush remaining items
	if len(toUpdate) > 0 {
		// Batch validate remaining items
		if len(toValidateIndices) > 0 && smarty != nil {
			toUpdate = reprocessBatchValidate(ctx, smarty, toUpdate, toValidateIndices, logFn)
		}

		if err := store.BatchUpsert(ctx, toUpdate); err != nil {
			if logFn != nil {
				logFn(fmt.Sprintf("final batch upsert error: %v", err))
			}
			return stats, fmt.Errorf("batch upsert: %w", err)
		}
		if logFn != nil {
			logFn(fmt.Sprintf("wrote final %d items to DB", len(toUpdate)))
		}
	}

	if logFn != nil {
		logFn(fmt.Sprintf("reprocessing complete: processed=%d, skipped=%d (noHTML=%d, upToDate=%d), failed=%d",
			stats.Processed, stats.Skipped, stats.NoHTML, stats.UpToDate, stats.Failed))
	}

	return stats, nil
}

// reprocessBatchValidate validates a subset of mailboxes by their indices using batch API.
func reprocessBatchValidate(
	ctx context.Context,
	validator ValidationClient,
	mailboxes []model.Mailbox,
	indices []int,
	logFn func(string),
) []model.Mailbox {
	if len(indices) == 0 {
		return mailboxes
	}

	// Extract subset to validate
	subset := make([]model.Mailbox, len(indices))
	for i, idx := range indices {
		subset[i] = mailboxes[idx]
	}

	// Batch validate
	validated, err := validator.ValidateMailboxBatch(ctx, subset)
	if err != nil {
		if logFn != nil {
			logFn(fmt.Sprintf("batch validation failed for %d items: %v", len(indices), err))
		}
		return mailboxes
	}

	// Merge results back
	now := time.Now()
	for i, idx := range indices {
		mailboxes[idx] = validated[i]
		mailboxes[idx].LastValidatedAt = now
	}

	return mailboxes
}
