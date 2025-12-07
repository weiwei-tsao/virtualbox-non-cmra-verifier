package repository

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// RunRepository manages crawl run lifecycle records.
type RunRepository struct {
	client *firestore.Client
}

func NewRunRepository(client *firestore.Client) *RunRepository {
	return &RunRepository{client: client}
}

func (r *RunRepository) CreateRun(ctx context.Context, run model.CrawlRun) error {
	if run.RunID == "" {
		return fmt.Errorf("runId is required")
	}
	ref := r.client.Collection("crawl_runs").Doc(run.RunID)
	if _, err := ref.Set(ctx, run); err != nil {
		return fmt.Errorf("create run %s: %w", run.RunID, err)
	}
	return nil
}

func (r *RunRepository) UpdateRun(ctx context.Context, run model.CrawlRun) error {
	if run.RunID == "" {
		return fmt.Errorf("runId is required")
	}
	ref := r.client.Collection("crawl_runs").Doc(run.RunID)
	if _, err := ref.Set(ctx, run); err != nil {
		return fmt.Errorf("update run %s: %w", run.RunID, err)
	}
	return nil
}
