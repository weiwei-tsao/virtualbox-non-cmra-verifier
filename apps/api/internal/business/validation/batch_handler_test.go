package validation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

type fakeValidationClient struct {
	batchResults []model.Mailbox
	batchErr     error
	singleErrs   map[string]error
}

func (f *fakeValidationClient) ValidateMailbox(ctx context.Context, mb model.Mailbox) (model.Mailbox, error) {
	if err, ok := f.singleErrs[mb.ID]; ok {
		return mb, err
	}

	mb.CMRA = "Y"
	mb.RDI = "Commercial"
	mb.LastValidatedAt = time.Now().UTC()
	return mb, nil
}

func (f *fakeValidationClient) ValidateMailboxBatch(ctx context.Context, mailboxes []model.Mailbox) ([]model.Mailbox, error) {
	if f.batchResults != nil {
		return f.batchResults, f.batchErr
	}
	return mailboxes, f.batchErr
}

func TestValidateBatchRetriesUnchangedBatchRows(t *testing.T) {
	previousValidation := time.Now().Add(-30 * 24 * time.Hour).UTC()
	batchValidation := time.Now().UTC()

	mailboxes := []model.Mailbox{
		{
			ID: "matched",
			AddressRaw: model.AddressRaw{
				Street: "123 Main St",
				City:   "New York",
				State:  "NY",
				Zip:    "10001",
			},
			ValidationStatus:   "pending",
			ValidationPriority: "high",
		},
		{
			ID: "unmatched",
			AddressRaw: model.AddressRaw{
				Street: "No Candidate Ave",
				City:   "Nowhere",
				State:  "NY",
				Zip:    "10000",
			},
			CMRA:                "N",
			RDI:                 "Residential",
			LastValidatedAt:     previousValidation,
			ValidationStatus:    "needs_revalidation",
			ValidationPriority:  "high",
			ValidationAttempts:  1,
			LastValidationError: "old error",
		},
	}

	validatedMatched := mailboxes[0]
	validatedMatched.CMRA = "Y"
	validatedMatched.RDI = "Commercial"
	validatedMatched.StandardizedAddress = model.StandardizedAddress{
		DeliveryLine1: "123 Main St",
		LastLine:      "New York, NY 10001",
	}
	validatedMatched.LastValidatedAt = batchValidation

	client := &fakeValidationClient{
		batchResults: []model.Mailbox{
			validatedMatched,
			mailboxes[1],
		},
		singleErrs: map[string]error{
			"unmatched": errors.New("smarty: no candidates returned"),
		},
	}

	handler := NewBatchHandler(client, BackoffConfig{}, 1, nil, nil)
	result := handler.ValidateBatch(context.Background(), mailboxes)

	if got, want := len(result.Succeeded), 1; got != want {
		t.Fatalf("Succeeded length = %d, want %d", got, want)
	}
	if got, want := result.Succeeded[0].ID, "matched"; got != want {
		t.Fatalf("Succeeded[0].ID = %q, want %q", got, want)
	}
	if got := result.Succeeded[0].ValidationStatus; got != "validated" {
		t.Fatalf("Succeeded[0].ValidationStatus = %q, want validated", got)
	}

	if got, want := len(result.Failed), 1; got != want {
		t.Fatalf("Failed length = %d, want %d", got, want)
	}
	if got, want := result.Failed[0].Mailbox.ID, "unmatched"; got != want {
		t.Fatalf("Failed[0].Mailbox.ID = %q, want %q", got, want)
	}
	if result.Failed[0].Mailbox.LastValidatedAt != previousValidation {
		t.Fatalf("unmatched LastValidatedAt was changed to %v, want original %v",
			result.Failed[0].Mailbox.LastValidatedAt, previousValidation)
	}
	if result.Failed[0].Error == nil {
		t.Fatal("Failed[0].Error is nil, want no-candidates error")
	}
}
