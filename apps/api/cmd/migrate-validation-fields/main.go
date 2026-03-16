package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Initialize Firestore client
	firestoreClient, credsSource, err := firestoreclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("init firestore: %v", err)
	}
	defer firestoreClient.Close()

	log.Printf("Connected to Firestore project %s using %s credentials", cfg.FirebaseProjectID, credsSource)

	// Initialize repository
	mailboxRepo := repository.NewMailboxRepository(firestoreClient)

	log.Println("Starting validation fields migration...")
	log.Println("Fetching all mailboxes...")

	// Fetch all mailboxes
	allMailboxes, err := mailboxRepo.FetchAllMap(ctx)
	if err != nil {
		log.Fatalf("fetch mailboxes: %v", err)
	}

	log.Printf("Found %d mailboxes to migrate", len(allMailboxes))

	stats := struct {
		Total             int
		AlreadyValidated  int
		Pending           int
		NeedsRevalidation int
		Updated           int
	}{Total: len(allMailboxes)}

	var toUpdate []model.Mailbox
	now := time.Now()
	revalidationThreshold := now.Add(-90 * 24 * time.Hour)      // 90 days ago
	mediumPriorityThreshold := now.Add(-70 * 24 * time.Hour)    // 70 days ago

	for _, mb := range allMailboxes {
		needsUpdate := false

		// Set validation status based on existing CMRA/RDI values
		if mb.ValidationStatus == "" {
			if mb.CMRA != "" && mb.RDI != "" {
				// Already validated
				mb.ValidationStatus = "validated"
				stats.AlreadyValidated++
				needsUpdate = true
			} else {
				// Never validated
				mb.ValidationStatus = "pending"
				stats.Pending++
				needsUpdate = true
			}
		}

		// Set validation priority based on last validated time
		if mb.ValidationPriority == "" {
			if mb.LastValidatedAt.IsZero() {
				// Never validated - high priority
				mb.ValidationPriority = "high"
				needsUpdate = true
			} else if mb.LastValidatedAt.Before(revalidationThreshold) {
				// Older than 90 days - needs revalidation
				mb.ValidationStatus = "needs_revalidation"
				mb.ValidationPriority = "high"
				stats.NeedsRevalidation++
				needsUpdate = true
			} else if mb.LastValidatedAt.Before(mediumPriorityThreshold) {
				// 70-89 days old - medium priority
				mb.ValidationPriority = "medium"
				needsUpdate = true
			} else {
				// Less than 70 days old - low priority
				mb.ValidationPriority = "low"
				needsUpdate = true
			}
		}

		// Initialize validation attempts to 0 if not set
		if mb.ValidationAttempts == 0 && needsUpdate {
			mb.ValidationAttempts = 0
		}

		if needsUpdate {
			toUpdate = append(toUpdate, mb)
			stats.Updated++
		}

		// Batch update every 400 items to avoid memory issues
		if len(toUpdate) >= 400 {
			if err := mailboxRepo.BatchUpsert(ctx, toUpdate); err != nil {
				log.Fatalf("batch upsert failed: %v", err)
			}
			log.Printf("Updated %d mailboxes...", stats.Updated)
			toUpdate = toUpdate[:0] // Clear slice
		}
	}

	// Final batch update
	if len(toUpdate) > 0 {
		if err := mailboxRepo.BatchUpsert(ctx, toUpdate); err != nil {
			log.Fatalf("final batch upsert failed: %v", err)
		}
	}

	log.Println("\n=== Migration Complete ===")
	log.Printf("Total mailboxes: %d", stats.Total)
	log.Printf("Already validated: %d", stats.AlreadyValidated)
	log.Printf("Pending (never validated): %d", stats.Pending)
	log.Printf("Needs revalidation (>90 days): %d", stats.NeedsRevalidation)
	log.Printf("Total updated: %d", stats.Updated)

	fmt.Println("\n✅ Migration successful!")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Create Firestore indexes in Firebase Console:")
	fmt.Println("   - (validationStatus ASC, validationPriority ASC, nextRetryAt ASC)")
	fmt.Println("   - (validationStatus ASC, lastValidatedAt ASC)")
	fmt.Println("   - (active ASC, validationStatus ASC)")
	fmt.Println("2. Allow 1-2 hours for indexes to build")
	fmt.Println("3. Proceed with Phase 2: Configuration System")
}
