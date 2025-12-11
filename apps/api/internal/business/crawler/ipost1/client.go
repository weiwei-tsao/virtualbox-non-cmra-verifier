package ipost1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	// Create chromedp options to avoid Cloudflare detection
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// Create chromedp context with the custom allocator
	ctx, cancel := chromedp.NewContext(allocCtx)

	// Set a reasonable timeout for the entire browser session
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Minute)

	return &Client{
		ctx: ctx,
		cancel: func() {
			timeoutCancel()
			cancel()
			allocCancel()
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
	// Wait longer for Cloudflare challenge to complete
	if err := chromedp.Run(c.ctx,
		chromedp.Navigate(BaseURL),
		chromedp.Sleep(8*time.Second), // Increased wait time for Cloudflare
	); err != nil {
		return nil, fmt.Errorf("failed to visit homepage: %w", err)
	}

	// Now fetch the states API
	var responseBody string
	err := chromedp.Run(c.ctx,
		chromedp.Navigate(BaseURL+StatesEndpoint),
		chromedp.Sleep(3*time.Second), // Increased wait time
		chromedp.Text("body", &responseBody, chromedp.NodeVisible),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch states: %w", err)
	}

	// Clean up response (Text mode should give us proper JSON)
	responseBody = strings.TrimSpace(responseBody)

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

	// Get the raw HTML response
	var rawHTML string
	err := chromedp.Run(c.ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(3*time.Second),
		chromedp.InnerHTML("body", &rawHTML, chromedp.NodeVisible),
	)
	if err != nil {
		return response, fmt.Errorf("failed to fetch locations for state %s: %w", stateID, err)
	}

	// The API returns malformed JSON that can't be parsed by any standard parser.
	// Instead of parsing JSON, we'll extract the display HTML content using string operations.
	// The format is: {"num_results":X,"num_results_text":"...","display":"<HTML>","searched":"","back":"..."}

	// Find the display field content between "display":" and ","searched"
	displayStart := strings.Index(rawHTML, `"display":"`)
	if displayStart == -1 {
		// No display field - might be empty result
		return response, nil
	}
	displayStart += len(`"display":"`)

	// Find the end marker
	displayEnd := strings.Index(rawHTML[displayStart:], `","searched"`)
	if displayEnd == -1 {
		return response, fmt.Errorf("malformed response for state %s: no searched field", stateID)
	}

	// Extract and unescape the HTML content
	displayHTML := rawHTML[displayStart : displayStart+displayEnd]

	// Unescape in correct order to avoid double-quotes
	// The malformed JSON has both \" and &quot; which creates ""
	// First remove the problematic \&quot; sequences (these are extraneous)
	displayHTML = strings.ReplaceAll(displayHTML, `\&quot;`, ``)
	displayHTML = strings.ReplaceAll(displayHTML, `&quot;`, ``)
	// Then unescape standard JSON escapes (including escaped backslashes)
	displayHTML = strings.ReplaceAll(displayHTML, `\\`, `\`)
	displayHTML = strings.ReplaceAll(displayHTML, `\n`, "\n")
	displayHTML = strings.ReplaceAll(displayHTML, `\"`, `"`)

	response.Display = displayHTML
	return response, nil
}
