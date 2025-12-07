package firestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// New creates a Firestore client using credentials provided via env (base64 or file).
// It returns the client and a description of which credential source was used.
func New(ctx context.Context, cfg config.Config) (*firestore.Client, string, error) {
	creds, source, err := cfg.FirebaseCredentialsJSON()
	if err != nil {
		return nil, "", err
	}

	client, err := firestore.NewClient(ctx, cfg.FirebaseProjectID, option.WithCredentialsJSON(creds))
	if err != nil {
		return nil, "", fmt.Errorf("init firestore client: %w", err)
	}
	return client, source, nil
}

// Ping performs a lightweight check by attempting to iterate collections.
func Ping(ctx context.Context, client *firestore.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	iter := client.Collections(ctx)
	_, err := iter.Next()
	if errors.Is(err, iterator.Done) {
		return nil
	}
	return err
}
