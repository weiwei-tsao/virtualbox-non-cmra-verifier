package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
	"google.golang.org/api/iterator"
)

func main() {
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		projectID = "virtualbox-non-cmra-verifier"
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer client.Close()

	// Query first 50 mailboxes to see what RDI values exist
	iter := client.Collection("mailboxes").Limit(50).Documents(ctx)

	rdiCounts := make(map[string]int)
	cmraCounts := make(map[string]int)
	total := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Error iterating: %v", err)
		}

		var m model.Mailbox
		if err := doc.DataTo(&m); err != nil {
			log.Printf("Error decoding %s: %v", doc.Ref.ID, err)
			continue
		}

		total++

		// Track RDI values
		if m.RDI == "" {
			rdiCounts["<empty>"]++
		} else {
			rdiCounts[m.RDI]++
		}

		// Track CMRA values
		if m.CMRA == "" {
			cmraCounts["<empty>"]++
		} else {
			cmraCounts[m.CMRA]++
		}

		// Print first few examples
		if total <= 5 {
			fmt.Printf("\nSample %d:\n", total)
			fmt.Printf("  Name: %s\n", m.Name)
			fmt.Printf("  RDI: '%s'\n", m.RDI)
			fmt.Printf("  CMRA: '%s'\n", m.CMRA)
			fmt.Printf("  LastValidated: %v\n", m.LastValidatedAt)
		}
	}

	fmt.Printf("\n=== Summary of %d mailboxes ===\n", total)
	fmt.Printf("\nRDI value distribution:\n")
	for val, count := range rdiCounts {
		fmt.Printf("  '%s': %d\n", val, count)
	}

	fmt.Printf("\nCMRA value distribution:\n")
	for val, count := range cmraCounts {
		fmt.Printf("  '%s': %d\n", val, count)
	}
}
