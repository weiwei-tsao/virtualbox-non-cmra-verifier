package crawler

import (
	"context"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// ValidationClient abstracts Smarty validation for testability.
type ValidationClient interface {
	ValidateMailbox(ctx context.Context, mailbox model.Mailbox) (model.Mailbox, error)
}
