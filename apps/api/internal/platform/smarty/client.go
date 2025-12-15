package smarty

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

var (
	// ErrCircuitOpen signals the breaker is open after repeated 402/429 responses.
	ErrCircuitOpen = errors.New("smarty circuit open due to repeated rate/limit errors")
	// ErrAllCredentialsExhausted signals all credentials have hit their circuit breaker.
	ErrAllCredentialsExhausted = errors.New("all smarty credentials exhausted")
)

// HTTPClient matches net/http.Client Do signature for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// credential represents a single Smarty API account.
type credential struct {
	authID           string
	authToken        string
	consecutiveLimit int // Circuit breaker counter for this credential
}

// Client wraps Smarty calls with retry and circuit breaker support.
// Supports multiple credentials with round-robin load balancing.
type Client struct {
	credentials      []credential
	currentIndex     int // Round-robin index
	mu               sync.Mutex
	baseURL          string
	httpClient       HTTPClient
	mock             bool
	maxRetries       int
	breakerThreshold int
}

// Config defines settings for the Smarty client.
type Config struct {
	AuthIDs    []string // Multiple auth IDs for load balancing
	AuthTokens []string // Multiple auth tokens (must match IDs length)
	BaseURL    string
	Mock       bool
	MaxRetries int
	BreakerMax int
}

// New creates a Smarty client with support for multiple credentials.
func New(httpClient HTTPClient, cfg Config) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://us-street.api.smarty.com/street-address"
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	breaker := cfg.BreakerMax
	if breaker <= 0 {
		breaker = 5
	}

	// Build credentials slice from AuthIDs and AuthTokens
	credentials := make([]credential, len(cfg.AuthIDs))
	for i := range cfg.AuthIDs {
		credentials[i] = credential{
			authID:    cfg.AuthIDs[i],
			authToken: cfg.AuthTokens[i],
		}
	}

	return &Client{
		credentials:      credentials,
		currentIndex:     0,
		baseURL:          base,
		httpClient:       httpClient,
		mock:             cfg.Mock,
		maxRetries:       maxRetries,
		breakerThreshold: breaker,
	}
}

// ValidateMailbox calls Smarty (or mock) to enrich mailbox data.
// Uses round-robin load balancing across multiple credentials.
func (c *Client) ValidateMailbox(ctx context.Context, mailbox model.Mailbox) (model.Mailbox, error) {
	if c.mock {
		mailbox.CMRA = "Y"
		mailbox.RDI = "Commercial"
		mailbox.StandardizedAddress = model.StandardizedAddress{
			DeliveryLine1: mailbox.AddressRaw.Street,
			LastLine:      fmt.Sprintf("%s, %s %s", mailbox.AddressRaw.City, mailbox.AddressRaw.State, mailbox.AddressRaw.Zip),
		}
		mailbox.LastValidatedAt = time.Now().UTC()
		return mailbox, nil
	}

	if len(c.credentials) == 0 {
		return mailbox, errors.New("no smarty credentials configured")
	}

	// Try each credential in round-robin order
	startIndex := c.getNextCredentialIndex()
	triedCount := 0

	for triedCount < len(c.credentials) {
		credIndex := (startIndex + triedCount) % len(c.credentials)
		cred := &c.credentials[credIndex]

		// Check if this credential's circuit breaker is open
		if cred.consecutiveLimit >= c.breakerThreshold {
			log.Printf("Credential %s circuit breaker open (%d/%d), trying next",
				maskAuthID(cred.authID), cred.consecutiveLimit, c.breakerThreshold)
			triedCount++
			continue
		}

		// Try validation with this credential
		result, err := c.validateWithCredential(ctx, mailbox, cred)
		if err == nil {
			// Success - reset circuit breaker and return
			cred.consecutiveLimit = 0
			return result, nil
		}

		// Check if error is rate limit or quota exhausted
		if errors.Is(err, ErrCircuitOpen) {
			log.Printf("Credential %s hit circuit breaker, trying next", maskAuthID(cred.authID))
			triedCount++
			continue
		}

		// For other errors, return immediately
		return mailbox, err
	}

	// All credentials exhausted
	return mailbox, ErrAllCredentialsExhausted
}

// getNextCredentialIndex returns the next credential index using round-robin.
func (c *Client) getNextCredentialIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	index := c.currentIndex
	c.currentIndex = (c.currentIndex + 1) % len(c.credentials)
	return index
}

