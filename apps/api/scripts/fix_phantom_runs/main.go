package main

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/firestore"
)

func main() {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "virtualbox-non-cmra-verifier")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Find the stuck run
	docRef := client.Collection("validation_runs").Doc("validation_1775698001")
	_, err = docRef.Update(ctx, []firestore.Update{
		{Path: "status", Value: "cancelled"},
		{Path: "finishedAt", Value: time.Now()},
	})
	if err != nil {
		log.Fatalf("Failed to update: %v", err)
	}
	log.Println("Successfully fixed phantom run validation_1775698001!")
}
