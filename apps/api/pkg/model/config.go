package model

import "time"

// CrawlerConfig holds configurable settings for scraping and validation.
type CrawlerConfig struct {
	// Scraping settings
	WorkerCount     int           `json:"workerCount" firestore:"workerCount" yaml:"workerCount"`
	ScrapeBatchSize int           `json:"scrapeBatchSize" firestore:"scrapeBatchSize" yaml:"scrapeBatchSize"`
	ScrapeTimeout   time.Duration `json:"scrapeTimeout" firestore:"scrapeTimeout" yaml:"scrapeTimeout"`

	// Validation settings
	ValidationBatchSize    int           `json:"validationBatchSize" firestore:"validationBatchSize" yaml:"validationBatchSize"`
	ValidationWorkerCount  int           `json:"validationWorkerCount" firestore:"validationWorkerCount" yaml:"validationWorkerCount"`
	MaxRetryAttempts       int           `json:"maxRetryAttempts" firestore:"maxRetryAttempts" yaml:"maxRetryAttempts"`
	RetryBackoffBase       time.Duration `json:"retryBackoffBase" firestore:"retryBackoffBase" yaml:"retryBackoffBase"`
	RetryBackoffMultiplier float64       `json:"retryBackoffMultiplier" firestore:"retryBackoffMultiplier" yaml:"retryBackoffMultiplier"`
	RetryBackoffJitter     float64       `json:"retryBackoffJitter" firestore:"retryBackoffJitter" yaml:"retryBackoffJitter"`

	// Re-validation settings
	RevalidationInterval  time.Duration `json:"revalidationInterval" firestore:"revalidationInterval" yaml:"revalidationInterval"`
	RevalidationThreshold time.Duration `json:"revalidationThreshold" firestore:"revalidationThreshold" yaml:"revalidationThreshold"`

	// Quota management
	DailyValidationBudget int `json:"dailyValidationBudget" firestore:"dailyValidationBudget" yaml:"dailyValidationBudget"`
	HighPriorityQuota     int `json:"highPriorityQuota" firestore:"highPriorityQuota" yaml:"highPriorityQuota"`
	MediumPriorityQuota   int `json:"mediumPriorityQuota" firestore:"mediumPriorityQuota" yaml:"mediumPriorityQuota"`
}

// DefaultCrawlerConfig returns sensible defaults for crawler configuration.
func DefaultCrawlerConfig() CrawlerConfig {
	return CrawlerConfig{
		// Scraping defaults
		WorkerCount:     5,
		ScrapeBatchSize: 20,
		ScrapeTimeout:   30 * time.Minute,

		// Validation defaults
		ValidationBatchSize:    100, // Smarty API limit
		ValidationWorkerCount:  3,
		MaxRetryAttempts:       5,
		RetryBackoffBase:       1 * time.Second,
		RetryBackoffMultiplier: 2.0,
		RetryBackoffJitter:     0.1,

		// Re-validation defaults (90 days)
		RevalidationInterval:  90 * 24 * time.Hour,
		RevalidationThreshold: 70 * 24 * time.Hour, // Start increasing priority at 70 days

		// Quota defaults
		DailyValidationBudget: 10000,
		HighPriorityQuota:     5000, // 50% for new addresses
		MediumPriorityQuota:   3000, // 30% for approaching revalidation
		// Remaining 20% (2000) for low priority
	}
}