// validateWithCredential performs validation using a specific credential.
func (c *Client) validateWithCredential(ctx context.Context, mailbox model.Mailbox, cred *credential) (model.Mailbox, error) {
	params := url.Values{}
	params.Set("auth-id", cred.authID)
	params.Set("auth-token", cred.authToken)
	params.Set("street", mailbox.AddressRaw.Street)
	params.Set("city", mailbox.AddressRaw.City)
	params.Set("state", mailbox.AddressRaw.State)
	params.Set("zipcode", mailbox.AddressRaw.Zip)

	endpoint := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return mailbox, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == c.maxRetries-1 {
				return mailbox, fmt.Errorf("request: %w", err)
			}
			time.Sleep(100 * time.Millisecond) // Brief backoff
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return decodeSmartyResponse(mailbox, resp.Body)
		}

		// Handle rate limiting or quota exhaustion
		if resp.StatusCode == http.StatusPaymentRequired || resp.StatusCode == http.StatusTooManyRequests {
			c.mu.Lock()
			cred.consecutiveLimit++
			limit := cred.consecutiveLimit
			c.mu.Unlock()

			log.Printf("Credential %s rate limited (status %d), count: %d/%d",
				maskAuthID(cred.authID), resp.StatusCode, limit, c.breakerThreshold)

			if limit >= c.breakerThreshold {
				return mailbox, ErrCircuitOpen
			}

			time.Sleep(500 * time.Millisecond) // Backoff before retry
			continue
		}

		// For other errors, read body for context
		body, _ := io.ReadAll(resp.Body)
		if attempt == c.maxRetries-1 {
			return mailbox, fmt.Errorf("smarty status %d: %s", resp.StatusCode, string(body))
		}

		time.Sleep(200 * time.Millisecond) // Backoff before retry
	}

	return mailbox, fmt.Errorf("smarty validation failed after %d retries", c.maxRetries)
}

// maskAuthID masks the auth ID for logging (shows first 8 chars only).
func maskAuthID(authID string) string {
	if len(authID) <= 8 {
		return authID
	}
	return authID[:8] + "..."
}

func decodeSmartyResponse(mailbox model.Mailbox, body io.Reader) (model.Mailbox, error) {
	var candidates []smartyCandidate
	buf, err := io.ReadAll(body)
	if err != nil {
		return mailbox, fmt.Errorf("read response: %w", err)
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf), &candidates); err != nil {
		return mailbox, fmt.Errorf("decode response: %w", err)
	}
	if len(candidates) == 0 {
		return mailbox, errors.New("smarty: no candidates returned")
	}
	first := candidates[0]
	mailbox.StandardizedAddress = model.StandardizedAddress{
		DeliveryLine1: first.DeliveryLine1,
		LastLine:      first.LastLine,
	}
	// CMRA is in analysis.dpv_cmra, RDI is in metadata.rdi
	mailbox.CMRA = first.Analysis.DPVCMRA
	mailbox.RDI = first.Metadata.RDI
	mailbox.LastValidatedAt = time.Now().UTC()
	return mailbox, nil
}

type smartyCandidate struct {
	DeliveryLine1 string          `json:"delivery_line_1"`
	LastLine      string          `json:"last_line"`
	Metadata      smartyMetadata  `json:"metadata"`
	Analysis      smartyAnalysis  `json:"analysis"`
}

type smartyMetadata struct {
	RDI string `json:"rdi"` // "Commercial" or "Residential"
}

type smartyAnalysis struct {
	DPVCMRA string `json:"dpv_cmra"` // "Y" or "N"
}

// ============================================================================
// Batch Validation Support
// ============================================================================

const maxBatchSize = 100 // Smarty API limit: 100 addresses per POST request

// batchRequest represents a single address in a batch POST request.
type batchRequest struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zipcode string `json:"zipcode"`
}

// batchResponseItem represents a single result in a batch response.
type batchResponseItem struct {
	InputIndex    int            `json:"input_index"`
	DeliveryLine1 string         `json:"delivery_line_1"`
	LastLine      string         `json:"last_line"`
	Metadata      smartyMetadata `json:"metadata"`
	Analysis      smartyAnalysis `json:"analysis"`
}

// ValidateMailboxBatch validates multiple mailboxes in batch using POST requests.
// This is significantly more efficient than individual calls (up to 100 addresses per request).
func (c *Client) ValidateMailboxBatch(ctx context.Context, mailboxes []model.Mailbox) ([]model.Mailbox, error) {
	if len(mailboxes) == 0 {
		return mailboxes, nil
	}

	if c.mock {
		return c.mockBatchValidation(mailboxes), nil
	}

	if len(c.credentials) == 0 {
		return nil, errors.New("no smarty credentials configured")
	}

	// Copy mailboxes to preserve original data
	results := make([]model.Mailbox, len(mailboxes))
	copy(results, mailboxes)

	// Process in chunks of maxBatchSize
	for start := 0; start < len(mailboxes); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(mailboxes) {
			end = len(mailboxes)
		}

		chunk := mailboxes[start:end]
		chunkResults, err := c.validateChunk(ctx, chunk)
		if err != nil {
			// On error, return partial results with original data for failed chunk
			log.Printf("batch validation chunk [%d:%d] failed: %v", start, end, err)
			return results, fmt.Errorf("batch validation chunk [%d:%d]: %w", start, end, err)
		}

		// Merge results back
		for i, res := range chunkResults {
			results[start+i] = res
		}
	}

	return results, nil
}

