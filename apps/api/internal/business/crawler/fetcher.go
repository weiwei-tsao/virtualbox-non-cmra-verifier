package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPFetcher fetches HTML pages over HTTP.
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher creates a fetcher with a sane timeout.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch url %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}
