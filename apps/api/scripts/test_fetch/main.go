package main

import (
	"context"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
)

func main() {
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

	mailboxes, err := mailboxRepo.FetchByValidationStatus(ctx, "needs_revalidation", "high", 5000)
	if err != nil {
		log.Fatalf("fetch mailboxes failed: %v", err)
	}

	fmt.Printf("TEST SCRIPT: FetchByValidationStatus returned %d mailboxes.\n", len(mailboxes))
}
