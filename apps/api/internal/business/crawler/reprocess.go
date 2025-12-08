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
	TargetVersion string    // Target parser version (defaults to CurrentParserVersion)
	OnlyOutdated  bool      // Only reprocess records with different parser version
	SinceTime     time.Time // Only reprocess records updated after this time
	BatchSize     int       // Number of records to process per batch (defaults to 100)
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
	const incrementalWriteThreshold = 100

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
		reparsed.Link = link
		reparsed.RawHTML = mb.RawHTML // Keep original HTML
		reparsed.CrawlRunID = mb.CrawlRunID
		reparsed.Active = mb.Active
		reparsed.DataHash = util.HashMailboxKey(reparsed.Name, reparsed.AddressRaw)
		reparsed.ParserVersion = opts.TargetVersion
		reparsed.LastParsedAt = time.Now()

		// Re-validate with Smarty if available and data changed
		if smarty != nil && reparsed.DataHash != mb.DataHash {
			validated, err := smarty.ValidateMailbox(ctx, reparsed)
			if err == nil {
				reparsed = validated
				reparsed.LastValidatedAt = time.Now()
			} else if logFn != nil {
				logFn(fmt.Sprintf("smarty validation failed for %s: %v", link, err))
			}
		} else {
			// Keep existing validation if data unchanged
			reparsed.CMRA = mb.CMRA
			reparsed.RDI = mb.RDI
			reparsed.StandardizedAddress = mb.StandardizedAddress
			reparsed.LastValidatedAt = mb.LastValidatedAt
		}

		toUpdate = append(toUpdate, reparsed)
		stats.Processed++

		// Incremental write: flush to DB every N items
		if len(toUpdate) >= incrementalWriteThreshold {
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

	// Final write: flush remaining items
	if len(toUpdate) > 0 {
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
