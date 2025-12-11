package model

import "time"

// AddressRaw mirrors the unstandardized address scraped from the source site.
type AddressRaw struct {
	Street string `json:"street,omitempty" firestore:"street,omitempty"`
	City   string `json:"city,omitempty" firestore:"city,omitempty"`
	State  string `json:"state,omitempty" firestore:"state,omitempty"`
	Zip    string `json:"zip,omitempty" firestore:"zip,omitempty"`
}

// StandardizedAddress represents the normalized address returned by Smarty.
type StandardizedAddress struct {
	DeliveryLine1 string `json:"deliveryLine1,omitempty" firestore:"deliveryLine1,omitempty"`
	LastLine      string `json:"lastLine,omitempty" firestore:"lastLine,omitempty"`
}

// Mailbox is the core document stored in the `mailboxes` collection.
type Mailbox struct {
	ID                  string              `json:"id,omitempty" firestore:"id,omitempty"`
	Source              string              `json:"source,omitempty" firestore:"source,omitempty"` // Data source: "ATMB" or "iPost1"
	Name                string              `json:"name,omitempty" firestore:"name,omitempty"`
	AddressRaw          AddressRaw          `json:"addressRaw,omitempty" firestore:"addressRaw,omitempty"`
	Price               float64             `json:"price,omitempty" firestore:"price,omitempty"`
	Link                string              `json:"link,omitempty" firestore:"link,omitempty"`
	CMRA                string              `json:"cmra,omitempty" firestore:"cmra,omitempty"`
	RDI                 string              `json:"rdi,omitempty" firestore:"rdi,omitempty"`
	StandardizedAddress StandardizedAddress `json:"standardizedAddress,omitempty" firestore:"standardizedAddress,omitempty"`
	DataHash            string              `json:"dataHash,omitempty" firestore:"dataHash,omitempty"`
	LastValidatedAt     time.Time           `json:"lastValidatedAt,omitempty" firestore:"lastValidatedAt,omitempty"`
	CrawlRunID          string              `json:"crawlRunId,omitempty" firestore:"crawlRunId,omitempty"`
	Active              bool                `json:"active,omitempty" firestore:"active,omitempty"`
	// Fields for reprocessing support
	RawHTML       string    `json:"-" firestore:"rawHTML,omitempty"`             // Original HTML (not exposed to API)
	ParserVersion string    `json:"parserVersion,omitempty" firestore:"parserVersion,omitempty"` // Parser version (e.g., "v1.0")
	LastParsedAt  time.Time `json:"lastParsedAt,omitempty" firestore:"lastParsedAt,omitempty"`   // Last parsing timestamp
}

// CrawlRunStats stores aggregated counters for a crawl job.
type CrawlRunStats struct {
	Found     int `json:"found,omitempty" firestore:"found,omitempty"`
	Validated int `json:"validated,omitempty" firestore:"validated,omitempty"`
	Skipped   int `json:"skipped,omitempty" firestore:"skipped,omitempty"`
	Failed    int `json:"failed,omitempty" firestore:"failed,omitempty"`
}

// CrawlRun tracks the lifecycle of a crawler execution.
type CrawlRun struct {
	RunID       string        `json:"runId,omitempty" firestore:"runId,omitempty"`
	Source      string        `json:"source,omitempty" firestore:"source,omitempty"` // Data source: "ATMB" or "iPost1"
	Status      string        `json:"status,omitempty" firestore:"status,omitempty"`
	Stats       CrawlRunStats `json:"stats,omitempty" firestore:"stats,omitempty"`
	StartedAt   time.Time     `json:"startedAt,omitempty" firestore:"startedAt,omitempty"`
	FinishedAt  time.Time     `json:"finishedAt,omitempty" firestore:"finishedAt,omitempty"`
	ErrorSample []ErrorSample `json:"errorsSample,omitempty" firestore:"errorsSample,omitempty"`
}

// ErrorSample captures a subset of errors for observability without heavy logging.
type ErrorSample struct {
	Link   string `json:"link,omitempty" firestore:"link,omitempty"`
	Reason string `json:"reason,omitempty" firestore:"reason,omitempty"`
}

// SystemStats is a singleton document that pre-aggregates dashboard metrics.
type SystemStats struct {
	LastUpdated      time.Time      `json:"lastUpdated,omitempty" firestore:"lastUpdated,omitempty"`
	TotalMailboxes   int            `json:"totalMailboxes,omitempty" firestore:"totalMailboxes,omitempty"`
	TotalCommercial  int            `json:"totalCommercial,omitempty" firestore:"totalCommercial,omitempty"`
	TotalResidential int            `json:"totalResidential,omitempty" firestore:"totalResidential,omitempty"`
	AvgPrice         float64        `json:"avgPrice,omitempty" firestore:"avgPrice,omitempty"`
	ByState          map[string]int `json:"byState,omitempty" firestore:"byState,omitempty"`
}
