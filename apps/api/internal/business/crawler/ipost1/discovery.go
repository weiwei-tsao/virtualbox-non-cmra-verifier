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

// ProcessAndValidate discovers all iPost1 locations using aggressive scraping strategy.
// Sets ValidationStatus="pending" for new/changed records - validation happens separately.
func ProcessAndValidate(
	ctx context.Context,
	store MailboxStore,
	runID string,
	logFn func(string),
) (Stats, error) {
	stats := Stats{}

	// PHASE 1: Discover all locations
	discovered, err := DiscoverAll(ctx, logFn)
	if err != nil {
		return stats, fmt.Errorf("discovery failed: %w", err)
	}

	stats.Found = len(discovered)

	if stats.Found == 0 {
		return stats, fmt.Errorf("no locations discovered")
	}

	// PHASE 2: Pre-check which locations are new (aggressive scraping strategy)
	existing, err := store.FetchAllMetadata(ctx)
	if err != nil {
		return stats, fmt.Errorf("failed to fetch existing mailboxes: %w", err)
	}

	// Build map of discovered locations by link for quick lookup
	discoveredMap := make(map[string]model.Mailbox)
	for _, mb := range discovered {
		discoveredMap[mb.Link] = mb
	}

	// Track which links to process (new or changed)
	var toProcess []model.Mailbox
	seenLinks := make(map[string]bool)

	for _, mb := range discovered {
		seenLinks[mb.Link] = true

		// Clean address data
		mb.AddressRaw = util.CleanAddress(mb.AddressRaw)
		mb.Link = util.CleanLink(mb.Link)

		// Set metadata fields
		mb.CrawlRunID = runID
		mb.Active = true
		mb.Source = "iPost1"
		mb.DataHash = hashMailbox(mb)

		// Set validation status to "pending" - validation happens separately
		mb.ValidationStatus = "pending"
		mb.ValidationPriority = "high" // New addresses are high priority

		// Check if already exists with same data
		if prev, ok := existing[mb.Link]; ok {
			if prev.DataHash == mb.DataHash && prev.CMRA != "" {
				// Skip - already validated and unchanged
				stats.Skipped++
				continue
			}
			// Preserve ID for updates
			mb.ID = prev.ID

			// If data unchanged but CMRA exists, preserve validation data
			if prev.DataHash == mb.DataHash {
				mb.ValidationStatus = prev.ValidationStatus
				mb.ValidationPriority = prev.ValidationPriority
				mb.CMRA = prev.CMRA
				mb.RDI = prev.RDI
				mb.LastValidatedAt = prev.LastValidatedAt
			}
		}

		toProcess = append(toProcess, mb)
		stats.Updated++
	}

	if logFn != nil {
		logFn(fmt.Sprintf("PreCheck: new/changed=%d, skipped=%d", len(toProcess), stats.Skipped))
	}

	// PHASE 3: Write new/changed records to DB
	const batchSize = 20
	var toSave []model.Mailbox

	for i, mb := range toProcess {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		toSave = append(toSave, mb)

		// Incremental write
		if len(toSave) >= batchSize {
			if err := store.BatchUpsert(ctx, toSave); err != nil {
				return stats, fmt.Errorf("batch upsert failed: %w", err)
			}
			if logFn != nil {
				logFn(fmt.Sprintf("wrote %d items to DB (%d/%d processed)", len(toSave), i+1, len(toProcess)))
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

	// PHASE 4: Mark-and-sweep - soft delete records no longer at source
	var deletedIDs []string
	for link, prev := range existing {
		if prev.Source == "iPost1" && !seenLinks[link] {
			deletedIDs = append(deletedIDs, prev.ID)
		}
	}

	if len(deletedIDs) > 0 {
		if err := store.BulkSetActive(ctx, deletedIDs, false); err != nil {
			return stats, fmt.Errorf("bulk set active: %w", err)
		}
		if logFn != nil {
			logFn(fmt.Sprintf("marked %d records as inactive (deleted from source)", len(deletedIDs)))
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
	ValidateMailboxBatch(ctx context.Context, mailboxes []model.Mailbox) ([]model.Mailbox, error)
}

// MailboxStore interface for database operations.
type MailboxStore interface {
	FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error)
	FetchAllMetadata(ctx context.Context) (map[string]model.Mailbox, error)
	BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error
	BulkSetActive(ctx context.Context, ids []string, active bool) error
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
