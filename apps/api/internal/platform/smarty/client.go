package smarty

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

var (
	// ErrCircuitOpen signals the breaker is open after repeated 402/429 responses.
	ErrCircuitOpen = errors.New("smarty circuit open due to repeated rate/limit errors")
)

// HTTPClient matches net/http.Client Do signature for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client wraps Smarty calls with retry and circuit breaker support.
type Client struct {
	authID     string
	authToken  string
	baseURL    string
	httpClient HTTPClient
	mock       bool

	maxRetries       int
	breakerThreshold int
	consecutiveLimit int
}

// Config defines settings for the Smarty client.
type Config struct {
	AuthID     string
	AuthToken  string
	BaseURL    string
	Mock       bool
	MaxRetries int
	BreakerMax int
}

// New creates a Smarty client.
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

	return &Client{
		authID:           cfg.AuthID,
		authToken:        cfg.AuthToken,
		baseURL:          base,
		httpClient:       httpClient,
		mock:             cfg.Mock,
		maxRetries:       maxRetries,
		breakerThreshold: breaker,
	}
}

// ValidateMailbox calls Smarty (or mock) to enrich mailbox data.
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

	if c.consecutiveLimit >= c.breakerThreshold {
		return mailbox, ErrCircuitOpen
	}

	params := url.Values{}
	params.Set("auth-id", c.authID)
	params.Set("auth-token", c.authToken)
	params.Set("street", mailbox.AddressRaw.Street)
	params.Set("city", mailbox.AddressRaw.City)
	params.Set("state", mailbox.AddressRaw.State)
	params.Set("zipcode", mailbox.AddressRaw.Zip)

	endpoint := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return mailbox, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == c.maxRetries-1 {
				return mailbox, fmt.Errorf("request: %w", err)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			c.consecutiveLimit = 0
			return decodeSmartyResponse(mailbox, resp.Body)
		}

		if resp.StatusCode == http.StatusPaymentRequired || resp.StatusCode == http.StatusTooManyRequests {
			c.consecutiveLimit++
			if c.consecutiveLimit >= c.breakerThreshold {
				return mailbox, ErrCircuitOpen
			}
			continue
		}

		// For other errors, read body for context.
		body, _ := io.ReadAll(resp.Body)
		if attempt == c.maxRetries-1 {
			return mailbox, fmt.Errorf("smarty status %d: %s", resp.StatusCode, string(body))
		}
	}

	return mailbox, fmt.Errorf("smarty validation failed after retries")
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
	mailbox.CMRA = first.Analysis.CMRA
	mailbox.RDI = first.Analysis.RDI
	mailbox.LastValidatedAt = time.Now().UTC()
	return mailbox, nil
}

type smartyCandidate struct {
	DeliveryLine1 string         `json:"delivery_line_1"`
	LastLine      string         `json:"last_line"`
	Analysis      smartyAnalysis `json:"analysis"`
}

type smartyAnalysis struct {
	CMRA string `json:"cmra"`
	RDI  string `json:"rdi"`
}
