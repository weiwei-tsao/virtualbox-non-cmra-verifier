package ipost1

import (
	"context"
	"fmt"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/util"
)

// DiscoverAll fetches all mailbox locations across all US states/territories.
// Returns a slice of mailboxes ready for validation and storage.
func DiscoverAll(ctx context.Context, logFn func(string)) ([]model.Mailbox, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	if logFn != nil {
		logFn("fetching US states list...")
	}

	// Step 1: Get all states
	states, err := client.GetStates()
	if err != nil {
		return nil, fmt.Errorf("failed to get states: %w", err)
	}

	if logFn != nil {
		logFn(fmt.Sprintf("found %d states/territories", len(states)))
	}

	var allMailboxes []model.Mailbox

	// Step 2: Iterate through each state and get locations
	for i, state := range states {
		select {
		case <-ctx.Done():
			return allMailboxes, ctx.Err()
		default:
		}

		if logFn != nil {
			logFn(fmt.Sprintf("[%d/%d] processing %s (ID: %s)", i+1, len(states), state.Name, state.ID))
		}

		// Fetch locations for this state
		response, err := client.GetLocationsByState(state.ID)
		if err != nil {
			if logFn != nil {
				logFn(fmt.Sprintf("error fetching locations for %s: %v", state.Name, err))
			}
			continue
		}

		// Parse HTML to extract mailboxes
		mailboxes, err := ParseLocationsHTML(response.Display)
		if err != nil {
			if logFn != nil {
				logFn(fmt.Sprintf("error parsing locations for %s: %v", state.Name, err))
			}
			continue
		}

		if logFn != nil {
			logFn(fmt.Sprintf("  found %d locations in %s", len(mailboxes), state.Name))
		}

		allMailboxes = append(allMailboxes, mailboxes...)

		// Rate limiting: wait between states to avoid overwhelming the server
		if i < len(states)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	if logFn != nil {
		logFn(fmt.Sprintf("discovery complete: %d total locations found", len(allMailboxes)))
	}

	return allMailboxes, nil
}

// ProcessAndValidate discovers all iPost1 locations and validates them with Smarty.
// This is similar to ATMB's ScrapeAndUpsert but adapted for iPost1's data structure.
func ProcessAndValidate(
	ctx context.Context,
	validator ValidationClient,
	store MailboxStore,
	runID string,
	logFn func(string),
) (Stats, error) {
	stats := Stats{}

	// Discover all locations
	discovered, err := DiscoverAll(ctx, logFn)
	if err != nil {
		return stats, fmt.Errorf("discovery failed: %w", err)
	}

	stats.Found = len(discovered)

	if stats.Found == 0 {
		return stats, fmt.Errorf("no locations discovered")
	}

	// Fetch existing mailboxes for deduplication
	existing, err := store.FetchAllMetadata(ctx)
	if err != nil {
		return stats, fmt.Errorf("failed to fetch existing mailboxes: %w", err)
	}

	var toSave []model.Mailbox
	const batchSize = 20 // Write every 20 items

	for i, mb := range discovered {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		// Clean address data (remove HTML remnants from scraper)
		mb.AddressRaw = util.CleanAddress(mb.AddressRaw)
		mb.Link = util.CleanLink(mb.Link)

		// Set metadata fields
		mb.CrawlRunID = runID
		mb.Active = true
		mb.Source = "iPost1"

		// Generate unique hash for deduplication
		mb.DataHash = hashMailbox(mb)

		// Check if already exists with same data
		if prev, ok := existing[mb.Link]; ok {
			if prev.DataHash == mb.DataHash && prev.CMRA != "" {
				stats.Skipped++
				continue
			}
			// Preserve ID for updates
			mb.ID = prev.ID
		}

		// Validate with Smarty if needed
		if validator != nil && (mb.CMRA == "" || mb.RDI == "") {
			validated, err := validator.ValidateMailbox(ctx, mb)
			if err != nil {
				stats.Failed++
				if logFn != nil {
					logFn(fmt.Sprintf("validation failed for %s: %v", mb.Name, err))
				}
			} else {
				mb = validated
				stats.Validated++
			}
		}

		toSave = append(toSave, mb)
		stats.Updated++

		// Incremental write
		if len(toSave) >= batchSize {
			if err := store.BatchUpsert(ctx, toSave); err != nil {
				return stats, fmt.Errorf("batch upsert failed: %w", err)
			}
			if logFn != nil {
				logFn(fmt.Sprintf("wrote %d items to DB (%d/%d processed)", len(toSave), i+1, stats.Found))
			}
			toSave = toSave[:0]
		}
	}

	// Final write
	if len(toSave) > 0 {
		if err := store.BatchUpsert(ctx, toSave); err != nil {
			return stats, fmt.Errorf("final batch upsert failed: %w", err)
		}
		if logFn != nil {
			logFn(fmt.Sprintf("wrote final %d items to DB", len(toSave)))
		}
	}

	return stats, nil
}

// Stats tracks the progress of iPost1 crawl.
type Stats struct {
	Found     int
	Updated   int
	Skipped   int
	Validated int
	Failed    int
}

// ValidationClient interface for Smarty API validation.
type ValidationClient interface {
	ValidateMailbox(ctx context.Context, mb model.Mailbox) (model.Mailbox, error)
}

// MailboxStore interface for database operations.
type MailboxStore interface {
	FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error)
	FetchAllMetadata(ctx context.Context) (map[string]model.Mailbox, error)
	BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error
}

// hashMailbox creates a unique hash for deduplication.
func hashMailbox(mb model.Mailbox) string {
	// Simple hash based on name and address
	key := fmt.Sprintf("%s|%s|%s|%s|%s",
		mb.Name,
		mb.AddressRaw.Street,
		mb.AddressRaw.City,
		mb.AddressRaw.State,
		mb.AddressRaw.Zip,
	)

	// Use a simple hash for now (you can import util.HashMailboxKey if available)
	return fmt.Sprintf("%x", []byte(key))
}
