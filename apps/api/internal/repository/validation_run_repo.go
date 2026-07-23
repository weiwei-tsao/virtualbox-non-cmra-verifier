package repository

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"google.golang.org/api/iterator"
)

// ValidationRunRepository handles Firestore operations for validation runs.
type ValidationRunRepository struct {
	client *firestore.Client
}

func NewValidationRunRepository(client *firestore.Client) *ValidationRunRepository {
	return &ValidationRunRepository{client: client}
}

// CreateRun creates a new validation run record.
func (r *ValidationRunRepository) CreateRun(ctx context.Context, run *model.ValidationRun) error {
	if run.RunID == "" {
		return fmt.Errorf("runID is required")
	}
	_, err := r.client.Collection("validation_runs").Doc(run.RunID).Set(ctx, run)
	return err
}

// UpdateRun updates an existing validation run.
func (r *ValidationRunRepository) UpdateRun(ctx context.Context, run *model.ValidationRun) error {
	if run.RunID == "" {
		return fmt.Errorf("runID is required")
	}
	_, err := r.client.Collection("validation_runs").Doc(run.RunID).Set(ctx, run)
	return err
}

// GetRun retrieves a specific validation run by ID.
func (r *ValidationRunRepository) GetRun(ctx context.Context, runID string) (*model.ValidationRun, error) {
	doc, err := r.client.Collection("validation_runs").Doc(runID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get validation run: %w", err)
	}

	var run model.ValidationRun
	if err := doc.DataTo(&run); err != nil {
		return nil, fmt.Errorf("decode validation run: %w", err)
	}

	return &run, nil
}

// ListRuns retrieves the most recent validation runs.
func (r *ValidationRunRepository) ListRuns(ctx context.Context, limit int) ([]model.ValidationRun, error) {
	if limit <= 0 {
		limit = 20
	}

	iter := r.client.Collection("validation_runs").
		OrderBy("startedAt", firestore.Desc).
		Limit(limit).
		Documents(ctx)

	var runs []model.ValidationRun
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate validation runs: %w", err)
		}

		var run model.ValidationRun
		if err := doc.DataTo(&run); err != nil {
			return nil, fmt.Errorf("decode validation run %s: %w", doc.Ref.ID, err)
		}

		runs = append(runs, run)
	}

	return runs, nil
}

// UpdateStatus updates the status and finished time of a validation run.
func (r *ValidationRunRepository) UpdateStatus(ctx context.Context, runID string, status string) error {
	updates := []firestore.Update{
		{Path: "status", Value: status},
	}

	if status != "running" {
		updates = append(updates, firestore.Update{
			Path:  "finishedAt",
			Value: time.Now(),
		})
	}

	_, err := r.client.Collection("validation_runs").Doc(runID).Update(ctx, updates)
	return err
}

// UpdateStats updates the stats of a validation run.
func (r *ValidationRunRepository) UpdateStats(ctx context.Context, runID string, stats model.ValidationRunStats) error {
	_, err := r.client.Collection("validation_runs").Doc(runID).Update(ctx, []firestore.Update{
		{Path: "stats", Value: stats},
	})
	return err
}
