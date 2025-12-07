package crawler

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// DiscoverLinks parses listing pages to extract ATMB detail links.
func DiscoverLinks(ctx context.Context, fetcher HTMLFetcher, seeds []string) ([]string, error) {
	seen := make(map[string]struct{})
	for _, seed := range seeds {
		body, err := fetcher.Fetch(ctx, seed)
		if err != nil {
			return nil, fmt.Errorf("fetch seed %s: %w", seed, err)
		}
		doc, err := goquery.NewDocumentFromReader(body)
		body.Close()
		if err != nil {
			return nil, fmt.Errorf("parse seed %s: %w", seed, err)
		}
		doc.Find("a").Each(func(_ int, s *goquery.Selection) {
			href, ok := s.Attr("href")
			if !ok {
				return
			}
			href = strings.TrimSpace(href)
			if href == "" {
				return
			}
			if !strings.Contains(href, "/locations/") {
				return
			}
			if _, exists := seen[href]; !exists {
				seen[href] = struct{}{}
			}
		})
	}
	links := make([]string, 0, len(seen))
	for link := range seen {
		links = append(links, link)
	}
	return links, nil
}
