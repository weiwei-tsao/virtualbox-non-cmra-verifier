package crawler

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/util"
)

type mockFetcher struct {
	html   []byte
	perURL map[string][]byte
	err    error
}

func (m mockFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	if buf, ok := m.perURL[url]; ok {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}
	return io.NopCloser(bytes.NewReader(m.html)), nil
}

type mockStore struct {
	existing map[string]model.Mailbox
	saved    []model.Mailbox
}

func (m *mockStore) FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error) {
	return m.existing, nil
}

func (m *mockStore) FetchAllMetadata(ctx context.Context) (map[string]model.Mailbox, error) {
	// Mock returns full data for simplicity in tests
	return m.existing, nil
}

func (m *mockStore) BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error {
	m.saved = append(m.saved, mailboxes...)
	return nil
}

func TestScrapeAndUpsert(t *testing.T) {
	sample, err := os.ReadFile("testdata/sample_page.html")
	if err != nil {
		t.Fatalf("read sample html: %v", err)
	}

	// Existing mailbox with matching hash and CMRA should be skipped.
	existingMailbox := model.Mailbox{
		ID:   "existing-id",
		Link: "https://anytimemailbox.com/locations/chicago-monroe-st",
		Name: "Chicago - Monroe St",
		AddressRaw: model.AddressRaw{
			Street: "73 W Monroe St",
			City:   "Chicago",
			State:  "IL",
			Zip:    "60603",
		},
		CMRA:     "Y",
		DataHash: util.HashMailboxKey("Chicago - Monroe St", model.AddressRaw{Street: "73 W Monroe St", City: "Chicago", State: "IL", Zip: "60603"}),
	}

	store := &mockStore{
		existing: map[string]model.Mailbox{
			existingMailbox.Link: existingMailbox,
		},
	}

	fetcher := mockFetcher{html: sample}
	links := []string{
		existingMailbox.Link,
		"https://anytimemailbox.com/locations/new-mailbox-store",
	}

	// The first link returns sample HTML as-is (matching existing mailbox).
	// The second link returns the same HTML (parser uses sourceLink as the link field).
	fetcher.perURL = map[string][]byte{
		links[0]: sample,
		links[1]: sample,
	}

	stats, err := ScrapeAndUpsert(context.Background(), fetcher, store, nil, links, "RUN_1", nil, nil)
	if err != nil {
		t.Fatalf("ScrapeAndUpsert: %v", err)
	}

	if stats.Found != 2 || stats.Skipped != 1 || stats.Updated != 1 || stats.Validated != 0 || stats.Failed != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	if len(store.saved) != 1 {
		t.Fatalf("expected 1 saved mailbox, got %d", len(store.saved))
	}

	saved := store.saved[0]
	if saved.CrawlRunID != "RUN_1" {
		t.Errorf("CrawlRunID = %q", saved.CrawlRunID)
	}
	if saved.DataHash == "" {
		t.Errorf("DataHash should be set")
	}
	if saved.Link != links[1] {
		t.Errorf("saved link = %q, want %q", saved.Link, links[1])
	}
}
