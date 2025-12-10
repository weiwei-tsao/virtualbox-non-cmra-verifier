package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
)

func main() {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "virtualbox-non-cmra-verifier")
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer client.Close()

	// Query the specific record the user mentioned
	docID := "134d6b225ebc0a947a572bb33cb78125"
	doc, err := client.Collection("mailboxes").Doc(docID).Get(ctx)
	if err != nil {
		log.Fatalf("Failed to get document: %v", err)
	}

	fmt.Printf("Document ID: %s\n", docID)
	fmt.Printf("Document exists: %v\n\n", doc.Exists())

	// Get raw data
	data := doc.Data()

	// Pretty print the entire document
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	fmt.Println("Full document data:")
	fmt.Println(string(jsonData))

	// Specifically check RDI and CMRA fields
	fmt.Printf("\n=== Specific field checks ===\n")
	if rdi, ok := data["rdi"]; ok {
		fmt.Printf("RDI field exists: '%v' (type: %T)\n", rdi, rdi)
	} else {
		fmt.Printf("RDI field: DOES NOT EXIST in Firestore\n")
	}

	if cmra, ok := data["cmra"]; ok {
		fmt.Printf("CMRA field exists: '%v' (type: %T)\n", cmra, cmra)
	} else {
		fmt.Printf("CMRA field: DOES NOT EXIST in Firestore\n")
	}
}
