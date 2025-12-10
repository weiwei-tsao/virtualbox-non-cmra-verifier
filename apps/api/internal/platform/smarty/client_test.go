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
	c := New(rt, Config{Mock: false, AuthID: "id", AuthToken: "token", BreakerMax: 2, MaxRetries: 1})

	_, err := c.ValidateMailbox(context.Background(), model.Mailbox{
		AddressRaw: model.AddressRaw{Street: "1", City: "2", State: "3", Zip: "4"},
	})
	if err == nil {
		t.Fatalf("expected error on 429")
	}
	if err != ErrCircuitOpen {
		// First 429 increments counter; second call should hit breaker.
		_, err = c.ValidateMailbox(context.Background(), model.Mailbox{
			AddressRaw: model.AddressRaw{Street: "1", City: "2", State: "3", Zip: "4"},
		})
		if err != ErrCircuitOpen {
			t.Fatalf("expected circuit open, got %v", err)
		}
	}
}

func TestClientSuccess(t *testing.T) {
	body := `[{"delivery_line_1":"123 Main","last_line":"Dover, DE 19901","analysis":{"cmra":"Y","rdi":"Commercial"}}]`
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	})
	c := New(rt, Config{Mock: false, AuthID: "id", AuthToken: "token"})
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
