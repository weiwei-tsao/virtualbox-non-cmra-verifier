package validation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
)

// QuotaConfig defines quota limits and allocations by priority.
type QuotaConfig struct {
	DailyBudget        int // Total validations allowed per day (e.g., 10,000)
	HighPriorityQuota  int // Quota reserved for high priority (e.g., 5,000 = 50%)
	MediumPriorityQuota int // Quota reserved for medium priority (e.g., 3,000 = 30%)
	// Remaining quota goes to low priority (e.g., 2,000 = 20%)
}

// QuotaUsage tracks daily quota consumption.
type QuotaUsage struct {
	Date         string `firestore:"date"`         // Date key (YYYY-MM-DD)
	TotalUsed    int    `firestore:"totalUsed"`    // Total validations used today
	HighUsed     int    `firestore:"highUsed"`     // High priority validations used
	MediumUsed   int    `firestore:"mediumUsed"`   // Medium priority validations used
	LowUsed      int    `firestore:"lowUsed"`      // Low priority validations used
	LastUpdated  time.Time `firestore:"lastUpdated"` // Last update timestamp
}

// QuotaManager tracks and enforces daily validation quota limits.
type QuotaManager struct {
	config          QuotaConfig
	firestoreClient *firestore.Client
	mu              sync.Mutex
	cache           *QuotaUsage // Cached quota for current day
	cacheDate       string      // Date of cached quota
}

// NewQuotaManager creates a new quota manager.
func NewQuotaManager(config QuotaConfig, client *firestore.Client) *QuotaManager {
	return &QuotaManager{
		config:          config,
		firestoreClient: client,
	}
}

// GetAvailableQuota returns how many validations can be done for each priority.
func (qm *QuotaManager) GetAvailableQuota(ctx context.Context) (high, medium, low int, err error) {
	usage, err := qm.getTodayUsage(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("get usage: %w", err)
	}

	// Calculate remaining quota for each priority
	highRemaining := qm.config.HighPriorityQuota - usage.HighUsed
	if highRemaining < 0 {
		highRemaining = 0
	}

	mediumRemaining := qm.config.MediumPriorityQuota - usage.MediumUsed
	if mediumRemaining < 0 {
		mediumRemaining = 0
	}

	// Low priority gets whatever is left after high and medium
	totalRemaining := qm.config.DailyBudget - usage.TotalUsed
	if totalRemaining < 0 {
		totalRemaining = 0
	}
	lowRemaining := totalRemaining - highRemaining - mediumRemaining
	if lowRemaining < 0 {
		lowRemaining = 0
	}

	return highRemaining, mediumRemaining, lowRemaining, nil
}

// CanValidate checks if there's quota available for a given priority and count.
func (qm *QuotaManager) CanValidate(ctx context.Context, priority string, count int) (bool, error) {
	high, medium, low, err := qm.GetAvailableQuota(ctx)
	if err != nil {
		return false, err
	}

	switch priority {
	case "high":
		return high >= count, nil
	case "medium":
		return medium >= count, nil
	case "low":
		return low >= count, nil
	default:
		return false, fmt.Errorf("unknown priority: %s", priority)
	}
}

// RecordUsage increments quota usage for validations performed.
func (qm *QuotaManager) RecordUsage(ctx context.Context, priority string, count int) error {
	if count <= 0 {
		return nil
	}

	qm.mu.Lock()
	defer qm.mu.Unlock()

	today := getTodayKey()
	docRef := qm.firestoreClient.Collection("system_config").Doc("quota_usage").Collection("daily").Doc(today)

	// Use transaction to ensure atomic increment
	err := qm.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		var usage QuotaUsage
		if err != nil {
			// Document doesn't exist - initialize it
			usage = QuotaUsage{
				Date:        today,
				TotalUsed:   0,
				HighUsed:    0,
				MediumUsed:  0,
				LowUsed:     0,
				LastUpdated: time.Now(),
			}
		} else {
			if err := doc.DataTo(&usage); err != nil {
				return fmt.Errorf("decode usage: %w", err)
			}
		}

		// Increment usage
		usage.TotalUsed += count
		switch priority {
		case "high":
			usage.HighUsed += count
		case "medium":
			usage.MediumUsed += count
		case "low":
			usage.LowUsed += count
		}
		usage.LastUpdated = time.Now()

		return tx.Set(docRef, usage)
	})

	if err != nil {
		return fmt.Errorf("record usage: %w", err)
	}

	// Invalidate cache
	qm.cache = nil
	qm.cacheDate = ""

	return nil
}

// getTodayUsage fetches or initializes today's quota usage.
func (qm *QuotaManager) getTodayUsage(ctx context.Context) (QuotaUsage, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	today := getTodayKey()

	// Return cached usage if available for today
	if qm.cache != nil && qm.cacheDate == today {
		return *qm.cache, nil
	}

	// Fetch from Firestore
	docRef := qm.firestoreClient.Collection("system_config").Doc("quota_usage").Collection("daily").Doc(today)
	doc, err := docRef.Get(ctx)

	var usage QuotaUsage
	if err != nil {
		// Document doesn't exist - initialize with zeros
		usage = QuotaUsage{
			Date:        today,
			TotalUsed:   0,
			HighUsed:    0,
			MediumUsed:  0,
			LowUsed:     0,
			LastUpdated: time.Now(),
		}
	} else {
		if err := doc.DataTo(&usage); err != nil {
			return usage, fmt.Errorf("decode usage: %w", err)
		}
	}

	// Cache for future calls
	qm.cache = &usage
	qm.cacheDate = today

	return usage, nil
}

// GetUsageSummary returns a human-readable summary of quota usage.
func (qm *QuotaManager) GetUsageSummary(ctx context.Context) (string, error) {
	usage, err := qm.getTodayUsage(ctx)
	if err != nil {
		return "", err
	}

	high, medium, low, _ := qm.GetAvailableQuota(ctx)

	return fmt.Sprintf(
		"Quota Usage (%s):\n"+
			"  Total: %d/%d (%.1f%%)\n"+
			"  High: %d/%d remaining\n"+
			"  Medium: %d/%d remaining\n"+
			"  Low: %d remaining",
		usage.Date,
		usage.TotalUsed, qm.config.DailyBudget,
		float64(usage.TotalUsed)/float64(qm.config.DailyBudget)*100,
		high, qm.config.HighPriorityQuota,
		medium, qm.config.MediumPriorityQuota,
		low,
	), nil
}

// ResetDailyQuota clears quota usage (typically called at midnight UTC).
// This is handled automatically by using date-based document keys.
func (qm *QuotaManager) ResetDailyQuota() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// Just clear the cache - new day will create new document
	qm.cache = nil
	qm.cacheDate = ""
}

// getTodayKey returns the date key for today (YYYY-MM-DD in UTC).
func getTodayKey() string {
	return time.Now().UTC().Format("2006-01-02")
}
