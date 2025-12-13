package util

import (
	"fmt"
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
	// First pass: clean HTML and escape sequences
	street := cleanField(addr.Street)
	city := cleanField(addr.City)
	state := cleanField(addr.State)
	zip := cleanField(addr.Zip)

	// Second pass: remove redundant city/state/zip from street
	street = removeRedundantCityStateZip(street, city, state, zip)

	return model.AddressRaw{
		Street: street,
		City:   city,
		State:  state,
		Zip:    zip,
	}
}

// removeRedundantCityStateZip removes duplicate city, state, zip info from street field.
// Example: "1601 29th St Boulder, CO 80301" -> "1601 29th St" (when city=Boulder, state=CO, zip=80301)
func removeRedundantCityStateZip(street, city, state, zip string) string {
	if street == "" || (city == "" && state == "") {
		return street
	}

	// Build possible redundant suffixes (most specific to least specific)
	var suffixes []string

	if city != "" && state != "" && zip != "" {
		// Full pattern: " Boulder, CO 80301"
		suffixes = append(suffixes, fmt.Sprintf(" %s, %s %s", city, state, zip))
		// Without comma: " Boulder CO 80301"
		suffixes = append(suffixes, fmt.Sprintf(" %s %s %s", city, state, zip))
	}

	if state != "" && zip != "" {
		// State and zip only: ", CO 80301"
		suffixes = append(suffixes, fmt.Sprintf(", %s %s", state, zip))
		// State and zip without comma: " CO 80301"
		suffixes = append(suffixes, fmt.Sprintf(" %s %s", state, zip))
	}

	if city != "" && state != "" {
		// City and state: " Boulder, CO"
		suffixes = append(suffixes, fmt.Sprintf(" %s, %s", city, state))
	}

	// Try to remove each suffix (case-insensitive comparison)
	streetLower := strings.ToLower(street)
	for _, suffix := range suffixes {
		suffixLower := strings.ToLower(suffix)
		if strings.HasSuffix(streetLower, suffixLower) {
			// Remove the suffix while preserving original case
			street = street[:len(street)-len(suffix)]
			break
		}
	}

	return strings.TrimSpace(street)
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

// NeedsCleanup checks if an address contains HTML remnants or redundant data that need cleaning.
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

	// Check for redundant city/state/zip in street
	if hasRedundantCityStateZip(addr.Street, addr.City, addr.State, addr.Zip) {
		return true
	}

	return false
}

// hasRedundantCityStateZip checks if street contains redundant city/state/zip.
func hasRedundantCityStateZip(street, city, state, zip string) bool {
	if street == "" || (city == "" && state == "") {
		return false
	}

	streetLower := strings.ToLower(street)

	// Check for common redundant patterns
	if city != "" && state != "" && zip != "" {
		pattern := strings.ToLower(fmt.Sprintf(" %s, %s %s", city, state, zip))
		if strings.HasSuffix(streetLower, pattern) {
			return true
		}
		pattern = strings.ToLower(fmt.Sprintf(" %s %s %s", city, state, zip))
		if strings.HasSuffix(streetLower, pattern) {
			return true
		}
	}

	if state != "" && zip != "" {
		pattern := strings.ToLower(fmt.Sprintf(", %s %s", state, zip))
		if strings.HasSuffix(streetLower, pattern) {
			return true
		}
	}

	return false
}
