package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
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

	// Query all mailboxes
	iter := client.Collection("mailboxes").Documents(ctx)

	stats := struct {
		Total           int
		CMRAYes         int
		CMRANo          int
		CMRAEmpty       int
		RDICommercial   int
		RDIResidential  int
		RDIEmpty        int
		SampleRecords   []model.Mailbox
	}{
		SampleRecords: make([]model.Mailbox, 0, 5),
	}

	fmt.Println("Analyzing CMRA and RDI distribution...")

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Iterator error: %v", err)
		}

		var mb model.Mailbox
		if err := doc.DataTo(&mb); err != nil {
			log.Printf("Error decoding mailbox %s: %v", doc.Ref.ID, err)
			continue
		}

		stats.Total++

		// Count CMRA values
		switch mb.CMRA {
		case "Y":
			stats.CMRAYes++
		case "N":
			stats.CMRANo++
		case "":
			stats.CMRAEmpty++
		}

		// Count RDI values
		switch mb.RDI {
		case "Commercial":
			stats.RDICommercial++
		case "Residential":
			stats.RDIResidential++
		case "":
			stats.RDIEmpty++
		}

		// Collect sample records (first 5)
		if len(stats.SampleRecords) < 5 {
			stats.SampleRecords = append(stats.SampleRecords, mb)
		}
	}

	// Print summary
	fmt.Printf("=== CMRA/RDI Distribution Summary ===\n\n")
	fmt.Printf("Total Mailboxes: %d\n\n", stats.Total)

	fmt.Printf("CMRA Field:\n")
	fmt.Printf("  Y (Commercial Mail Receiving Agency): %d (%.1f%%)\n",
		stats.CMRAYes, float64(stats.CMRAYes)/float64(stats.Total)*100)
	fmt.Printf("  N (Not CMRA):                         %d (%.1f%%)\n",
		stats.CMRANo, float64(stats.CMRANo)/float64(stats.Total)*100)
	fmt.Printf("  Empty/Missing:                        %d (%.1f%%)\n\n",
		stats.CMRAEmpty, float64(stats.CMRAEmpty)/float64(stats.Total)*100)

	fmt.Printf("RDI Field:\n")
	fmt.Printf("  Commercial:    %d (%.1f%%)\n",
		stats.RDICommercial, float64(stats.RDICommercial)/float64(stats.Total)*100)
	fmt.Printf("  Residential:   %d (%.1f%%)\n",
		stats.RDIResidential, float64(stats.RDIResidential)/float64(stats.Total)*100)
	fmt.Printf("  Empty/Missing: %d (%.1f%%)\n\n",
		stats.RDIEmpty, float64(stats.RDIEmpty)/float64(stats.Total)*100)

	// Print sample records
	fmt.Printf("=== Sample Records (First 5) ===\n\n")
	output, _ := json.MarshalIndent(stats.SampleRecords, "", "  ")
	fmt.Println(string(output))

	// Print analysis
	fmt.Printf("\n=== Analysis ===\n\n")
	if stats.RDIResidential == 0 && stats.CMRAYes > 0 {
		fmt.Println("Finding: All addresses are Commercial")
		fmt.Println("   Reason: AnytimeMailbox addresses are CMRA services")
		fmt.Println("   CMRA (Commercial Mail Receiving Agency) = always Commercial RDI")
		fmt.Println("\n   This is expected behavior - virtual mailbox services are")
		fmt.Println("   inherently commercial operations, not residential addresses.")
	}

	if stats.CMRAEmpty > 0 || stats.RDIEmpty > 0 {
		fmt.Printf("\nWarning: Found %d records with missing CMRA and/or %d with missing RDI\n",
			stats.CMRAEmpty, stats.RDIEmpty)
		fmt.Println("   These records may not have been validated by Smarty API yet.")
		fmt.Println("   Run a reprocess to update them.")
	}
}
