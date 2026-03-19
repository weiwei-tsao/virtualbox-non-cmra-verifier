package repository

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	firestorepb "cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/util"
	"google.golang.org/api/iterator"
)

// MailboxRepository handles Firestore read/write for mailboxes.
type MailboxRepository struct {
	client *firestore.Client
}

func NewMailboxRepository(client *firestore.Client) *MailboxRepository {
	return &MailboxRepository{client: client}
}

// FetchAllMap loads all mailboxes into a memory map keyed by link.
func (r *MailboxRepository) FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error) {
	iter := r.client.Collection("mailboxes").Documents(ctx)
	result := make(map[string]model.Mailbox)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate mailboxes: %w", err)
		}
		var m model.Mailbox
		if err := doc.DataTo(&m); err != nil {
			return nil, fmt.Errorf("decode mailbox %s: %w", doc.Ref.ID, err)
		}
		if m.ID == "" {
			m.ID = doc.Ref.ID
		}
		key := m.Link
		if key == "" {
			key = doc.Ref.ID
		}
		result[key] = m
	}
	return result, nil
}

// FetchAllMetadata loads only essential fields for deduplication (excludes RawHTML).
// This is ~90% faster than FetchAllMap as it doesn't load the large RawHTML field.
func (r *MailboxRepository) FetchAllMetadata(ctx context.Context) (map[string]model.Mailbox, error) {
	// Select only the fields needed for scraper deduplication
	iter := r.client.Collection("mailboxes").
		Select("link", "dataHash", "cmra", "rdi", "id", "source").
		Documents(ctx)

	result := make(map[string]model.Mailbox)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate mailboxes metadata: %w", err)
		}
		var m model.Mailbox
		if err := doc.DataTo(&m); err != nil {
			return nil, fmt.Errorf("decode mailbox metadata %s: %w", doc.Ref.ID, err)
		}
		if m.ID == "" {
			m.ID = doc.Ref.ID
		}
		key := m.Link
		if key == "" {
			key = doc.Ref.ID
		}
		result[key] = m
	}
	return result, nil
}

// BatchUpsert writes mailboxes in batches to reduce round trips.
func (r *MailboxRepository) BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error {
	if len(mailboxes) == 0 {
		return nil
	}
	const batchSize = 400

	for start := 0; start < len(mailboxes); start += batchSize {
		end := start + batchSize
		if end > len(mailboxes) {
			end = len(mailboxes)
		}
		batch := r.client.Batch()
		for _, m := range mailboxes[start:end] {
			docID := documentID(m)
			ref := r.client.Collection("mailboxes").Doc(docID)
			if m.ID == "" {
				m.ID = docID
			}
			batch.Set(ref, m)
		}
		if _, err := batch.Commit(ctx); err != nil {
			return fmt.Errorf("commit batch [%d:%d]: %w", start, end, err)
		}
	}
	return nil
}

// MailboxQuery represents filters and pagination options.
type MailboxQuery struct {
	State    string
	CMRA     string
	RDI      string
	Source   string
	Active   *bool
	Page     int
	PageSize int
}

// List returns filtered mailboxes with pagination and total count.
func (r *MailboxRepository) List(ctx context.Context, q MailboxQuery) ([]model.Mailbox, int, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}

	query := r.client.Collection("mailboxes").Query
	if q.State != "" {
		query = query.Where("addressRaw.state", "==", q.State)
	}
	if q.CMRA != "" {
		query = query.Where("cmra", "==", q.CMRA)
	}
	if q.RDI != "" {
		query = query.Where("rdi", "==", q.RDI)
	}
	if q.Source != "" {
		query = query.Where("source", "==", q.Source)
	}
	if q.Active != nil {
		query = query.Where("active", "==", *q.Active)
	}

	// Use Firestore Aggregation Count API for efficient counting (SDK v1.11+)
	countQuery := query.NewAggregationQuery().WithCount("total")
	countResult, err := countQuery.Get(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count mailboxes: %w", err)
	}
	countValue := countResult["total"].(*firestorepb.Value)
	total := int(countValue.GetIntegerValue())

	offset := (q.Page - 1) * q.PageSize
	iter := query.Offset(offset).Limit(q.PageSize).Documents(ctx)

	var items []model.Mailbox
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("list mailboxes: %w", err)
		}
		var m model.Mailbox
		if err := doc.DataTo(&m); err != nil {
			return nil, 0, fmt.Errorf("decode mailbox %s: %w", doc.Ref.ID, err)
		}
		if m.ID == "" {
			m.ID = doc.Ref.ID
		}
		items = append(items, m)
	}
	return items, total, nil
}

// StreamAll streams mailboxes (optionally filtered by active) to a callback without loading all into memory.
func (r *MailboxRepository) StreamAll(ctx context.Context, activeOnly bool, fn func(model.Mailbox) error) error {
	query := r.client.Collection("mailboxes").Query
	if activeOnly {
		query = query.Where("active", "==", true)
	}
	iter := query.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return fmt.Errorf("iterate mailboxes: %w", err)
		}
		var m model.Mailbox
		if err := doc.DataTo(&m); err != nil {
			return fmt.Errorf("decode mailbox %s: %w", doc.Ref.ID, err)
		}
		if m.ID == "" {
			m.ID = doc.Ref.ID
		}
		if err := fn(m); err != nil {
			return err
		}
	}
}

