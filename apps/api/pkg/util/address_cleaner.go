package util

import (
	"regexp"
	"strings"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

var (
	// htmlTagPattern matches HTML tags like <span>, </span>, <div>, </div>, etc.
	htmlTagPattern = regexp.MustCompile(`<[^>]*>`)
	// multiSpacePattern matches multiple consecutive whitespace characters
	multiSpacePattern = regexp.MustCompile(`\s+`)
)

// CleanAddress removes HTML remnants and normalizes an AddressRaw struct.
// This function is designed to handle malformed data from web scrapers.
func CleanAddress(addr model.AddressRaw) model.AddressRaw {
	return model.AddressRaw{
		Street: cleanField(addr.Street),
		City:   cleanField(addr.City),
		State:  cleanField(addr.State),
		Zip:    cleanField(addr.Zip),
	}
}

// CleanStandardizedAddress removes HTML remnants from StandardizedAddress.
func CleanStandardizedAddress(addr model.StandardizedAddress) model.StandardizedAddress {
	return model.StandardizedAddress{
		DeliveryLine1: cleanField(addr.DeliveryLine1),
		LastLine:      cleanField(addr.LastLine),
	}
}

// CleanLink fixes escaped URLs (e.g., https:\/\/ -> https://)
func CleanLink(link string) string {
	// Fix escaped forward slashes in URLs
	link = strings.ReplaceAll(link, `\/`, `/`)
	return strings.TrimSpace(link)
}

// cleanField removes HTML tags, escape sequences, and normalizes whitespace.
func cleanField(s string) string {
	if s == "" {
		return ""
	}

	// 1. Fix escaped HTML closing tags: <\/ -> </
	s = strings.ReplaceAll(s, `<\/`, `</`)

	// 2. Fix escaped forward slashes
	s = strings.ReplaceAll(s, `\/`, `/`)

	// 3. Remove HTML tags
	s = htmlTagPattern.ReplaceAllString(s, "")

	// 4. Decode common HTML entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")

	// 5. Remove "United States" suffix (common in iPost1 data)
	s = strings.ReplaceAll(s, "United States", "")

	// 6. Normalize whitespace (collapse multiple spaces/newlines into single space)
	s = multiSpacePattern.ReplaceAllString(s, " ")

	// 7. Trim leading/trailing whitespace
	return strings.TrimSpace(s)
}

// NeedsCleanup checks if an address contains HTML remnants that need cleaning.
func NeedsCleanup(addr model.AddressRaw) bool {
	fields := []string{addr.Street, addr.City, addr.State, addr.Zip}
	for _, f := range fields {
		if strings.Contains(f, "<") ||
			strings.Contains(f, `\/`) ||
			strings.Contains(f, `\n`) ||
			strings.Contains(f, "United States") {
			return true
		}
	}
	return false
}
