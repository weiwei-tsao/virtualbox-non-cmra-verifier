package smarty

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestClientMock(t *testing.T) {
	c := New(http.DefaultClient, Config{Mock: true})
	m := model.Mailbox{
		AddressRaw: model.AddressRaw{
			Street: "123 Main",
			City:   "Dover",
			State:  "DE",
			Zip:    "19901",
		},
	}
	got, err := c.ValidateMailbox(context.Background(), m)
	if err != nil {
		t.Fatalf("ValidateMailbox mock: %v", err)
	}
	if got.CMRA == "" || got.RDI == "" {
		t.Fatalf("expected CMRA/RDI set in mock")
	}
	if got.StandardizedAddress.DeliveryLine1 == "" {
		t.Fatalf("expected standardized address in mock")
	}
}

func TestClientCircuitBreaker(t *testing.T) {
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString("429")),
		}, nil
	})
	c := New(rt, Config{Mock: false, AuthIDs: []string{"id"}, AuthTokens: []string{"token"}, BreakerMax: 2, MaxRetries: 1})

	_, err := c.ValidateMailbox(context.Background(), model.Mailbox{
		AddressRaw: model.AddressRaw{Street: "1", City: "2", State: "3", Zip: "4"},
	})
	if err == nil {
		t.Fatalf("expected error on 429")
	}
	if err != ErrAllCredentialsExhausted {
		// First call exhausts the single credential's breaker after 2 retries.
		// Since there's only one credential and it's exhausted, we get ErrAllCredentialsExhausted.
		_, err = c.ValidateMailbox(context.Background(), model.Mailbox{
			AddressRaw: model.AddressRaw{Street: "1", City: "2", State: "3", Zip: "4"},
		})
		if err != ErrAllCredentialsExhausted {
			t.Fatalf("expected all credentials exhausted, got %v", err)
		}
	}
}

func TestClientSuccess(t *testing.T) {
	// Match actual Smarty API response structure: dpv_cmra in analysis, rdi in metadata
	body := `[{"delivery_line_1":"123 Main","last_line":"Dover, DE 19901","metadata":{"rdi":"Commercial"},"analysis":{"dpv_cmra":"Y"}}]`
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	})
	c := New(rt, Config{Mock: false, AuthIDs: []string{"id"}, AuthTokens: []string{"token"}})
	got, err := c.ValidateMailbox(context.Background(), model.Mailbox{
		AddressRaw: model.AddressRaw{Street: "123 Main", City: "Dover", State: "DE", Zip: "19901"},
	})
	if err != nil {
		t.Fatalf("ValidateMailbox success: %v", err)
	}
	if got.CMRA != "Y" || got.RDI != "Commercial" {
		t.Fatalf("unexpected cmra/rdi: %s/%s", got.CMRA, got.RDI)
	}
	if got.StandardizedAddress.DeliveryLine1 == "" || got.StandardizedAddress.LastLine == "" {
		t.Fatalf("expected standardized address set")
	}
}

// ============================================================================
// Batch Validation Tests
// ============================================================================

