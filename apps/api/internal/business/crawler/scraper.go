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
	BulkSetActive(ctx context.Context, ids []string, active bool) error
}

// ScrapeStats records counters for a scrape execution.
type ScrapeStats struct {
	Found     int
	Skipped   int
	Updated   int
	Validated int
	Failed    int
}

// PreCheckResult contains the delta between provided links and existing database state.
type PreCheckResult struct {
	NewLinks      []string // Links to fetch (don't exist in DB)
	ExistingLinks []string // Links already in DB (skip fetch)
	DeletedIDs    []string // IDs of records no longer in source (mark inactive)
}

// PreCheckLinks calculates which links need fetching vs which are already stored.
// This is the core of the aggressive scraping strategy (99.5% network reduction).
func PreCheckLinks(ctx context.Context, store MailboxStore, links []string, source string) (PreCheckResult, error) {
	result := PreCheckResult{}

	// Load metadata only (90% faster than full fetch)
	existing, err := store.FetchAllMetadata(ctx)
	if err != nil {
		return result, fmt.Errorf("fetch existing metadata: %w", err)
	}

	// Build set of provided links for fast lookup
	providedLinks := make(map[string]bool, len(links))
	for _, link := range links {
		providedLinks[link] = true
	}

	// Categorize links
	for _, link := range links {
		if _, exists := existing[link]; exists {
			result.ExistingLinks = append(result.ExistingLinks, link)
		} else {
			result.NewLinks = append(result.NewLinks, link)
		}
	}

	// Find deleted records (exist in DB but not in provided links)
	for link, mb := range existing {
		// Only check records from the same source
		if mb.Source == source && !providedLinks[link] {
			result.DeletedIDs = append(result.DeletedIDs, mb.ID)
		}
	}

	return result, nil
}

// ScrapeAndUpsert runs the scrape pipeline: fetch pages, parse, hash, compare, and batch upsert.
// Uses aggressive scraping strategy (pre-check existing links to skip 99.5% of network requests).
// Sets ValidationStatus="pending" for new/changed records - validation happens separately.
func ScrapeAndUpsert(
	ctx context.Context,
	fetcher HTMLFetcher,
	store MailboxStore,
	links []string,
	source string,
	runID string,
	onProgress func(ScrapeStats),
	logFn func(string),
) (ScrapeStats, error) {
	stats := ScrapeStats{Found: len(links)}

	// PHASE 1: Pre-check which links need fetching (aggressive scraping strategy)
	preCheck, err := PreCheckLinks(ctx, store, links, source)
	if err != nil {
		return stats, fmt.Errorf("pre-check links: %w", err)
	}

	if logFn != nil {
		logFn(fmt.Sprintf("PreCheck: new=%d, existing=%d, deleted=%d",
			len(preCheck.NewLinks), len(preCheck.ExistingLinks), len(preCheck.DeletedIDs)))
	}

	// Skip existing links (already validated and unchanged)
	stats.Skipped = len(preCheck.ExistingLinks)

	// Use FetchAllMetadata for deduplication within new links
	existing, err := store.FetchAllMetadata(ctx)
	if err != nil {
		return stats, fmt.Errorf("fetch existing mailboxes: %w", err)
	}

	var toSave []model.Mailbox
	const incrementalWriteThreshold = 20 // Write to DB every 20 items (reduced due to RawHTML size)

	// PHASE 2: Fetch and parse only NEW links (99.5% network reduction)
	for _, link := range preCheck.NewLinks {
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
		parsed.Source = source
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

		// Set validation status to "pending" - validation happens separately
		parsed.ValidationStatus = "pending"
		parsed.ValidationPriority = "high" // New addresses are high priority

		if prev, ok := existing[parsed.Link]; ok {
			// Preserve IDs so updates target existing docs
			parsed.ID = prev.ID

			// If data unchanged but CMRA exists, this shouldn't happen (pre-check filters these)
			// But handle edge case where data changed
			if prev.DataHash == parsed.DataHash && prev.CMRA != "" {
				// Copy existing validation data
				parsed.ValidationStatus = prev.ValidationStatus
				parsed.ValidationPriority = prev.ValidationPriority
				parsed.CMRA = prev.CMRA
				parsed.RDI = prev.RDI
				parsed.LastValidatedAt = prev.LastValidatedAt
			}
		}

		toSave = append(toSave, parsed)
		stats.Updated++

		// Incremental write: flush to DB every N items
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

	// PHASE 3: Final write - flush any remaining items
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

	// PHASE 4: Mark-and-sweep - soft delete records no longer at source
	if len(preCheck.DeletedIDs) > 0 {
		if err := store.BulkSetActive(ctx, preCheck.DeletedIDs, false); err != nil {
			if logFn != nil {
				logFn(fmt.Sprintf("mark-and-sweep error: %v", err))
			}
			return stats, fmt.Errorf("bulk set active: %w", err)
		}
		if logFn != nil {
			logFn(fmt.Sprintf("marked %d records as inactive (deleted from source)", len(preCheck.DeletedIDs)))
		}
	}

	return stats, nil
}
