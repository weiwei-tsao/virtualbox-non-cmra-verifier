package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"google.golang.org/api/iterator"
)

func main() {
	limitPtr := flag.Int("limit", 1000, "Number of mailboxes to force revalidation on")
	flag.Parse()

	ctx := context.Background()

	// Try to load env variables depending on where the user runs the script
	if err := godotenv.Load(".env.local", ".env"); err != nil {
		_ = godotenv.Load("../../.env.local", "../../.env") // Fallback
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	firestoreClient, credsSource, err := firestoreclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("init firestore: %v", err)
	}
	defer firestoreClient.Close()

	log.Printf("Connected to Firestore project %s using %s credentials", cfg.FirebaseProjectID, credsSource)

	mailboxRepo := repository.NewMailboxRepository(firestoreClient)

	log.Printf("Starting to force %d items to need revalidation...", *limitPtr)

	iter := firestoreClient.Collection("mailboxes").
		Where("validationStatus", "==", "validated").
		Limit(*limitPtr).
		Documents(ctx)

	var mailboxes []model.Mailbox
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("iterate mailboxes: %v", err)
		}
		var mb model.Mailbox
		if err := doc.DataTo(&mb); err != nil {
			log.Fatalf("decode mailbox %s: %v", doc.Ref.ID, err)
		}
		if mb.ID == "" {
			mb.ID = doc.Ref.ID
		}
		mailboxes = append(mailboxes, mb)
	}

	log.Printf("Found %d validated mailboxes to update", len(mailboxes))
	if len(mailboxes) == 0 {
		fmt.Println("No validated mailboxes found!")
		return
	}

	var toUpdate []model.Mailbox
	updatedCount := 0

	for _, mb := range mailboxes {
		mb.ValidationStatus = "needs_revalidation"
		mb.ValidationPriority = "high"
		toUpdate = append(toUpdate, mb)

		// Batch update every 50 items to optimize size
		if len(toUpdate) >= 50 {
			if err := mailboxRepo.BatchUpsert(ctx, toUpdate); err != nil {
				log.Fatalf("batch upsert failed: %v", err)
			}
			updatedCount += len(toUpdate)
			log.Printf("Updated %d mailboxes...", updatedCount)
			toUpdate = toUpdate[:0]
		}
	}

	// Final batch update
	if len(toUpdate) > 0 {
		if err := mailboxRepo.BatchUpsert(ctx, toUpdate); err != nil {
			log.Fatalf("final batch upsert failed: %v", err)
		}
		updatedCount += len(toUpdate)
	}

	fmt.Printf("\n✅ Successfully marked %d mailboxes as needs_revalidation.\n", updatedCount)
	fmt.Println("You can now go to the web UI and click 'Start Validation'.")
}
