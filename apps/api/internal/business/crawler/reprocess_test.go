package crawler

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

func TestReprocessFromDB(t *testing.T) {
	// Read sample HTML
	sample, err := os.ReadFile("testdata/sample_page.html")
	if err != nil {
		t.Fatalf("read sample html: %v", err)
	}

	// Mock existing mailbox with old parser version and raw HTML
	existingMailbox := model.Mailbox{
		ID:   "existing-id",
		Link: "https://anytimemailbox.com/locations/chicago-monroe-st",
		Name: "OLD NAME", // Intentionally wrong - should be updated
		AddressRaw: model.AddressRaw{
			Street: "OLD STREET", // Intentionally wrong
			City:   "OLD CITY",
			State:  "XX",
			Zip:    "00000",
		},
		RawHTML:       string(sample),                 // Has raw HTML
		ParserVersion: "v0.9",                         // Old version
		LastParsedAt:  time.Now().Add(-24 * time.Hour), // Parsed 1 day ago
		Active:        true,
	}

	// Mock mailbox without raw HTML (should be skipped)
	noHTMLMailbox := model.Mailbox{
		ID:            "no-html-id",
		Link:          "https://anytimemailbox.com/locations/no-html",
		Name:          "No HTML Mailbox",
		RawHTML:       "", // No raw HTML
		ParserVersion: "v0.9",
		Active:        true,
	}

	// Mock mailbox already at target version (should be skipped if OnlyOutdated=true)
	upToDateMailbox := model.Mailbox{
		ID:   "up-to-date-id",
		Link: "https://anytimemailbox.com/locations/up-to-date",
		Name: "Up To Date Mailbox",
		AddressRaw: model.AddressRaw{
			Street: "123 Main St",
			City:   "Dover",
			State:  "DE",
			Zip:    "19901",
		},
		RawHTML:       string(sample),
		ParserVersion: "v1.0", // Already at target version
		LastParsedAt:  time.Now(),
		Active:        true,
	}

	store := &mockStore{
		existing: map[string]model.Mailbox{
			existingMailbox.Link: existingMailbox,
			noHTMLMailbox.Link:   noHTMLMailbox,
			upToDateMailbox.Link: upToDateMailbox,
		},
	}

	// Test reprocessing with OnlyOutdated=true
	opts := ReprocessOptions{
		TargetVersion: "v1.0",
		OnlyOutdated:  true,
		BatchSize:     100,
	}

	stats, err := ReprocessFromDB(context.Background(), store, nil, opts, nil, nil)
	if err != nil {
		t.Fatalf("ReprocessFromDB: %v", err)
	}

	// Verify stats
	if stats.Total != 3 {
		t.Errorf("total = %d, want 3", stats.Total)
	}
	if stats.Processed != 1 {
		t.Errorf("processed = %d, want 1 (only existingMailbox should be reprocessed)", stats.Processed)
	}
	if stats.NoHTML != 1 {
		t.Errorf("noHTML = %d, want 1 (noHTMLMailbox)", stats.NoHTML)
	}
	if stats.UpToDate != 1 {
		t.Errorf("upToDate = %d, want 1 (upToDateMailbox)", stats.UpToDate)
	}
	if stats.Skipped != 2 {
		t.Errorf("skipped = %d, want 2 (noHTMLMailbox + upToDateMailbox)", stats.Skipped)
	}

	// Verify saved mailbox was correctly reparsed
	if len(store.saved) != 1 {
		t.Fatalf("expected 1 saved mailbox, got %d", len(store.saved))
	}

	saved := store.saved[0]
	if saved.Name != "Chicago - Monroe St" {
		t.Errorf("reparsed name = %q, want %q", saved.Name, "Chicago - Monroe St")
	}
	if saved.AddressRaw.Street != "73 W Monroe St" {
		t.Errorf("reparsed street = %q, want %q", saved.AddressRaw.Street, "73 W Monroe St")
	}
	if saved.AddressRaw.City != "Chicago" {
		t.Errorf("reparsed city = %q, want %q", saved.AddressRaw.City, "Chicago")
	}
	if saved.ParserVersion != "v1.0" {
		t.Errorf("parserVersion = %q, want v1.0", saved.ParserVersion)
	}
	if saved.RawHTML == "" {
		t.Errorf("RawHTML should be preserved")
	}
	if saved.ID != existingMailbox.ID {
		t.Errorf("ID should be preserved: got %q, want %q", saved.ID, existingMailbox.ID)
	}
}

func TestReprocessFromDB_AllRecords(t *testing.T) {
	sample, err := os.ReadFile("testdata/sample_page.html")
	if err != nil {
		t.Fatalf("read sample html: %v", err)
	}

	upToDateMailbox := model.Mailbox{
		ID:   "up-to-date-id",
		Link: "https://anytimemailbox.com/locations/up-to-date",
		Name: "OLD NAME",
		AddressRaw: model.AddressRaw{
			Street: "OLD STREET",
		},
		RawHTML:       string(sample),
		ParserVersion: "v1.0", // Already at target version
		Active:        true,
	}

	store := &mockStore{
		existing: map[string]model.Mailbox{
			upToDateMailbox.Link: upToDateMailbox,
		},
	}

	// Test reprocessing with OnlyOutdated=false (should reprocess all)
	opts := ReprocessOptions{
		TargetVersion: "v1.0",
		OnlyOutdated:  false, // Reprocess ALL records
		BatchSize:     100,
	}

	stats, err := ReprocessFromDB(context.Background(), store, nil, opts, nil, nil)
	if err != nil {
		t.Fatalf("ReprocessFromDB: %v", err)
	}

	if stats.Processed != 1 {
		t.Errorf("processed = %d, want 1 (should reprocess even if version matches)", stats.Processed)
	}
	if stats.UpToDate != 0 {
		t.Errorf("upToDate = %d, want 0 (OnlyOutdated=false)", stats.UpToDate)
	}

	// Verify it was actually reparsed with correct data
	if len(store.saved) != 1 {
		t.Fatalf("expected 1 saved mailbox, got %d", len(store.saved))
	}
	saved := store.saved[0]
	if saved.Name != "Chicago - Monroe St" {
		t.Errorf("should reparse even if version matches: name = %q", saved.Name)
	}
}
