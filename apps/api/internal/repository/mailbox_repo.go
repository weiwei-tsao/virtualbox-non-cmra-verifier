package repository

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
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

func documentID(m model.Mailbox) string {
	if m.ID != "" {
		return m.ID
	}
	if m.Link != "" {
		return util.HashString(m.Link)
	}
	return util.HashMailboxKey(m.Name, m.AddressRaw)
}
