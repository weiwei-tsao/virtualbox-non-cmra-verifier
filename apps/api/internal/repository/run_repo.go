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

// GetRun returns a crawl run by ID.
func (r *RunRepository) GetRun(ctx context.Context, runID string) (model.CrawlRun, error) {
	if runID == "" {
		return model.CrawlRun{}, fmt.Errorf("runId is required")
	}
	snap, err := r.client.Collection("crawl_runs").Doc(runID).Get(ctx)
	if err != nil {
		return model.CrawlRun{}, fmt.Errorf("get run %s: %w", runID, err)
	}
	var run model.CrawlRun
	if err := snap.DataTo(&run); err != nil {
		return model.CrawlRun{}, fmt.Errorf("decode run %s: %w", runID, err)
	}
	return run, nil
}
