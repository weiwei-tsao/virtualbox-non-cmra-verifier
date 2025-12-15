package crawler

import (
	"context"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// ValidationClient abstracts Smarty validation for testability.
type ValidationClient interface {
	ValidateMailbox(ctx context.Context, mailbox model.Mailbox) (model.Mailbox, error)
	// ValidateMailboxBatch validates multiple mailboxes in a single batch request.
	// This is significantly more efficient than individual calls (up to 100 addresses per request).
	ValidateMailboxBatch(ctx context.Context, mailboxes []model.Mailbox) ([]model.Mailbox, error)
}
