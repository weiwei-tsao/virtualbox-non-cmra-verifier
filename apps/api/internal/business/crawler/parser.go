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

	name := strings.TrimSpace(doc.Find(".mailbox-name").First().Text())
	if name == "" {
		name = strings.TrimSpace(doc.Find("h1.t-title").First().Text())
	}

	street := strings.TrimSpace(doc.Find(".address .street").First().Text())
	city := strings.TrimSpace(doc.Find(".address .city").First().Text())
	state := strings.TrimSpace(doc.Find(".address .state").First().Text())
	zip := strings.TrimSpace(doc.Find(".address .zip").First().Text())

	// Fallback for ATMB detail page structure (t-sec1 > t-text > div)
	if street == "" || city == "" || state == "" || zip == "" {
		var lines []string
		doc.Find("div.t-sec1 div.t-text div").Each(func(_ int, s *goquery.Selection) {
			txt := strings.TrimSpace(s.Text())
			if txt != "" {
				lines = append(lines, txt)
			}
		})
		if len(lines) >= 3 {
			street = lines[0]
			line2 := lines[1]
			parts := strings.Split(line2, ",")
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
	}

	priceRaw := strings.TrimSpace(doc.Find(".price, .t-price, .t-plan-price").First().Text())

	price, err := parsePrice(priceRaw)
	if err != nil {
		return model.Mailbox{}, fmt.Errorf("parse price: %w", err)
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
		return 0, fmt.Errorf("no price found in %q", raw)
	}
	return strconv.ParseFloat(string(clean), 64)
}
