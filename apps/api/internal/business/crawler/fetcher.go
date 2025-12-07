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
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.anytimemailbox.com/")

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp.Body, nil
		}
		lastErr = fmt.Errorf("status %d for %s", resp.StatusCode, url)
		resp.Body.Close()
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	return nil, fmt.Errorf("fetch url %s: %w", url, lastErr)
}