func TestClientBatchMock(t *testing.T) {
	c := New(http.DefaultClient, Config{Mock: true})
	mailboxes := []model.Mailbox{
		{AddressRaw: model.AddressRaw{Street: "123 Main", City: "Dover", State: "DE", Zip: "19901"}},
		{AddressRaw: model.AddressRaw{Street: "456 Oak", City: "Newark", State: "DE", Zip: "19702"}},
	}

	results, err := c.ValidateMailboxBatch(context.Background(), mailboxes)
	if err != nil {
		t.Fatalf("ValidateMailboxBatch mock: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i, mb := range results {
		if mb.CMRA == "" || mb.RDI == "" {
			t.Errorf("mailbox %d: CMRA/RDI not set", i)
		}
		if mb.StandardizedAddress.DeliveryLine1 == "" {
			t.Errorf("mailbox %d: standardized address not set", i)
		}
	}
}

func TestClientBatchSuccess(t *testing.T) {
	// Mock batch response with input_index to map results back
	body := `[
		{"input_index": 0, "delivery_line_1": "123 Main St", "last_line": "Dover, DE 19901", "metadata": {"rdi": "Commercial"}, "analysis": {"dpv_cmra": "Y"}},
		{"input_index": 1, "delivery_line_1": "456 Oak Ave", "last_line": "Newark, DE 19702", "metadata": {"rdi": "Residential"}, "analysis": {"dpv_cmra": "N"}}
	]`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// Verify POST method and Content-Type
		if req.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", req.Method)
		}
		if ct := req.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	})

	c := New(rt, Config{Mock: false, AuthIDs: []string{"id"}, AuthTokens: []string{"token"}})

	mailboxes := []model.Mailbox{
		{AddressRaw: model.AddressRaw{Street: "123 Main", City: "Dover", State: "DE", Zip: "19901"}},
		{AddressRaw: model.AddressRaw{Street: "456 Oak", City: "Newark", State: "DE", Zip: "19702"}},
	}

	results, err := c.ValidateMailboxBatch(context.Background(), mailboxes)
	if err != nil {
		t.Fatalf("ValidateMailboxBatch: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify first mailbox
	if results[0].CMRA != "Y" || results[0].RDI != "Commercial" {
		t.Errorf("mailbox 0: unexpected CMRA/RDI: %s/%s", results[0].CMRA, results[0].RDI)
	}
	if results[0].StandardizedAddress.DeliveryLine1 != "123 Main St" {
		t.Errorf("mailbox 0: unexpected address: %s", results[0].StandardizedAddress.DeliveryLine1)
	}

	// Verify second mailbox
	if results[1].CMRA != "N" || results[1].RDI != "Residential" {
		t.Errorf("mailbox 1: unexpected CMRA/RDI: %s/%s", results[1].CMRA, results[1].RDI)
	}
	if results[1].StandardizedAddress.DeliveryLine1 != "456 Oak Ave" {
		t.Errorf("mailbox 1: unexpected address: %s", results[1].StandardizedAddress.DeliveryLine1)
	}
}

func TestClientBatchEmpty(t *testing.T) {
	c := New(http.DefaultClient, Config{Mock: false, AuthIDs: []string{"id"}, AuthTokens: []string{"token"}})

	results, err := c.ValidateMailboxBatch(context.Background(), []model.Mailbox{})
	if err != nil {
		t.Fatalf("ValidateMailboxBatch empty: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestClientBatchPartialResponse(t *testing.T) {
	// Test when Smarty returns fewer results than input (some addresses failed to validate)
	body := `[
		{"input_index": 0, "delivery_line_1": "123 Main St", "last_line": "Dover, DE 19901", "metadata": {"rdi": "Commercial"}, "analysis": {"dpv_cmra": "Y"}}
	]`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	})

	c := New(rt, Config{Mock: false, AuthIDs: []string{"id"}, AuthTokens: []string{"token"}})

	mailboxes := []model.Mailbox{
		{AddressRaw: model.AddressRaw{Street: "123 Main", City: "Dover", State: "DE", Zip: "19901"}},
		{AddressRaw: model.AddressRaw{Street: "Invalid Address", City: "Nowhere", State: "XX", Zip: "00000"}},
	}

	results, err := c.ValidateMailboxBatch(context.Background(), mailboxes)
	if err != nil {
		t.Fatalf("ValidateMailboxBatch partial: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First mailbox should be validated
	if results[0].CMRA != "Y" {
		t.Errorf("mailbox 0: expected CMRA=Y, got %s", results[0].CMRA)
	}

	// Second mailbox should retain original data (not validated)
	if results[1].CMRA != "" {
		t.Errorf("mailbox 1: expected empty CMRA (not validated), got %s", results[1].CMRA)
	}
}
