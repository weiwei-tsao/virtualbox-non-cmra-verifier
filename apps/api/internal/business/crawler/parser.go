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
	street := strings.TrimSpace(doc.Find(".address .street").First().Text())
	city := strings.TrimSpace(doc.Find(".address .city").First().Text())
	state := strings.TrimSpace(doc.Find(".address .state").First().Text())
	zip := strings.TrimSpace(doc.Find(".address .zip").First().Text())
	priceRaw := strings.TrimSpace(doc.Find(".price").First().Text())

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
