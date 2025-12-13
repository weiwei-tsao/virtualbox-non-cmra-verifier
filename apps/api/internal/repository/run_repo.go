package repository

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"google.golang.org/api/iterator"
)

// StaleRunTimeout is the duration after which a running job is considered stale.
const StaleRunTimeout = 45 * time.Minute

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

// ListRuns returns recent crawl runs ordered by startedAt descending.
// It also auto-marks stale running jobs as "timeout".
func (r *RunRepository) ListRuns(ctx context.Context, limit int) ([]model.CrawlRun, error) {
	if limit <= 0 {
		limit = 20
	}
	// Order by runId to include documents that might be missing startedAt.
	iter := r.client.Collection("crawl_runs").OrderBy("runId", firestore.Desc).Limit(limit).Documents(ctx)
	var runs []model.CrawlRun
	now := time.Now().UTC()

	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list runs: %w", err)
		}
		var run model.CrawlRun
		if err := snap.DataTo(&run); err != nil {
			return nil, fmt.Errorf("decode run %s: %w", snap.Ref.ID, err)
		}

		// Auto-mark stale running jobs as timeout
		if run.Status == "running" && !run.StartedAt.IsZero() && now.Sub(run.StartedAt) > StaleRunTimeout {
			run.Status = "timeout"
			run.FinishedAt = now
			// Update in background, don't block the list response
			go func(r *RunRepository, run model.CrawlRun) {
				_ = r.UpdateRun(context.Background(), run)
			}(r, run)
		}

		runs = append(runs, run)
	}
	return runs, nil
}

// CancelRun marks a running job as cancelled.
func (r *RunRepository) CancelRun(ctx context.Context, runID string) error {
	if runID == "" {
		return fmt.Errorf("runId is required")
	}

	// Get the current run
	run, err := r.GetRun(ctx, runID)
	if err != nil {
		return err
	}

	// Only allow cancelling running jobs
	if run.Status != "running" {
		return fmt.Errorf("run %s is not running (status: %s)", runID, run.Status)
	}

	// Update status to cancelled
	run.Status = "cancelled"
	run.FinishedAt = time.Now().UTC()

	return r.UpdateRun(ctx, run)
}
