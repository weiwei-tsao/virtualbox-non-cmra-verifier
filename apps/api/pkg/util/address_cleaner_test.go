package util

import (
	"testing"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

func TestCleanAddress(t *testing.T) {
	tests := []struct {
		name string
		addr model.AddressRaw
		want model.AddressRaw
	}{
		{
			name: "removes HTML tags from street",
			addr: model.AddressRaw{
				Street: "123 Main St<wbr><span></span>",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
			want: model.AddressRaw{
				Street: "123 Main St",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
		},
		{
			name: "removes redundant city/state/zip from street",
			addr: model.AddressRaw{
				Street: "1601 29th St Boulder, CO 80301",
				City:   "Boulder",
				State:  "CO",
				Zip:    "80301",
			},
			want: model.AddressRaw{
				Street: "1601 29th St",
				City:   "Boulder",
				State:  "CO",
				Zip:    "80301",
			},
		},
		{
			name: "fixes escaped HTML closing tags",
			addr: model.AddressRaw{
				Street: "123 Main St<\\/span>",
				City:   "Austin",
				State:  "TX",
				Zip:    "78701",
			},
			want: model.AddressRaw{
				Street: "123 Main St",
				City:   "Austin",
				State:  "TX",
				Zip:    "78701",
			},
		},
		{
			name: "removes United States suffix",
			addr: model.AddressRaw{
				Street: "456 Oak Ave",
				City:   "Portland United States",
				State:  "OR",
				Zip:    "97201",
			},
			want: model.AddressRaw{
				Street: "456 Oak Ave",
				City:   "Portland",
				State:  "OR",
				Zip:    "97201",
			},
		},
		{
			name: "normalizes multiple spaces",
			addr: model.AddressRaw{
				Street: "123   Main    St",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
			want: model.AddressRaw{
				Street: "123 Main St",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
		},
		{
			name: "decodes HTML entities",
			addr: model.AddressRaw{
				Street: "123 Main St &amp; Oak Ave",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
			want: model.AddressRaw{
				Street: "123 Main St & Oak Ave",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
		},
		{
			name: "handles empty address",
			addr: model.AddressRaw{},
			want: model.AddressRaw{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanAddress(tt.addr)
			if got.Street != tt.want.Street {
				t.Errorf("Street = %q, want %q", got.Street, tt.want.Street)
			}
			if got.City != tt.want.City {
				t.Errorf("City = %q, want %q", got.City, tt.want.City)
			}
			if got.State != tt.want.State {
				t.Errorf("State = %q, want %q", got.State, tt.want.State)
			}
			if got.Zip != tt.want.Zip {
				t.Errorf("Zip = %q, want %q", got.Zip, tt.want.Zip)
			}
		})
	}
}

func TestCleanLink(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fixes escaped forward slashes",
			input: "https:\\/\\/ipostal1.com\\/secure_checkout?id=123",
			want:  "https://ipostal1.com/secure_checkout?id=123",
		},
		{
			name:  "trims whitespace",
			input: "  https://example.com  ",
			want:  "https://example.com",
		},
		{
			name:  "handles normal URL",
			input: "https://example.com/path",
			want:  "https://example.com/path",
		},
		{
			name:  "handles empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanLink(tt.input)
			if got != tt.want {
				t.Errorf("CleanLink(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNeedsCleanup(t *testing.T) {
	tests := []struct {
		name string
		addr model.AddressRaw
		want bool
	}{
		{
			name: "returns true for HTML tags",
			addr: model.AddressRaw{
				Street: "123 Main St<wbr>",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
			want: true,
		},
		{
			name: "returns true for escaped slashes",
			addr: model.AddressRaw{
				Street: "123 Main St\\/Apt 1",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
			want: true,
		},
		{
			name: "returns true for United States",
			addr: model.AddressRaw{
				Street: "123 Main St",
				City:   "Denver United States",
				State:  "CO",
				Zip:    "80202",
			},
			want: true,
		},
		{
			name: "returns true for redundant city/state/zip",
			addr: model.AddressRaw{
				Street: "123 Main St Denver, CO 80202",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
			want: true,
		},
		{
			name: "returns false for clean address",
			addr: model.AddressRaw{
				Street: "123 Main St",
				City:   "Denver",
				State:  "CO",
				Zip:    "80202",
			},
			want: false,
		},
		{
			name: "returns false for empty address",
			addr: model.AddressRaw{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsCleanup(tt.addr)
			if got != tt.want {
				t.Errorf("NeedsCleanup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveRedundantCityStateZip(t *testing.T) {
	tests := []struct {
		name   string
		street string
		city   string
		state  string
		zip    string
		want   string
	}{
		{
			name:   "removes full city state zip with comma",
			street: "1601 29th St Boulder, CO 80301",
			city:   "Boulder",
			state:  "CO",
			zip:    "80301",
			want:   "1601 29th St",
		},
		{
			name:   "removes full city state zip without comma",
			street: "1601 29th St Boulder CO 80301",
			city:   "Boulder",
			state:  "CO",
			zip:    "80301",
			want:   "1601 29th St",
		},
		{
			name:   "removes state and zip only",
			street: "1601 29th St, CO 80301",
			city:   "Boulder",
			state:  "CO",
			zip:    "80301",
			want:   "1601 29th St",
		},
		{
			name:   "handles case insensitive match",
			street: "1601 29th St BOULDER, CO 80301",
			city:   "Boulder",
			state:  "CO",
			zip:    "80301",
			want:   "1601 29th St",
		},
		{
			name:   "returns unchanged if no match",
			street: "1601 29th St",
			city:   "Boulder",
			state:  "CO",
			zip:    "80301",
			want:   "1601 29th St",
		},
		{
			name:   "handles empty street",
			street: "",
			city:   "Boulder",
			state:  "CO",
			zip:    "80301",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeRedundantCityStateZip(tt.street, tt.city, tt.state, tt.zip)
			if got != tt.want {
				t.Errorf("removeRedundantCityStateZip() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanField(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes HTML tags",
			input: "<span>Hello</span> World",
			want:  "Hello World",
		},
		{
			name:  "fixes escaped closing tags",
			input: "<span>Hello<\\/span>",
			want:  "Hello",
		},
		{
			name:  "decodes amp entity",
			input: "A &amp; B",
			want:  "A & B",
		},
		{
			name:  "decodes lt gt entities",
			input: "&lt;div&gt;",
			want:  "<div>",
		},
		{
			name:  "decodes quote entities",
			input: "&quot;quoted&#39;",
			want:  `"quoted'`,
		},
		{
			name:  "removes nbsp",
			input: "Hello&nbsp;World",
			want:  "Hello World",
		},
		{
			name:  "removes United States",
			input: "Denver United States",
			want:  "Denver",
		},
		{
			name:  "normalizes whitespace",
			input: "Hello   \n   World",
			want:  "Hello World",
		},
		{
			name:  "handles empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanField(tt.input)
			if got != tt.want {
				t.Errorf("cleanField(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
