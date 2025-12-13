package ipost1

import (
	"testing"
)

func TestParseLocationsHTML(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		wantCount   int
		wantFirst   map[string]string // Expected fields of first mailbox
		wantErr     bool
	}{
		{
			name:      "empty HTML returns nil",
			html:      "",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "single mailbox card",
			html: `
				<article class="mail-center-card">
					<div class="store-name">Test Location</div>
					<div class="store-street-address"><span>Street Address:</span> 123 Main St</div>
					<div class="store-city-state-zip"><span>City, State Zip:</span> San Francisco, CA 94102</div>
					<div class="store-plan-desktop"><b>$15.95/month</b></div>
					<a href="/secure_checkout?id=123">Sign Up</a>
				</article>
			`,
			wantCount: 1,
			wantFirst: map[string]string{
				"name":   "Test Location",
				"street": "123 Main St",
				"city":   "San Francisco",
				"state":  "CA",
				"zip":    "94102",
			},
			wantErr: false,
		},
		{
			name: "mailbox without name uses city/state fallback",
			html: `
				<article class="mail-center-card">
					<div class="store-street-address"><span>Street Address:</span> 456 Oak Ave</div>
					<div class="store-city-state-zip"><span>City, State Zip:</span> Austin, TX 78701</div>
				</article>
			`,
			wantCount: 1,
			wantFirst: map[string]string{
				"name":   "iPost1 - Austin, TX",
				"street": "456 Oak Ave",
				"city":   "Austin",
				"state":  "TX",
				"zip":    "78701",
			},
			wantErr: false,
		},
		{
			name: "multiple mailbox cards",
			html: `
				<article class="mail-center-card">
					<div class="store-street-address"><span>Street Address:</span> 111 First St</div>
					<div class="store-city-state-zip"><span>City, State Zip:</span> Denver, CO 80202</div>
				</article>
				<article class="mail-center-card">
					<div class="store-street-address"><span>Street Address:</span> 222 Second St</div>
					<div class="store-city-state-zip"><span>City, State Zip:</span> Boulder, CO 80301</div>
				</article>
			`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "skips card without required fields",
			html: `
				<article class="mail-center-card">
					<div class="store-name">Incomplete Location</div>
				</article>
			`,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mailboxes, err := ParseLocationsHTML(tt.html)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLocationsHTML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(mailboxes) != tt.wantCount {
				t.Errorf("ParseLocationsHTML() got %d mailboxes, want %d", len(mailboxes), tt.wantCount)
				return
			}

			if tt.wantFirst != nil && len(mailboxes) > 0 {
				mb := mailboxes[0]
				if mb.Name != tt.wantFirst["name"] {
					t.Errorf("Name = %q, want %q", mb.Name, tt.wantFirst["name"])
				}
				if mb.AddressRaw.Street != tt.wantFirst["street"] {
					t.Errorf("Street = %q, want %q", mb.AddressRaw.Street, tt.wantFirst["street"])
				}
				if mb.AddressRaw.City != tt.wantFirst["city"] {
					t.Errorf("City = %q, want %q", mb.AddressRaw.City, tt.wantFirst["city"])
				}
				if mb.AddressRaw.State != tt.wantFirst["state"] {
					t.Errorf("State = %q, want %q", mb.AddressRaw.State, tt.wantFirst["state"])
				}
				if mb.AddressRaw.Zip != tt.wantFirst["zip"] {
					t.Errorf("Zip = %q, want %q", mb.AddressRaw.Zip, tt.wantFirst["zip"])
				}
				if mb.Source != "iPost1" {
					t.Errorf("Source = %q, want %q", mb.Source, "iPost1")
				}
			}
		})
	}
}

func TestParseCityStateZip(t *testing.T) {
	tests := []struct {
		input     string
		wantCity  string
		wantState string
		wantZip   string
	}{
		{
			input:     "San Francisco, CA 94102",
			wantCity:  "San Francisco",
			wantState: "CA",
			wantZip:   "94102",
		},
		{
			input:     "New York, NY 10001",
			wantCity:  "New York",
			wantState: "NY",
			wantZip:   "10001",
		},
		{
			input:     "Austin, TX 78701",
			wantCity:  "Austin",
			wantState: "TX",
			wantZip:   "78701",
		},
		{
			input:     "",
			wantCity:  "",
			wantState: "",
			wantZip:   "",
		},
		{
			input:     "InvalidFormat",
			wantCity:  "",
			wantState: "",
			wantZip:   "",
		},
		{
			input:     "City, ST",
			wantCity:  "City",
			wantState: "ST",
			wantZip:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			city, state, zip := parseCityStateZip(tt.input)
			if city != tt.wantCity {
				t.Errorf("city = %q, want %q", city, tt.wantCity)
			}
			if state != tt.wantState {
				t.Errorf("state = %q, want %q", state, tt.wantState)
			}
			if zip != tt.wantZip {
				t.Errorf("zip = %q, want %q", zip, tt.wantZip)
			}
		})
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"$15.95/month", 15.95},
		{"$9.99", 9.99},
		{"15.95", 15.95},
		{"$29.95 USD/month", 29.95},
		{"", 0.0},
		{"Free", 0.0},
		{"$0.00", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parsePrice(tt.input)
			if got != tt.want {
				t.Errorf("parsePrice(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractTextAfterLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes Street Address label",
			input: "<span>Street Address:</span> 123 Main St",
			want:  "123 Main St",
		},
		{
			name:  "removes City State Zip label",
			input: "<span>City, State Zip:</span> Denver, CO 80202",
			want:  "Denver, CO 80202",
		},
		{
			name:  "handles plain text",
			input: "123 Main St",
			want:  "123 Main St",
		},
		{
			name:  "handles empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextAfterLabel(tt.input)
			if got != tt.want {
				t.Errorf("extractTextAfterLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
