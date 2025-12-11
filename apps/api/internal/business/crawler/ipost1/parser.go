package ipost1

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// ParseLocationsHTML extracts mailbox data from iPost1 HTML fragments.
// The HTML contains <article class="mail-center-card"> elements with mailbox details.
func ParseLocationsHTML(htmlContent string) ([]model.Mailbox, error) {
	if htmlContent == "" {
		return nil, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var mailboxes []model.Mailbox

	// Each mailbox is in an <article class="mail-center-card">
	doc.Find("article.mail-center-card").Each(func(i int, s *goquery.Selection) {
		var mb model.Mailbox

		// Extract location name from the title
		name := strings.TrimSpace(s.Find(".store-name").Text())

		// Extract street address - skip the label "Street Address:"
		streetHTML, _ := s.Find(".store-street-address").Html()
		street := extractTextAfterLabel(streetHTML)

		// Extract city, state, zip - skip the label "City, State Zip:"
		cityStateZipHTML, _ := s.Find(".store-city-state-zip").Html()
		cityStateZip := extractTextAfterLabel(cityStateZipHTML)
		city, state, zip := parseCityStateZip(cityStateZip)

		// Extract price from the desktop view
		priceText := strings.TrimSpace(s.Find(".store-plan-desktop b").Text())
		price := parsePrice(priceText)

		// Extract link to checkout page
		link, _ := s.Find("a[href*='secure_checkout']").Attr("href")
		if link != "" && !strings.HasPrefix(link, "http") {
			link = BaseURL + link
		}

		// Only add if we have minimum required fields
		if name != "" && street != "" && city != "" {
			mb.Name = name
			mb.AddressRaw = model.AddressRaw{
				Street: street,
				City:   city,
				State:  state,
				Zip:    zip,
			}
			mb.Price = price
			mb.Link = link
			mb.Source = "iPost1"

			mailboxes = append(mailboxes, mb)
		}
	})

	return mailboxes, nil
}

// extractTextAfterLabel removes HTML labels and placeholders, extracting only the actual text.
// Example: "<span>Street Address:</span> 123 Main St" -> "123 Main St"
func extractTextAfterLabel(htmlContent string) string {
	// Remove HTML tags
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return strings.TrimSpace(htmlContent)
	}

	text := doc.Text()

	// Remove common labels
	labels := []string{"Street Address:", "City, State Zip:", "Address:"}
	for _, label := range labels {
		text = strings.Replace(text, label, "", 1)
	}

	return strings.TrimSpace(text)
}

// parseCityStateZip splits "San Francisco, CA 94102" into separate fields.
func parseCityStateZip(input string) (city, state, zip string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Pattern: "City, ST ZIP"
	parts := strings.Split(input, ",")
	if len(parts) < 2 {
		return
	}

	city = strings.TrimSpace(parts[0])

	// Parse state and zip from "CA 94102"
	stateZip := strings.TrimSpace(parts[1])
	fields := strings.Fields(stateZip)
	if len(fields) >= 1 {
		state = fields[0]
	}
	if len(fields) >= 2 {
		zip = fields[1]
	}

	return
}

// parsePrice extracts numeric price from text like "$15.95/month" or "15.95".
func parsePrice(input string) float64 {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0.0
	}

	// Remove currency symbols, "per month", etc.
	input = strings.ReplaceAll(input, "$", "")
	input = strings.ReplaceAll(input, "USD", "")
	input = strings.ToLower(input)
	input = strings.Split(input, "/")[0] // Remove "/month" suffix
	input = strings.TrimSpace(input)

	// Extract first number
	re := regexp.MustCompile(`[\d.]+`)
	match := re.FindString(input)
	if match == "" {
		return 0.0
	}

	price, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0.0
	}

	return price
}
