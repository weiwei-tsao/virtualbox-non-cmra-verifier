package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

func main() {
	// Get Firebase project ID from environment
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		log.Fatal("FIREBASE_PROJECT_ID environment variable not set")
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer client.Close()

	// Query mailboxes without RawHTML
	iter := client.Collection("mailboxes").Documents(ctx)

	var missingHTML []map[string]interface{}
	count := 0

	fmt.Println("Searching for mailboxes without RawHTML...")

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Iterator error: %v", err)
		}

		data := doc.Data()
		rawHTML, ok := data["rawHTML"]

		// Check if rawHTML is missing or empty
		if !ok || rawHTML == nil || rawHTML == "" {
			count++
			missingHTML = append(missingHTML, map[string]interface{}{
				"id":   doc.Ref.ID,
				"name": data["name"],
				"link": data["link"],
			})
		}
	}

	fmt.Printf("\nFound %d mailboxes without RawHTML:\n\n", count)

	if count > 0 {
		// Print as JSON
		output, _ := json.MarshalIndent(missingHTML, "", "  ")
		fmt.Println(string(output))

		// Extract just the links
		fmt.Println("\n\nLinks only (for re-crawling):")
		fmt.Println("[")
		for i, m := range missingHTML {
			link := m["link"]
			if i < len(missingHTML)-1 {
				fmt.Printf("  \"%s\",\n", link)
			} else {
				fmt.Printf("  \"%s\"\n", link)
			}
		}
		fmt.Println("]")
	}
}