// StreamWithQuery streams mailboxes with filters to a callback without loading all into memory.
func (r *MailboxRepository) StreamWithQuery(ctx context.Context, q MailboxQuery, fn func(model.Mailbox) error) error {
	query := r.client.Collection("mailboxes").Query
	if q.State != "" {
		query = query.Where("addressRaw.state", "==", q.State)
	}
	if q.CMRA != "" {
		query = query.Where("cmra", "==", q.CMRA)
	}
	if q.RDI != "" {
		query = query.Where("rdi", "==", q.RDI)
	}
	if q.Source != "" {
		query = query.Where("source", "==", q.Source)
	}
	if q.Active != nil {
		query = query.Where("active", "==", *q.Active)
	}

	iter := query.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return fmt.Errorf("iterate mailboxes: %w", err)
		}
		var m model.Mailbox
		if err := doc.DataTo(&m); err != nil {
			return fmt.Errorf("decode mailbox %s: %w", doc.Ref.ID, err)
		}
		if m.ID == "" {
			m.ID = doc.Ref.ID
		}
		if err := fn(m); err != nil {
			return err
		}
	}
}

// BulkSetActive marks multiple mailboxes as active or inactive (soft delete).
// Used for mark-and-sweep deletion when records are no longer found at source.
func (r *MailboxRepository) BulkSetActive(ctx context.Context, ids []string, active bool) error {
	if len(ids) == 0 {
		return nil
	}

	const batchSize = 500
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		batch := r.client.Batch()
		for _, id := range ids[start:end] {
			ref := r.client.Collection("mailboxes").Doc(id)
			batch.Update(ref, []firestore.Update{
				{Path: "active", Value: active},
			})
		}

		if _, err := batch.Commit(ctx); err != nil {
			return fmt.Errorf("bulk set active [%d:%d]: %w", start, end, err)
		}
	}

	return nil
}

// FetchByValidationStatus fetches mailboxes by validation status and optionally by priority.
// Used by validation service to process pending queue.
func (r *MailboxRepository) FetchByValidationStatus(ctx context.Context, status, priority string, limit int) ([]model.Mailbox, error) {
	query := r.client.Collection("mailboxes").
		Where("validationStatus", "==", status)

	// Add priority filter if specified
	if priority != "" {
		query = query.Where("validationPriority", "==", priority)
	}

	// Note: OrderBy requires composite index. For now, fetch without ordering
	// and let the service handle prioritization
	// TODO: Enable once composite index (validationStatus, validationPriority, nextRetryAt) is built
	// query = query.OrderBy("validationPriority", firestore.Asc).OrderBy("nextRetryAt", firestore.Asc)

	// Apply limit
	if limit > 0 {
		query = query.Limit(limit)
	}

	iter := query.Documents(ctx)
	var results []model.Mailbox

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate mailboxes: %w", err)
		}

		var mb model.Mailbox
		if err := doc.DataTo(&mb); err != nil {
			return nil, fmt.Errorf("decode mailbox %s: %w", doc.Ref.ID, err)
		}

		if mb.ID == "" {
			mb.ID = doc.Ref.ID
		}

		results = append(results, mb)
	}

	return results, nil
}

// UpdateValidationStatus updates validation-related fields for a single mailbox.
// Used by validation service to update status after validation attempts.
func (r *MailboxRepository) UpdateValidationStatus(
	ctx context.Context,
	id string,
	status string,
	priority string,
	errorMsg string,
	nextRetryAt time.Time,
	attempts int,
) error {
	ref := r.client.Collection("mailboxes").Doc(id)

	updates := []firestore.Update{
		{Path: "validationStatus", Value: status},
		{Path: "validationPriority", Value: priority},
		{Path: "validationAttempts", Value: attempts},
		{Path: "lastValidationAttempt", Value: time.Now()},
	}

	if errorMsg != "" {
		updates = append(updates, firestore.Update{
			Path: "lastValidationError", Value: errorMsg,
		})
	}

	if !nextRetryAt.IsZero() {
		updates = append(updates, firestore.Update{
			Path: "nextRetryAt", Value: nextRetryAt,
		})
	}

	_, err := ref.Update(ctx, updates)
	if err != nil {
		return fmt.Errorf("update validation status: %w", err)
	}

	return nil
}

// ValidationStats represents counts by validation status.
type ValidationStats struct {
	Pending         int `json:"pending"`
	Validated       int `json:"validated"`
	Failed          int `json:"failed"`
	NeedsRevalidation int `json:"needsRevalidation"`
	RetryScheduled  int `json:"retryScheduled"`
	ManualReview    int `json:"manualReview"`
	Total           int `json:"total"`
}

// GetValidationStats returns counts of mailboxes by validation status.
// Used for dashboard metrics and monitoring.
func (r *MailboxRepository) GetValidationStats(ctx context.Context) (ValidationStats, error) {
	stats := ValidationStats{}

	// Count by each status
	statuses := []string{"pending", "validated", "failed", "needs_revalidation", "retry_scheduled", "manual_review"}

	for _, status := range statuses {
		query := r.client.Collection("mailboxes").Where("validationStatus", "==", status)
		countQuery := query.NewAggregationQuery().WithCount("total")

		result, err := countQuery.Get(ctx)
		if err != nil {
			return stats, fmt.Errorf("count %s: %w", status, err)
		}

		countValue := result["total"].(*firestorepb.Value)
		count := int(countValue.GetIntegerValue())

		switch status {
		case "pending":
			stats.Pending = count
		case "validated":
			stats.Validated = count
		case "failed":
			stats.Failed = count
		case "needs_revalidation":
			stats.NeedsRevalidation = count
		case "retry_scheduled":
			stats.RetryScheduled = count
		case "manual_review":
			stats.ManualReview = count
		}

		stats.Total += count
	}

	return stats, nil
}

func documentID(m model.Mailbox) string {
	if m.ID != "" {
		return m.ID
	}
	if m.Link != "" {
		return util.HashString(m.Link)
	}
	return util.HashMailboxKey(m.Name, m.AddressRaw)
}
