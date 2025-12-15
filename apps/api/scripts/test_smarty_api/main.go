package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/joho/godotenv"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/smarty"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

const sampleSize = 10

func main() {
	// Load environment variables from .env files
	_ = godotenv.Load(".env.local", ".env")

	// Get environment variables
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		log.Fatal("FIREBASE_PROJECT_ID environment variable not set")
	}

	authIDs := splitCSV(os.Getenv("SMARTY_AUTH_ID"))
	authTokens := splitCSV(os.Getenv("SMARTY_AUTH_TOKEN"))

	if len(authIDs) == 0 || len(authTokens) == 0 {
		log.Fatal("SMARTY_AUTH_ID and SMARTY_AUTH_TOKEN environment variables not set")
	}

	mockEnv := os.Getenv("SMARTY_MOCK")
	if strings.ToLower(mockEnv) == "true" {
		log.Println("WARNING: SMARTY_MOCK is set to true. This test will use mock data.")
		log.Println("Set SMARTY_MOCK=false to test the real Smarty API.")
	}

	ctx := context.Background()

	// Get Firebase credentials
	credsJSON, credsSource, err := getFirebaseCredentials()
	if err != nil {
		log.Fatalf("Failed to get Firebase credentials: %v", err)
	}

	// Connect to Firestore with credentials
	client, err := firestore.NewClient(ctx, projectID, option.WithCredentialsJSON(credsJSON))
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer client.Close()
	fmt.Printf("Connected to Firestore using %s credentials\n", credsSource)

	fmt.Println("=== Smarty API Test Script ===")
	fmt.Printf("Testing with %d credential(s)\n", len(authIDs))
	fmt.Printf("SMARTY_MOCK: %s\n\n", mockEnv)

	// Create Smarty client
	validator := smarty.New(nil, smarty.Config{
		AuthIDs:    authIDs,
		AuthTokens: authTokens,
		Mock:       strings.ToLower(mockEnv) == "true",
	})

	// Fetch samples from both sources
	atmSamples, err := fetchRandomSamples(ctx, client, "ATMB", sampleSize)
	if err != nil {
		log.Fatalf("Failed to fetch ATMB samples: %v", err)
	}

	ipostSamples, err := fetchRandomSamples(ctx, client, "iPost1", sampleSize)
	if err != nil {
		log.Fatalf("Failed to fetch iPost1 samples: %v", err)
	}

	// Test ATMB samples
	fmt.Printf("=== Testing ATMB Samples (%d records) ===\n\n", len(atmSamples))
	testSamples(ctx, validator, atmSamples)

	// Test iPost1 samples
	fmt.Printf("\n=== Testing iPost1 Samples (%d records) ===\n\n", len(ipostSamples))
	testSamples(ctx, validator, ipostSamples)

	// Summary
	fmt.Println("\n=== Summary ===")
	fmt.Printf("ATMB records tested: %d\n", len(atmSamples))
	fmt.Printf("iPost1 records tested: %d\n", len(ipostSamples))
}

func fetchRandomSamples(ctx context.Context, client *firestore.Client, source string, count int) ([]model.Mailbox, error) {
	// Fetch all mailboxes from the specified source
	iter := client.Collection("mailboxes").
		Where("source", "==", source).
		Documents(ctx)

	var all []model.Mailbox
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate mailboxes: %w", err)
		}

		var mb model.Mailbox
		if err := doc.DataTo(&mb); err != nil {
			log.Printf("Error decoding mailbox %s: %v", doc.Ref.ID, err)
			continue
		}
		mb.ID = doc.Ref.ID
		all = append(all, mb)
	}

	if len(all) == 0 {
		fmt.Printf("No mailboxes found for source: %s\n", source)
		return nil, nil
	}

	fmt.Printf("Found %d total %s records\n", len(all), source)

	// Random shuffle and take first N
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(all), func(i, j int) {
		all[i], all[j] = all[j], all[i]
	})

	if len(all) > count {
		all = all[:count]
	}

	return all, nil
}

func testSamples(ctx context.Context, validator *smarty.Client, samples []model.Mailbox) {
	if len(samples) == 0 {
		fmt.Println("No samples to test.")
		return
	}

	successCount := 0
	failCount := 0

	for i, mb := range samples {
		fmt.Printf("--- Record %d ---\n", i+1)
		fmt.Printf("Name: %s\n", mb.Name)
		fmt.Printf("Address: %s, %s, %s %s\n",
			mb.AddressRaw.Street,
			mb.AddressRaw.City,
			mb.AddressRaw.State,
			mb.AddressRaw.Zip)
		fmt.Printf("Current DB values - CMRA: %q, RDI: %q\n", mb.CMRA, mb.RDI)

		// Call Smarty API
		validated, err := validator.ValidateMailbox(ctx, mb)
		if err != nil {
			fmt.Printf("ERROR: Smarty API call failed: %v\n", err)
			failCount++
			fmt.Println()
			continue
		}

		fmt.Printf("Smarty API result - CMRA: %q, RDI: %q\n", validated.CMRA, validated.RDI)
		if validated.StandardizedAddress.DeliveryLine1 != "" {
			fmt.Printf("Standardized: %s, %s\n",
				validated.StandardizedAddress.DeliveryLine1,
				validated.StandardizedAddress.LastLine)
		}
		fmt.Printf("LastValidatedAt: %s\n", validated.LastValidatedAt.Format(time.RFC3339))

		// Compare
		if mb.CMRA != validated.CMRA || mb.RDI != validated.RDI {
			fmt.Printf("CHANGE DETECTED: CMRA(%q->%q), RDI(%q->%q)\n",
				mb.CMRA, validated.CMRA,
				mb.RDI, validated.RDI)
		} else {
			fmt.Println("No change from current DB values")
		}

		successCount++
		fmt.Println()

		// Small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("Results: %d success, %d failed\n", successCount, failCount)
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
	// Try base64 encoded credentials first
	if b64 := strings.TrimSpace(os.Getenv("FIREBASE_CREDS_BASE64")); b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, "", fmt.Errorf("decode FIREBASE_CREDS_BASE64: %w", err)
		}
		return decoded, "base64", nil
	}

	// Try credentials file
	if file := strings.TrimSpace(os.Getenv("FIREBASE_CREDS_FILE")); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, "", fmt.Errorf("read FIREBASE_CREDS_FILE: %w", err)
		}
		return data, "file", nil
	}

	return nil, "", fmt.Errorf("no Firebase credentials found (set FIREBASE_CREDS_BASE64 or FIREBASE_CREDS_FILE)")
}
