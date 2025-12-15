package crawler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/util"
)

// CurrentParserVersion tracks the parser logic version for reprocessing support.
const CurrentParserVersion = "v1.1"

// HTMLFetcher abstracts how pages are fetched so we can test the scraper without network calls.
type HTMLFetcher interface {
	Fetch(ctx context.Context, url string) (io.ReadCloser, error)
}

// MailboxStore abstracts the persistence layer for mailboxes.
type MailboxStore interface {
	FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error)
	FetchAllMetadata(ctx context.Context) (map[string]model.Mailbox, error)
	BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error
}

// ScrapeStats records counters for a scrape execution.
type ScrapeStats struct {
	Found     int
	Skipped   int
	Updated   int
	Validated int
	Failed    int
}

// ScrapeAndUpsert runs the scrape pipeline: fetch pages, parse, hash, compare, and batch upsert.
// Uses batch validation to reduce API calls by up to 99%.
func ScrapeAndUpsert(
	ctx context.Context,
	fetcher HTMLFetcher,
	store MailboxStore,
	validator ValidationClient,
	links []string,
	runID string,
	onProgress func(ScrapeStats),
	logFn func(string),
) (ScrapeStats, error) {
	stats := ScrapeStats{Found: len(links)}

	// Use FetchAllMetadata for deduplication (90% faster, excludes RawHTML)
	existing, err := store.FetchAllMetadata(ctx)
	if err != nil {
		return stats, fmt.Errorf("fetch existing mailboxes: %w", err)
	}

	var toSave []model.Mailbox
	var toValidateIndices []int // Track indices that need validation
	const incrementalWriteThreshold = 20 // Write to DB every 20 items (reduced due to RawHTML size)

	for _, link := range links {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}
		body, err := fetcher.Fetch(ctx, link)
		if err != nil {
			stats.Failed++
			if logFn != nil {
				logFn(fmt.Sprintf("fetch %s error: %v", link, err))
			}
			if onProgress != nil {
				onProgress(stats)
			}
			continue
		}

		// Read HTML into memory for both parsing and storage
		htmlBytes, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			stats.Failed++
			if logFn != nil {
				logFn(fmt.Sprintf("read %s error: %v", link, err))
			}
			if onProgress != nil {
				onProgress(stats)
			}
			continue
		}

		// Parse HTML from bytes
		parsed, err := ParseMailboxHTML(bytes.NewReader(htmlBytes), link)
		if err != nil {
			stats.Failed++
			if logFn != nil {
				logFn(fmt.Sprintf("parse %s error: %v", link, err))
			}
			if onProgress != nil {
				onProgress(stats)
			}
			continue
		}

		// Set metadata fields
		parsed.Source = "ATMB" // Mark as ATMB source
		parsed.DataHash = util.HashMailboxKey(parsed.Name, parsed.AddressRaw)
		if parsed.Link == "" {
			parsed.Link = link
		}
		parsed.CrawlRunID = runID
		parsed.Active = true

		// Save raw HTML for reprocessing support
		parsed.RawHTML = string(htmlBytes)
		parsed.ParserVersion = CurrentParserVersion
		parsed.LastParsedAt = time.Now()

		if prev, ok := existing[parsed.Link]; ok {
			if prev.DataHash == parsed.DataHash && prev.CMRA != "" {
				stats.Skipped++
				continue
			}
			// Preserve IDs so updates target existing docs.
			parsed.ID = prev.ID
		}

		// Track if validation needed (CMRA/RDI are always empty after HTML parsing)
		needsValidation := parsed.CMRA == "" || parsed.RDI == ""

		toSave = append(toSave, parsed)
		if needsValidation && validator != nil {
			toValidateIndices = append(toValidateIndices, len(toSave)-1)
		}
		stats.Updated++

		// Incremental write with batch validation: flush to DB every N items
		if len(toSave) >= incrementalWriteThreshold {
			// Batch validate before writing
			if len(toValidateIndices) > 0 && validator != nil {
				toSave, stats = batchValidateSubset(ctx, validator, toSave, toValidateIndices, stats, logFn)
				toValidateIndices = toValidateIndices[:0]
			}

			if err := store.BatchUpsert(ctx, toSave); err != nil {
				if logFn != nil {
					logFn(fmt.Sprintf("incremental batch upsert error: %v", err))
				}
				return stats, fmt.Errorf("batch upsert: %w", err)
			}
			if logFn != nil {
				logFn(fmt.Sprintf("wrote %d items to DB (incremental)", len(toSave)))
			}
			toSave = toSave[:0] // Clear slice but keep capacity
		}

		if onProgress != nil {
			onProgress(stats)
		}
	}

	// Final write with batch validation: flush any remaining items
	if len(toSave) > 0 {
		// Batch validate remaining items
		if len(toValidateIndices) > 0 && validator != nil {
			toSave, stats = batchValidateSubset(ctx, validator, toSave, toValidateIndices, stats, logFn)
		}

		if err := store.BatchUpsert(ctx, toSave); err != nil {
			if logFn != nil {
				logFn(fmt.Sprintf("final batch upsert error: %v", err))
			}
			return stats, fmt.Errorf("batch upsert: %w", err)
		}
		if logFn != nil {
			logFn(fmt.Sprintf("wrote final %d items to DB", len(toSave)))
		}
	}
	return stats, nil
}

// batchValidateSubset validates a subset of mailboxes by their indices using batch API.
func batchValidateSubset(
	ctx context.Context,
	validator ValidationClient,
	mailboxes []model.Mailbox,
	indices []int,
	stats ScrapeStats,
	logFn func(string),
) ([]model.Mailbox, ScrapeStats) {
	if len(indices) == 0 {
		return mailboxes, stats
	}

	// Extract subset to validate
	subset := make([]model.Mailbox, len(indices))
	for i, idx := range indices {
		subset[i] = mailboxes[idx]
	}

	// Batch validate
	validated, err := validator.ValidateMailboxBatch(ctx, subset)
	if err != nil {
		// On error, count all as failed
		stats.Failed += len(indices)
		if logFn != nil {
			logFn(fmt.Sprintf("batch validation failed for %d items: %v", len(indices), err))
		}
		return mailboxes, stats
	}

	// Merge results back
	for i, idx := range indices {
		mailboxes[idx] = validated[i]
		if validated[i].CMRA != "" {
			stats.Validated++
		} else {
			stats.Failed++
		}
	}

	return mailboxes, stats
}
