package ipost1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	BaseURL           = "https://ipostal1.com"
	StatesEndpoint    = "/locations_ajax.php?action=get_states_list&country_id=223"
	LocationsEndpoint = "/locations_ajax.php?action=get_mail_centers&state_id=%s&country_id=223"
)

// StateResponse represents a US state/territory from the API.
type StateResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LocationsResponse contains HTML fragments with mailbox data.
type LocationsResponse struct {
	Display string `json:"display"` // HTML content with mailbox listings
}

// Client handles API requests to iPost1 using chromedp to bypass Cloudflare.
type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new iPost1 client with a browser context.
func NewClient() (*Client, error) {
	// Create chromedp context
	ctx, cancel := chromedp.NewContext(context.Background())

	// Set a reasonable timeout for the entire browser session
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Minute)

	return &Client{
		ctx: ctx,
		cancel: func() {
			timeoutCancel()
			cancel()
		},
	}, nil
}

// Close releases browser resources.
func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
}

// GetStates fetches the list of US states/territories.
func (c *Client) GetStates() ([]StateResponse, error) {
	var states []StateResponse

	// First, visit the homepage to establish session (bypass Cloudflare)
	if err := chromedp.Run(c.ctx,
		chromedp.Navigate(BaseURL),
		chromedp.Sleep(3*time.Second), // Wait for Cloudflare check
	); err != nil {
		return nil, fmt.Errorf("failed to visit homepage: %w", err)
	}

	// Now fetch the states API
	var responseBody string
	err := chromedp.Run(c.ctx,
		chromedp.Navigate(BaseURL+StatesEndpoint),
		chromedp.Sleep(2*time.Second),
		chromedp.Text("body", &responseBody, chromedp.NodeVisible),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch states: %w", err)
	}

	// Parse JSON response
	if err := json.Unmarshal([]byte(responseBody), &states); err != nil {
		return nil, fmt.Errorf("failed to parse states JSON: %w", err)
	}

	return states, nil
}

// GetLocationsByState fetches all mailbox locations for a specific state.
func (c *Client) GetLocationsByState(stateID string) (LocationsResponse, error) {
	var response LocationsResponse

	url := BaseURL + fmt.Sprintf(LocationsEndpoint, stateID)

	var responseBody string
	err := chromedp.Run(c.ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(2*time.Second),
		chromedp.Text("body", &responseBody, chromedp.NodeVisible),
	)
	if err != nil {
		return response, fmt.Errorf("failed to fetch locations for state %s: %w", stateID, err)
	}

	// Parse JSON response
	if err := json.Unmarshal([]byte(responseBody), &response); err != nil {
		return response, fmt.Errorf("failed to parse locations JSON: %w", err)
	}

	return response, nil
}
