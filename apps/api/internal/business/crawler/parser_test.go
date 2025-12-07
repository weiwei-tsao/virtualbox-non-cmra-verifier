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

	if mailbox.Name != "ABC Mailbox Store" {
		t.Errorf("name = %q, want %q", mailbox.Name, "ABC Mailbox Store")
	}
	if mailbox.AddressRaw.Street != "123 Main St Suite 100" {
		t.Errorf("street = %q", mailbox.AddressRaw.Street)
	}
	if mailbox.AddressRaw.City != "Dover" || mailbox.AddressRaw.State != "DE" || mailbox.AddressRaw.Zip != "19901" {
		t.Errorf("address = %+v", mailbox.AddressRaw)
	}
	if mailbox.Price != 12.99 {
		t.Errorf("price = %v, want 12.99", mailbox.Price)
	}
	if mailbox.Link != "https://anytimemailbox.com/locations/abc-mailbox-store" {
		t.Errorf("link = %q", mailbox.Link)
	}
}
