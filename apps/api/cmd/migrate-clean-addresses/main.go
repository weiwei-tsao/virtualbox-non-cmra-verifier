package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
	"github.com/joho/godotenv"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/util"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "Preview changes without writing to Firestore")
	sourceFilter := flag.String("source", "iPost1", "Filter by source (ATMB, iPost1, or empty for all)")
	flag.Parse()

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

	mode := "LIVE"
	if *dryRun {
		mode = "DRY-RUN"
	}

	fmt.Printf("\n=== Address Cleanup Migration [%s] ===\n", mode)
	fmt.Printf("Source filter: %s\n", *sourceFilter)
	fmt.Println("==========================================")

	if err := cleanAddresses(ctx, client, *dryRun, *sourceFilter); err != nil {
		log.Fatalf("Failed to clean addresses: %v", err)
	}

	fmt.Println("==========================================")
	fmt.Println("Migration completed!")
}

func cleanAddresses(ctx context.Context, client *firestore.Client, dryRun bool, sourceFilter string) error {
	fmt.Println("\nScanning mailboxes collection...")

	// Build query
	query := client.Collection("mailboxes").Query
	if sourceFilter != "" {
		query = query.Where("source", "==", sourceFilter)
	}

	docs, err := query.Documents(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("failed to get mailboxes: %w", err)
	}

	total := len(docs)
	fmt.Printf("Found %d mailbox documents\n", total)

	if total == 0 {
		fmt.Println("No mailboxes to process")
		return nil
	}

	// Analyze documents
	needsCleanup := 0
	cleanDocs := 0
	var toUpdate []updateItem

	for _, doc := range docs {
		var mb model.Mailbox
		if err := doc.DataTo(&mb); err != nil {
			log.Printf("Warning: failed to parse doc %s: %v", doc.Ref.ID, err)
			continue
		}

		if util.NeedsCleanup(mb.AddressRaw) {
			needsCleanup++
			toUpdate = append(toUpdate, updateItem{
				ref:      doc.Ref,
				original: mb,
			})

			// Show sample in dry-run mode
			if dryRun && needsCleanup <= 5 {
				fmt.Printf("\n--- Sample %d: %s ---\n", needsCleanup, mb.Name)
				fmt.Printf("BEFORE:\n")
				fmt.Printf("  Street: %q\n", mb.AddressRaw.Street)
				fmt.Printf("  City:   %q\n", mb.AddressRaw.City)
				fmt.Printf("  State:  %q\n", mb.AddressRaw.State)
				fmt.Printf("  Zip:    %q\n", mb.AddressRaw.Zip)
				fmt.Printf("  Link:   %q\n", mb.Link)

				cleaned := util.CleanAddress(mb.AddressRaw)
				cleanedLink := util.CleanLink(mb.Link)
				cleanedStdAddr := util.CleanStandardizedAddress(mb.StandardizedAddress)

				fmt.Printf("AFTER:\n")
				fmt.Printf("  Street: %q\n", cleaned.Street)
				fmt.Printf("  City:   %q\n", cleaned.City)
				fmt.Printf("  State:  %q\n", cleaned.State)
				fmt.Printf("  Zip:    %q\n", cleaned.Zip)
				fmt.Printf("  Link:   %q\n", cleanedLink)
				fmt.Printf("  StdAddr.DeliveryLine1: %q\n", cleanedStdAddr.DeliveryLine1)
				fmt.Printf("  StdAddr.LastLine: %q\n", cleanedStdAddr.LastLine)
			}
		} else {
			cleanDocs++
		}
	}

	fmt.Printf("\n=== Analysis Summary ===\n")
	fmt.Printf("Total documents:    %d\n", total)
	fmt.Printf("Need cleanup:       %d\n", needsCleanup)
	fmt.Printf("Already clean:      %d\n", cleanDocs)

	if needsCleanup == 0 {
		fmt.Println("\nNo documents need cleanup!")
		return nil
	}

	if dryRun {
		fmt.Printf("\n[DRY-RUN] Would update %d documents. Run without --dry-run to apply changes.\n", needsCleanup)
		return nil
	}

	// Apply updates in batches
	fmt.Printf("\nApplying cleanup to %d documents...\n", needsCleanup)

	batchSize := 100
	updated := 0

	for i := 0; i < len(toUpdate); i += batchSize {
		end := i + batchSize
		if end > len(toUpdate) {
			end = len(toUpdate)
		}

		batch := client.Batch()

		for j := i; j < end; j++ {
			item := toUpdate[j]
			cleaned := util.CleanAddress(item.original.AddressRaw)
			cleanedLink := util.CleanLink(item.original.Link)
			cleanedStdAddr := util.CleanStandardizedAddress(item.original.StandardizedAddress)

			batch.Update(item.ref, []firestore.Update{
				{Path: "addressRaw.street", Value: cleaned.Street},
				{Path: "addressRaw.city", Value: cleaned.City},
				{Path: "addressRaw.state", Value: cleaned.State},
				{Path: "addressRaw.zip", Value: cleaned.Zip},
				{Path: "link", Value: cleanedLink},
				{Path: "standardizedAddress.deliveryLine1", Value: cleanedStdAddr.DeliveryLine1},
				{Path: "standardizedAddress.lastLine", Value: cleanedStdAddr.LastLine},
			})
			updated++
		}

		if _, err := batch.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit batch: %w", err)
		}

		fmt.Printf("  Progress: %d/%d documents cleaned\n", updated, needsCleanup)
	}

	fmt.Printf("\n✓ Successfully cleaned %d documents\n", updated)
	return nil
}

type updateItem struct {
	ref      *firestore.DocumentRef
	original model.Mailbox
}
