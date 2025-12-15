package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/crawler"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/smarty"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
)

func main() {
	// Parse flags
	dryRun := flag.Bool("dry-run", false, "Only show what would be done without making changes")
	force := flag.Bool("force", false, "Force re-validation even if data hash unchanged")
	flag.Parse()

	// Load environment variables
	_ = godotenv.Load(".env.local", ".env")

	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		log.Fatal("FIREBASE_PROJECT_ID not set")
	}

	authIDs := splitCSV(os.Getenv("SMARTY_AUTH_ID"))
	authTokens := splitCSV(os.Getenv("SMARTY_AUTH_TOKEN"))
	if len(authIDs) == 0 || len(authTokens) == 0 {
		log.Fatal("SMARTY_AUTH_ID and SMARTY_AUTH_TOKEN not set")
	}

	mockEnv := os.Getenv("SMARTY_MOCK")
	isMock := strings.ToLower(mockEnv) == "true"

	ctx := context.Background()

	// Get Firebase credentials
	credsJSON, credsSource, err := getFirebaseCredentials()
	if err != nil {
		log.Fatalf("Failed to get Firebase credentials: %v", err)
	}

	// Connect to Firestore
	fsClient, err := firestore.NewClient(ctx, projectID, option.WithCredentialsJSON(credsJSON))
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer fsClient.Close()

	fmt.Println("=== Batch Validation Script ===")
	fmt.Printf("Firebase: %s (credentials: %s)\n", projectID, credsSource)
	fmt.Printf("Smarty: %d credential(s), mock=%v\n", len(authIDs), isMock)
	fmt.Printf("Options: dry-run=%v, force=%v\n\n", *dryRun, *force)

	if *dryRun {
		fmt.Println("DRY RUN MODE - No changes will be made")
		fmt.Println()
	}

	// Create repository
	repo := repository.NewMailboxRepository(fsClient)

	// Create Smarty client
	validator := smarty.New(&http.Client{Timeout: 30 * time.Second}, smarty.Config{
		AuthIDs:    authIDs,
		AuthTokens: authTokens,
		Mock:       isMock,
	})

	// First, show current stats
	existing, err := repo.FetchAllMetadata(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch mailboxes: %v", err)
	}

	var needsValidation, alreadyValidated int
	for _, mb := range existing {
		if mb.CMRA == "" || mb.RDI == "" {
			needsValidation++
		} else {
			alreadyValidated++
		}
	}

	fmt.Printf("Current Database Status:\n")
	fmt.Printf("  Total records: %d\n", len(existing))
	fmt.Printf("  Already validated (CMRA/RDI set): %d\n", alreadyValidated)
	fmt.Printf("  Needs validation: %d\n", needsValidation)
	fmt.Println()

	if *dryRun {
		fmt.Println("Dry run complete. Run without -dry-run to apply changes.")
		return
	}

	if needsValidation == 0 && !*force {
		fmt.Println("All records already validated. Use -force to re-validate.")
		return
	}

	// Run reprocess with batch validation
	fmt.Println("Starting batch validation...")
	startTime := time.Now()

	opts := crawler.ReprocessOptions{
		ForceRevalidate: *force,
		BatchSize:       100,
	}

	stats, err := crawler.ReprocessFromDB(ctx, repo, validator, opts,
		func(msg string) {
			fmt.Printf("[%s] %s\n", time.Now().Format("15:04:05"), msg)
		},
		func(s crawler.ReprocessStats) {
			fmt.Printf("Progress: processed=%d, skipped=%d, failed=%d\n",
				s.Processed, s.Skipped, s.Failed)
		},
	)

	duration := time.Since(startTime)

	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Printf("Duration: %s\n", duration.Round(time.Second))
	fmt.Printf("Total: %d\n", stats.Total)
	fmt.Printf("Processed: %d\n", stats.Processed)
	fmt.Printf("Skipped: %d (NoHTML=%d, UpToDate=%d)\n", stats.Skipped, stats.NoHTML, stats.UpToDate)
	fmt.Printf("Failed: %d\n", stats.Failed)

	if err != nil {
		log.Fatalf("Reprocess failed: %v", err)
	}

	fmt.Println("\nBatch validation complete!")
}

func splitCSV(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func getFirebaseCredentials() ([]byte, string, error) {
	if b64 := strings.TrimSpace(os.Getenv("FIREBASE_CREDS_BASE64")); b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, "", fmt.Errorf("decode FIREBASE_CREDS_BASE64: %w", err)
		}
		return decoded, "base64", nil
	}

	if file := strings.TrimSpace(os.Getenv("FIREBASE_CREDS_FILE")); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, "", fmt.Errorf("read FIREBASE_CREDS_FILE: %w", err)
		}
		return data, "file", nil
	}

	return nil, "", fmt.Errorf("no Firebase credentials found")
}
