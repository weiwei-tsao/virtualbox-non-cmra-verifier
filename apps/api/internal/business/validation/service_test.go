package validation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

type fakeMailboxRepository struct {
	byStatus map[string][]model.Mailbox
	updates  []validationStatusUpdate
	upserts  [][]model.Mailbox
}

type validationStatusUpdate struct {
	id          string
	status      string
	priority    string
	err         string
	nextRetryAt time.Time
	attempts    int
}

func (f *fakeMailboxRepository) FetchByValidationStatus(ctx context.Context, status, priority string, limit int) ([]model.Mailbox, error) {
	mailboxes := f.byStatus[status]
	result := []model.Mailbox{}

	for _, mb := range mailboxes {
		if priority != "" && mb.ValidationPriority != priority {
			continue
		}
		result = append(result, mb)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

func (f *fakeMailboxRepository) BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error {
	f.upserts = append(f.upserts, mailboxes)
	return nil
}

func (f *fakeMailboxRepository) UpdateValidationStatus(ctx context.Context, id string, status string, priority string, err string, nextRetryAt time.Time, attempts int) error {
	f.updates = append(f.updates, validationStatusUpdate{
		id:          id,
		status:      status,
		priority:    priority,
		err:         err,
		nextRetryAt: nextRetryAt,
		attempts:    attempts,
	})
	return nil
}

func TestProcessPrioritySchedulesQuotaExhaustionInsteadOfPending(t *testing.T) {
	repo := &fakeMailboxRepository{
		byStatus: map[string][]model.Mailbox{
			"pending": {
				{
					ID:                 "quota-hit",
					ValidationStatus:   "pending",
					ValidationPriority: "high",
				},
			},
		},
	}

	config := model.DefaultCrawlerConfig()
	config.ValidationBatchSize = 100
	config.MaxRetryAttempts = 5

	service := &ValidationService{
		repository:    repo,
		batchHandler:  NewBatchHandler(&fakeValidationClient{batchErr: errors.New("smarty status 402: payment required")}, DefaultBackoffConfig(), config.MaxRetryAttempts, nil, nil),
		config:        config,
		itemsSample:   []model.ValidationItem{},
		errorsSample:  []model.ErrorSample{},
		maxSampleSize: 50,
	}

	processed, succeeded, failed, quotaHit := service.processPriority(context.Background(), "high", 2)

	if processed != 1 || succeeded != 0 || failed != 1 || !quotaHit {
		t.Fatalf("processPriority = (%d, %d, %d, %v), want (1, 0, 1, true)",
			processed, succeeded, failed, quotaHit)
	}
	if len(repo.updates) != 1 {
		t.Fatalf("updates length = %d, want 1", len(repo.updates))
	}

	update := repo.updates[0]
	if update.status != "retry_scheduled" {
		t.Fatalf("quota exhausted status = %q, want retry_scheduled", update.status)
	}
	if !update.nextRetryAt.After(time.Now()) {
		t.Fatalf("nextRetryAt = %v, want a future retry time", update.nextRetryAt)
	}
	if update.attempts != 0 {
		t.Fatalf("attempts = %d, want unchanged attempts 0", update.attempts)
	}
}