// validateChunk validates a single chunk of mailboxes (up to 100).
func (c *Client) validateChunk(ctx context.Context, mailboxes []model.Mailbox) ([]model.Mailbox, error) {
	// Build request body
	reqBody := make([]batchRequest, len(mailboxes))
	for i, mb := range mailboxes {
		reqBody[i] = batchRequest{
			Street:  mb.AddressRaw.Street,
			City:    mb.AddressRaw.City,
			State:   mb.AddressRaw.State,
			Zipcode: mb.AddressRaw.Zip,
		}
	}

	// Try credentials with round-robin and circuit breaker
	startIndex := c.getNextCredentialIndex()
	triedCount := 0

	for triedCount < len(c.credentials) {
		credIndex := (startIndex + triedCount) % len(c.credentials)
		cred := &c.credentials[credIndex]

		if cred.consecutiveLimit >= c.breakerThreshold {
			log.Printf("Credential %s circuit breaker open (%d/%d), trying next",
				maskAuthID(cred.authID), cred.consecutiveLimit, c.breakerThreshold)
			triedCount++
			continue
		}

		results, err := c.postBatchWithCredential(ctx, mailboxes, reqBody, cred)
		if err == nil {
			cred.consecutiveLimit = 0
			return results, nil
		}

		if errors.Is(err, ErrCircuitOpen) {
			log.Printf("Credential %s hit circuit breaker, trying next", maskAuthID(cred.authID))
			triedCount++
			continue
		}

		return mailboxes, err
	}

	return mailboxes, ErrAllCredentialsExhausted
}

// postBatchWithCredential sends a batch POST request using a specific credential.
func (c *Client) postBatchWithCredential(ctx context.Context, mailboxes []model.Mailbox, reqBody []batchRequest, cred *credential) ([]model.Mailbox, error) {
	// Build URL with auth params
	params := url.Values{}
	params.Set("auth-id", cred.authID)
	params.Set("auth-token", cred.authToken)
	endpoint := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	// Serialize request body
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return mailboxes, fmt.Errorf("marshal batch request: %w", err)
	}

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
		if err != nil {
			return mailboxes, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == c.maxRetries-1 {
				return mailboxes, fmt.Errorf("request: %w", err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return decodeBatchResponse(mailboxes, resp.Body)
		}

		// Handle rate limiting
		if resp.StatusCode == http.StatusPaymentRequired || resp.StatusCode == http.StatusTooManyRequests {
			c.mu.Lock()
			cred.consecutiveLimit++
			limit := cred.consecutiveLimit
			c.mu.Unlock()

			log.Printf("Credential %s rate limited (status %d), count: %d/%d",
				maskAuthID(cred.authID), resp.StatusCode, limit, c.breakerThreshold)

			if limit >= c.breakerThreshold {
				return mailboxes, ErrCircuitOpen
			}

			time.Sleep(500 * time.Millisecond)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		if attempt == c.maxRetries-1 {
			return mailboxes, fmt.Errorf("smarty batch status %d: %s", resp.StatusCode, string(body))
		}

		time.Sleep(200 * time.Millisecond)
	}

	return mailboxes, fmt.Errorf("smarty batch validation failed after %d retries", c.maxRetries)
}

// decodeBatchResponse parses the batch API response and maps results back to mailboxes.
func decodeBatchResponse(mailboxes []model.Mailbox, body io.Reader) ([]model.Mailbox, error) {
	var responses []batchResponseItem
	buf, err := io.ReadAll(body)
	if err != nil {
		return mailboxes, fmt.Errorf("read batch response: %w", err)
	}

	if err := json.Unmarshal(bytes.TrimSpace(buf), &responses); err != nil {
		return mailboxes, fmt.Errorf("decode batch response: %w", err)
	}

	// Copy mailboxes to avoid modifying input
	results := make([]model.Mailbox, len(mailboxes))
	copy(results, mailboxes)

	// Map responses back to mailboxes by input_index
	now := time.Now().UTC()
	for _, resp := range responses {
		if resp.InputIndex < 0 || resp.InputIndex >= len(results) {
			continue // Skip invalid indices
		}

		mb := &results[resp.InputIndex]
		mb.StandardizedAddress = model.StandardizedAddress{
			DeliveryLine1: resp.DeliveryLine1,
			LastLine:      resp.LastLine,
		}
		mb.CMRA = resp.Analysis.DPVCMRA
		mb.RDI = resp.Metadata.RDI
		mb.LastValidatedAt = now
	}

	return results, nil
}

// mockBatchValidation returns mock data for batch validation.
func (c *Client) mockBatchValidation(mailboxes []model.Mailbox) []model.Mailbox {
	results := make([]model.Mailbox, len(mailboxes))
	now := time.Now().UTC()
	for i, mb := range mailboxes {
		mb.CMRA = "Y"
		mb.RDI = "Commercial"
		mb.StandardizedAddress = model.StandardizedAddress{
			DeliveryLine1: mb.AddressRaw.Street,
			LastLine:      fmt.Sprintf("%s, %s %s", mb.AddressRaw.City, mb.AddressRaw.State, mb.AddressRaw.Zip),
		}
		mb.LastValidatedAt = now
		results[i] = mb
	}
	return results
}
