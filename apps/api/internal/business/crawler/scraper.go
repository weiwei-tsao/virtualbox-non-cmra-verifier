package crawler

import (
	"context"
	"fmt"
	"io"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/util"
)

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
func ScrapeAndUpsert(ctx context.Context, fetcher HTMLFetcher, store MailboxStore, validator ValidationClient, links []string, runID string, onProgress func(ScrapeStats)) (ScrapeStats, error) {
	stats := ScrapeStats{Found: len(links)}

	existing, err := store.FetchAllMap(ctx)
	if err != nil {
		return stats, fmt.Errorf("fetch existing mailboxes: %w", err)
	}

	var toSave []model.Mailbox
	for _, link := range links {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}
		body, err := fetcher.Fetch(ctx, link)
		if err != nil {
			stats.Failed++
			if onProgress != nil {
				onProgress(stats)
			}
			continue
		}
		parsed, err := ParseMailboxHTML(body, link)
		body.Close()
		if err != nil {
			stats.Failed++
			if onProgress != nil {
				onProgress(stats)
			}
			continue
		}

		parsed.DataHash = util.HashMailboxKey(parsed.Name, parsed.AddressRaw)
		parsed.CrawlRunID = runID
		parsed.Active = true

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
			} else {
				parsed = validated
				stats.Validated++
			}
		}

		toSave = append(toSave, parsed)
		stats.Updated++

		if onProgress != nil {
			onProgress(stats)
		}
	}

	if err := store.BatchUpsert(ctx, toSave); err != nil {
		return stats, fmt.Errorf("batch upsert: %w", err)
	}
	return stats, nil
}
