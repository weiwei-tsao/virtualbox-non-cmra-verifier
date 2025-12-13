package main

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
	"github.com/joho/godotenv"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
)

func main() {
	ctx := context.Background()

	// Load environment variables
	_ = godotenv.Load(".env.local", ".env")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Firestore client with credentials
	client, credsSource, err := firestoreclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer client.Close()

	log.Printf("Connected to Firestore project %s using %s credentials", cfg.FirebaseProjectID, credsSource)

	fmt.Println("Starting migration: Adding Source field to existing data...")
	fmt.Println("========================================")

	// Migrate mailboxes collection
	if err := migrateMailboxes(ctx, client); err != nil {
		log.Fatalf("Failed to migrate mailboxes: %v", err)
	}

	// Migrate crawl_runs collection
	if err := migrateCrawlRuns(ctx, client); err != nil {
		log.Fatalf("Failed to migrate crawl_runs: %v", err)
	}

	fmt.Println("========================================")
	fmt.Println("Migration completed successfully!")
}

func migrateMailboxes(ctx context.Context, client *firestore.Client) error {
	fmt.Println("\n[1/2] Migrating mailboxes collection...")

	// Get all mailbox documents
	docs, err := client.Collection("mailboxes").Documents(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("failed to get mailboxes: %w", err)
	}

	total := len(docs)
	fmt.Printf("Found %d mailbox documents\n", total)

	if total == 0 {
		fmt.Println("No mailboxes to migrate")
		return nil
	}

	// Firestore batch write limit is 500 operations, but with updates we use smaller batches
	batchSize := 100
	updated := 0
	skipped := 0

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batch := client.Batch()
		batchCount := 0

		for j := i; j < end; j++ {
			doc := docs[j]
			data := doc.Data()

			// Skip if source field already exists
			if _, exists := data["source"]; exists {
				skipped++
				continue
			}

			// Add Source="ATMB" for all existing mailboxes
			batch.Update(doc.Ref, []firestore.Update{
				{Path: "source", Value: "ATMB"},
			})
			batchCount++
			updated++
		}

		// Commit batch if there are updates
		if batchCount > 0 {
			if _, err := batch.Commit(ctx); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			fmt.Printf("  Processed %d/%d documents...\n", end, total)
		}
	}

	fmt.Printf("✓ Mailboxes migration complete: %d updated, %d skipped\n", updated, skipped)
	return nil
}

func migrateCrawlRuns(ctx context.Context, client *firestore.Client) error {
	fmt.Println("\n[2/2] Migrating crawl_runs collection...")

	// Get all crawl_run documents
	docs, err := client.Collection("crawl_runs").Documents(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("failed to get crawl_runs: %w", err)
	}

	total := len(docs)
	fmt.Printf("Found %d crawl_run documents\n", total)

	if total == 0 {
		fmt.Println("No crawl_runs to migrate")
		return nil
	}

	// Firestore batch write limit is 500 operations, but with updates we use smaller batches
	batchSize := 100
	updated := 0
	skipped := 0

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batch := client.Batch()
		batchCount := 0

		for j := i; j < end; j++ {
			doc := docs[j]
			data := doc.Data()

			// Skip if source field already exists
			if _, exists := data["source"]; exists {
				skipped++
				continue
			}

			// Add Source="ATMB" for all existing crawl runs
			batch.Update(doc.Ref, []firestore.Update{
				{Path: "source", Value: "ATMB"},
			})
			batchCount++
			updated++
		}

		// Commit batch if there are updates
		if batchCount > 0 {
			if _, err := batch.Commit(ctx); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			fmt.Printf("  Processed %d/%d documents...\n", end, total)
		}
	}

	fmt.Printf("✓ Crawl_runs migration complete: %d updated, %d skipped\n", updated, skipped)
	return nil
}
