package crawler

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// ParseMailboxHTML extracts mailbox details from a single ATMB detail page HTML.
func ParseMailboxHTML(r io.Reader, sourceLink string) (model.Mailbox, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return model.Mailbox{}, fmt.Errorf("parse html: %w", err)
	}

	// Extract name from h1 tag (e.g., "Chicago - Monroe St")
	name := strings.TrimSpace(doc.Find("h1").First().Text())
	if name == "" {
		name = "YOUR NAME" // Default fallback
	}

	// Extract address lines from .t-text > div structure
	// Expected structure:
	//   Line 0: "73 W Monroe St" (street)
	//   Line 1: "5th Floor #MAILBOX" (suite/unit - optional)
	//   Line 2: "Chicago, IL 60603" (city, state, zip)
	//   Line 3: "United States" (country)
	var street, city, state, zip string
	var addressLines []string
	doc.Find(".t-text > div").Each(func(_ int, s *goquery.Selection) {
		txt := strings.TrimSpace(s.Text())
		if txt != "" && txt != "United States" {
			addressLines = append(addressLines, txt)
		}
	})

	// Parse address lines
	if len(addressLines) >= 2 {
		// First line is street (possibly with suite/unit on second line)
		street = addressLines[0]

		// Last line should be "City, State Zip" format
		cityStateZip := addressLines[len(addressLines)-1]
		parts := strings.Split(cityStateZip, ",")
		if len(parts) >= 2 {
			city = strings.TrimSpace(parts[0])
			stateZip := strings.Fields(strings.TrimSpace(parts[1]))
			if len(stateZip) >= 1 {
				state = stateZip[0]
			}
			if len(stateZip) >= 2 {
				zip = stateZip[1]
			}
		}
	}

	// Extract price from first plan (e.g., "US$ 19.99 / month")
	priceRaw := strings.TrimSpace(doc.Find(".t-plan .t-price").First().Text())

	price, err := parsePrice(priceRaw)
	if err != nil {
		// Some pages omit price; treat as zero instead of failing the record.
		price = 0
	}

	link := sourceLink
	if href, ok := doc.Find("a.store-link").First().Attr("href"); ok && strings.TrimSpace(href) != "" {
		link = strings.TrimSpace(href)
	}

	return model.Mailbox{
		Name: name,
		AddressRaw: model.AddressRaw{
			Street: street,
			City:   city,
			State:  state,
			Zip:    zip,
		},
		Price:  price,
		Link:   link,
		Active: true,
	}, nil
}

func parsePrice(raw string) (float64, error) {
	clean := make([]rune, 0, len(raw))
	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '.' {
			clean = append(clean, r)
		}
	}
	if len(clean) == 0 {
		// Missing price is acceptable; treat as zero.
		return 0, nil
	}
	return strconv.ParseFloat(string(clean), 64)
}
