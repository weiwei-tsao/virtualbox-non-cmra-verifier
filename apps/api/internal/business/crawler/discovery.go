package crawler

import (
	"context"
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const baseURL = "https://www.anytimemailbox.com"
const detailBaseURL = "https://location.anytimemailbox.com"

// DiscoverLinks parses listing pages to extract ATMB detail links.
func DiscoverLinks(ctx context.Context, fetcher HTMLFetcher, seeds []string) ([]string, error) {
	seen := make(map[string]struct{})
	for _, seed := range seeds {
		body, err := fetcher.Fetch(ctx, seed)
		if err != nil {
			log.Printf("discover: fetch seed %s error: %v", seed, err)
			continue
		}
		doc, err := goquery.NewDocumentFromReader(body)
		body.Close()
		if err != nil {
			log.Printf("discover: parse seed %s error: %v", seed, err)
			continue
		}
		// Country page: find state links
		stateLinks := extractStateLinks(doc)
		if len(stateLinks) > 0 {
			for _, stateLink := range stateLinks {
				stateBody, err := fetcher.Fetch(ctx, stateLink)
				if err != nil {
					log.Printf("discover: fetch state %s error: %v", stateLink, err)
					continue
				}
				stateDoc, err := goquery.NewDocumentFromReader(stateBody)
				stateBody.Close()
				if err != nil {
					log.Printf("discover: parse state %s error: %v", stateLink, err)
					continue
				}
				addDetailLinks(stateDoc, seen)
			}
			continue
		}

		// State page: find detail links
		addDetailLinks(doc, seen)
	}
	links := make([]string, 0, len(seen))
	for link := range seen {
		links = append(links, link)
	}
	return links, nil
}

func extractStateLinks(doc *goquery.Document) []string {
	var links []string
	doc.Find("a.theme-simple-link").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" {
			return
		}
		if !strings.HasPrefix(href, "http") {
			href = baseURL + href
		}
		links = append(links, href)
	})
	return links
}

func addDetailLinks(doc *goquery.Document, seen map[string]struct{}) {
	doc.Find("a.gt-plan").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" {
			return
		}
		if !strings.HasPrefix(href, "http") {
			// Detail pages live under location.anytimemailbox.com
			if strings.HasPrefix(href, "/s/") {
				href = detailBaseURL + href
			} else {
				href = baseURL + href
			}
		}
		if _, exists := seen[href]; !exists {
			seen[href] = struct{}{}
		}
	})
}
