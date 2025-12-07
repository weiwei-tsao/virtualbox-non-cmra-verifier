package repository

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// StatsRepository manages the system/stats singleton document.
type StatsRepository struct {
	client *firestore.Client
}

func NewStatsRepository(client *firestore.Client) *StatsRepository {
	return &StatsRepository{client: client}
}

func (r *StatsRepository) SaveSystemStats(ctx context.Context, stats model.SystemStats) error {
	stats.LastUpdated = time.Now().UTC()
	ref := r.client.Collection("system").Doc("stats")
	if _, err := ref.Set(ctx, stats); err != nil {
		return fmt.Errorf("save system stats: %w", err)
	}
	return nil
}

func (r *StatsRepository) GetSystemStats(ctx context.Context) (model.SystemStats, error) {
	ref := r.client.Collection("system").Doc("stats")
	snap, err := ref.Get(ctx)
	if err != nil {
		return model.SystemStats{}, fmt.Errorf("get system stats: %w", err)
	}
	var stats model.SystemStats
	if err := snap.DataTo(&stats); err != nil {
		return model.SystemStats{}, fmt.Errorf("decode system stats: %w", err)
	}
	return stats, nil
}
