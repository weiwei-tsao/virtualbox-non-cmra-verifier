package http

import (
	"regexp"
	"strings"
	"testing"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
)

func ptr(b bool) *bool {
	return &b
}

func TestGenerateExportFilename(t *testing.T) {
	tests := []struct {
		name      string
		query     repository.MailboxQuery
		wantParts []string
		wantRe    string
	}{
		{
			name:      "no filters - default",
			query:     repository.MailboxQuery{Active: ptr(true)},
			wantParts: []string{"mailbox-", ".csv"},
			wantRe:    `^mailbox-\d{8}T\d{6}Z\.csv$`,
		},
		{
			name:      "state only",
			query:     repository.MailboxQuery{State: "CA", Active: ptr(true)},
			wantParts: []string{"mailbox-CA-"},
			wantRe:    `^mailbox-CA-\d{8}T\d{6}Z\.csv$`,
		},
		{
			name:      "source only",
			query:     repository.MailboxQuery{Source: "ATMB", Active: ptr(true)},
			wantParts: []string{"mailbox-ATMB-"},
			wantRe:    `^mailbox-ATMB-\d{8}T\d{6}Z\.csv$`,
		},
		{
			name:      "cmra only",
			query:     repository.MailboxQuery{CMRA: "Y", Active: ptr(true)},
			wantParts: []string{"mailbox-Y-"},
			wantRe:    `^mailbox-Y-\d{8}T\d{6}Z\.csv$`,
		},
		{
			name:      "rdi only",
			query:     repository.MailboxQuery{RDI: "Commercial", Active: ptr(true)},
			wantParts: []string{"mailbox-Commercial-"},
			wantRe:    `^mailbox-Commercial-\d{8}T\d{6}Z\.csv$`,
		},
		{
			name: "multiple filters",
			query: repository.MailboxQuery{
				State:  "TX",
				Source: "iPost1",
				Active: ptr(true),
			},
			wantParts: []string{"mailbox-TX-iPost1-"},
			wantRe:    `^mailbox-TX-iPost1-\d{8}T\d{6}Z\.csv$`,
		},
		{
			name: "all filters",
			query: repository.MailboxQuery{
				State:  "NY",
				Source: "ATMB",
				CMRA:   "Y",
				RDI:    "Residential",
				Active: ptr(true),
			},
			wantParts: []string{"mailbox-NY-ATMB-Y-Residential-"},
			wantRe:    `^mailbox-NY-ATMB-Y-Residential-\d{8}T\d{6}Z\.csv$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateExportFilename(tt.query)

			// Check regex match
			matched, _ := regexp.MatchString(tt.wantRe, got)
			if !matched {
				t.Errorf("filename %q does not match pattern %q", got, tt.wantRe)
			}

			// Check required parts
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("filename %q missing required part %q", got, part)
				}
			}

			// Verify ends with .csv
			if !strings.HasSuffix(got, ".csv") {
				t.Errorf("filename %q does not end with .csv", got)
			}
		})
	}
}
