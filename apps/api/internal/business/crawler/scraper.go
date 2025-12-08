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
const CurrentParserVersion = "v1.0"

// HTMLFetcher abstracts how pages are fetched so we can test the scraper without network calls.
type HTMLFetcher interface {
	Fetch(ctx context.Context, url string) (io.ReadCloser, error)
}

// MailboxStore abstracts the persistence layer for mailboxes.
type MailboxStore interface {
	FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error)
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

	existing, err := store.FetchAllMap(ctx)
	if err != nil {
		return stats, fmt.Errorf("fetch existing mailboxes: %w", err)
	}

	var toSave []model.Mailbox
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

		needsValidation := true
		if parsed.CMRA != "" && parsed.RDI != "" {
			needsValidation = false
		}

		if needsValidation && validator != nil {
			validated, err := validator.ValidateMailbox(ctx, parsed)
			if err != nil {
				stats.Failed++
				if logFn != nil {
					logFn(fmt.Sprintf("validate %s error: %v", link, err))
				}
			} else {
				parsed = validated
				stats.Validated++
			}
		}

		toSave = append(toSave, parsed)
		stats.Updated++

		// Incremental write: flush to DB every N items to prevent data loss
		if len(toSave) >= incrementalWriteThreshold {
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

	// Final write: flush any remaining items
	if len(toSave) > 0 {
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
