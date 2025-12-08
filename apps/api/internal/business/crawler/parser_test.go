package crawler

import (
	"os"
	"testing"
)

func TestParseMailboxHTML(t *testing.T) {
	f, err := os.Open("testdata/sample_page.html")
	if err != nil {
		t.Fatalf("open sample html: %v", err)
	}
	defer f.Close()

	mailbox, err := ParseMailboxHTML(f, "https://anytimemailbox.com/locations/default")
	if err != nil {
		t.Fatalf("ParseMailboxHTML: %v", err)
	}

	if mailbox.Name != "Chicago - Monroe St" {
		t.Errorf("name = %q, want %q", mailbox.Name, "Chicago - Monroe St")
	}
	if mailbox.AddressRaw.Street != "73 W Monroe St" {
		t.Errorf("street = %q, want %q", mailbox.AddressRaw.Street, "73 W Monroe St")
	}
	if mailbox.AddressRaw.City != "Chicago" {
		t.Errorf("city = %q, want %q", mailbox.AddressRaw.City, "Chicago")
	}
	if mailbox.AddressRaw.State != "IL" {
		t.Errorf("state = %q, want %q", mailbox.AddressRaw.State, "IL")
	}
	if mailbox.AddressRaw.Zip != "60603" {
		t.Errorf("zip = %q, want %q", mailbox.AddressRaw.Zip, "60603")
	}
	if mailbox.Price != 19.99 {
		t.Errorf("price = %v, want 19.99", mailbox.Price)
	}
	if mailbox.Link != "https://anytimemailbox.com/locations/default" {
		t.Errorf("link = %q", mailbox.Link)
	}
}
