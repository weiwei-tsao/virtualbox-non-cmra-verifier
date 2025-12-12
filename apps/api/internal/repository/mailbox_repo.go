package repository

import (
	"context"
	"fmt"

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
		Select("link", "dataHash", "cmra", "rdi", "id").
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

func documentID(m model.Mailbox) string {
	if m.ID != "" {
		return m.ID
	}
	if m.Link != "" {
		return util.HashString(m.Link)
	}
	return util.HashMailboxKey(m.Name, m.AddressRaw)
}
