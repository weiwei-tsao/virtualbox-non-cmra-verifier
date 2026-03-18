# Three-Layer Architecture Rollout Plan

## Overview

This document outlines the incremental deployment strategy for the three-layer architecture refactoring. The rollout is designed to minimize risk by deploying changes incrementally over 8 days with verification at each step.

## Rollout Schedule

### Day 1-2: Database Schema Migration

**Objective**: Add validation lifecycle fields to existing records

**Steps**:
1. Deploy schema changes (already complete - Phase 1)
2. Run migration script to backfill validation fields
3. Verify indexes are created

**Migration Script** (apps/api/cmd/migrate-validation-fields/main.go):
```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
	"google.golang.org/api/iterator"
)

func main() {
	_ = godotenv.Load(".env.local", ".env")

	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	firestoreClient, _, err := firestoreclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("firestore init: %v", err)
	}
	defer firestoreClient.Close()

	log.Println("Starting validation fields migration...")

	// Stream all mailboxes and update validation fields
	iter := firestoreClient.Collection("mailboxes").Documents(ctx)
	updated := 0
	skipped := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("iterate: %v", err)
		}

		data := doc.Data()

		// Check if already migrated
		if _, exists := data["validationStatus"]; exists {
			skipped++
			continue
		}

		// Determine initial status based on existing CMRA/RDI values
		status := "pending"
		priority := "high"
		if cmra, ok := data["cmra"].(string); ok && cmra != "" {
			status = "validated"
			// Determine priority based on lastValidatedAt age
			if lastValidated, ok := data["lastValidatedAt"].(time.Time); ok {
				age := time.Since(lastValidated)
				if age > 90*24*time.Hour {
					status = "needs_revalidation"
					priority = "high"
				} else if age > 70*24*time.Hour {
					priority = "medium"
				} else {
					priority = "low"
				}
			}
		}

		// Update document
		updates := []repository.Update{
			{Path: "validationStatus", Value: status},
			{Path: "validationPriority", Value: priority},
			{Path: "validationAttempts", Value: 0},
			{Path: "lastValidationError", Value: ""},
		}

		if status == "pending" || status == "needs_revalidation" {
			updates = append(updates, repository.Update{
				Path:  "nextRetryAt",
				Value: time.Now(),
			})
		}

		_, err = doc.Ref.Update(ctx, updates)
		if err != nil {
			log.Printf("Failed to update %s: %v", doc.Ref.ID, err)
			continue
		}

		updated++
		if updated%100 == 0 {
			log.Printf("Progress: %d updated, %d skipped", updated, skipped)
		}
	}

	log.Printf("Migration complete: %d updated, %d skipped", updated, skipped)
}
```

**Verification**:
```bash
# Run migration
go run ./cmd/migrate-validation-fields

# Check sample records in Firestore console
# Verify fields: validationStatus, validationPriority, validationAttempts, lastValidationError, nextRetryAt
```

**Required Firestore Indexes**:
Create these composite indexes in Firebase Console:
1. `(validationStatus ASC, validationPriority ASC, nextRetryAt ASC)`
2. `(validationStatus ASC, lastValidatedAt ASC)`
3. `(active ASC, validationStatus ASC)`

---

### Day 3-4: Deploy Validation Service (Inactive)

**Objective**: Deploy validation service code but don't activate background workers yet

**Steps**:
1. Deploy code with validation service
2. Set environment variable: `ENABLE_VALIDATION_WORKERS=false`
3. Test endpoints manually via API

**Environment Variables**:
```bash
# Add to .env or Render environment
ENABLE_VALIDATION_WORKERS=false  # Start with workers disabled
VALIDATION_DAILY_BUDGET=10000
VALIDATION_HIGH_PRIORITY_QUOTA=5000
VALIDATION_MEDIUM_PRIORITY_QUOTA=3000
```

**Manual Testing**:
```bash
# Test validation endpoint manually
curl -X POST http://localhost:8080/api/validation/run

# Check validation stats
curl http://localhost:8080/api/validation/stats

# Expected response:
{
  "pending": 0,
  "validated": 2000,
  "failed": 0,
  "needsRevalidation": 0,
  "retryScheduled": 0,
  "manualReview": 0,
  "total": 2000
}
```

**Code Change** (cmd/server/main.go):
```go
// Only start background workers if enabled
if cfg.EnableValidationWorkers {
    startBackgroundWorkers(ctx, validationSvc, revalidationChk)
    log.Println("Background validation workers enabled")
} else {
    log.Println("Background validation workers disabled (set ENABLE_VALIDATION_WORKERS=true to enable)")
}
```

**Verification**:
- Endpoints respond correctly
- No background workers running
- Logs show "Background validation workers disabled"

---

### Day 5-6: Deploy Aggressive Scraping (Pre-check Enabled)

**Objective**: Enable aggressive scraping with pre-check to reduce network requests

**Steps**:
1. Set environment variable: `USE_AGGRESSIVE_SCRAPING=true`
2. Trigger test crawl
3. Monitor logs for pre-check stats
4. Verify new addresses marked as "pending"

**Expected Log Output**:
```
PreCheck: new=10, existing=1990, deleted=5
Fetching 10 new links (skipping 1990 existing)
Marked 5 records as inactive (source: ATMB)
Crawl completed in 2m15s (was 28m before)
```

**Verification**:
```bash
# Trigger crawl
curl -X POST http://localhost:8080/api/crawl/run

# Check stats after crawl
curl http://localhost:8080/api/stats

# Verify:
# - Crawl time < 5 minutes (was ~30 minutes before)
# - New addresses have validationStatus="pending"
# - Existing addresses unchanged
# - Deleted addresses have active=false
```

**Monitoring**:
- Firestore reads should drop from ~2000 to ~20 per crawl
- Network requests should drop from ~2000 to ~10-50
- Total crawl time should be < 5 minutes

---

### Day 7: Enable Background Validation Workers

**Objective**: Activate automatic validation processing

**Steps**:
1. Set environment variable: `ENABLE_VALIDATION_WORKERS=true`
2. Deploy updated config
3. Monitor validation processing
4. Verify pending queue is processed

**Monitoring**:
```bash
# Watch validation stats every minute
while true; do
  curl -s http://localhost:8080/api/validation/stats | jq
  sleep 60
done

# Expected progression:
# Minute 0:  pending=50, validated=1950, ...
# Minute 5:  pending=40, validated=1960, ...
# Minute 10: pending=30, validated=1970, ...
# ...eventually all pending → validated
```

**Background Worker Logs** (check Render logs):
```
Background validation worker started (interval: 5 minutes)
Validation worker running...
Moved 0 mailboxes from retry_scheduled to pending
Validation worker completed - High: 20, Medium: 15, Low: 15, Succeeded: 50, Failed: 0, Quota remaining: 9950
```

**Verification**:
- Workers run every 5 minutes
- Pending count decreases over time
- Validated count increases
- Quota tracking works correctly
- No errors in logs

---

### Day 8: Enable Revalidation Checker

**Objective**: Activate time-based re-validation for old addresses

**Steps**:
1. Set environment variable: `ENABLE_REVALIDATION_CHECKER=true`
2. Deploy updated config
3. Manually trigger revalidation check
4. Verify old addresses marked for revalidation

**Manual Trigger**:
```bash
# Trigger revalidation check manually
curl -X POST http://localhost:8080/api/validation/revalidation/check

# Expected response:
{
  "message": "Revalidation check completed",
  "stats": {
    "checked": 2000,
    "needsRevalidation": 50,
    "approachingThreshold": 120,
    "upToDate": 1830
  }
}
```

**Revalidation Checker Logs**:
```
Background revalidation checker started (daily at 2 AM UTC)
Next revalidation check scheduled for 2026-03-17 02:00:00 UTC (in 9h 23m)
```

**Verification**:
- Addresses >90 days old have status="needs_revalidation"
- Addresses 70-89 days old have priority="medium"
- Revalidation checker scheduled for next 2 AM UTC

---

## Feature Flags

Add these boolean environment variables for gradual rollout:

```bash
# In .env or Render environment variables
USE_AGGRESSIVE_SCRAPING=true      # Enable pre-check strategy
ENABLE_VALIDATION_WORKERS=true    # Enable background validation processing
ENABLE_REVALIDATION_CHECKER=true  # Enable time-based re-validation
```

**Implementation** (internal/platform/config/config.go):
```go
type Config struct {
    // ... existing fields ...

    // Feature flags
    UseAggressiveScraping    bool `env:"USE_AGGRESSIVE_SCRAPING" envDefault:"true"`
    EnableValidationWorkers  bool `env:"ENABLE_VALIDATION_WORKERS" envDefault:"false"`
    EnableRevalidationChecker bool `env:"ENABLE_REVALIDATION_CHECKER" envDefault:"false"`
}
```

**Usage in Code**:
```go
// In crawler/service.go
func (s *Service) Start(ctx context.Context, links []string) (string, error) {
    if s.config.UseAggressiveScraping {
        precheck := PreCheckLinks(ctx, s.mailboxes, links, "ATMB")
        links = precheck.NewLinks  // Only fetch new links
    }
    // ... rest of crawl logic
}

// In cmd/server/main.go
if cfg.EnableValidationWorkers {
    startBackgroundWorkers(ctx, validationSvc, revalidationChk)
}
```

---

## Rollback Plan

If issues occur at any stage:

### Stage 1 (Schema): Rollback is NOT recommended
- Fields are backward compatible (old code ignores new fields)
- No data loss risk

### Stage 2-4 (Validation Service): Safe to rollback
```bash
# Disable validation workers
ENABLE_VALIDATION_WORKERS=false

# Redeploy previous version
git revert <commit-hash>
```

### Stage 5 (Aggressive Scraping): Safe to rollback
```bash
# Disable aggressive scraping
USE_AGGRESSIVE_SCRAPING=false

# Falls back to old scraping behavior
```

### Complete Rollback
```bash
# Set all feature flags to false
USE_AGGRESSIVE_SCRAPING=false
ENABLE_VALIDATION_WORKERS=false
ENABLE_REVALIDATION_CHECKER=false

# Redeploy previous branch
git checkout main
```

---

## Monitoring Checklist

After each deployment day, verify:

- [ ] **Application Health**
  - No error spikes in Render logs
  - Memory usage stable (<500MB)
  - CPU usage stable (<50%)
  - Response times < 500ms

- [ ] **Firestore Usage**
  - Daily reads within free tier (< 50K/day)
  - Daily writes within free tier (< 20K/day)
  - No index errors in logs

- [ ] **API Endpoints**
  - All endpoints respond with 200
  - No 500 errors in logs
  - CORS headers present

- [ ] **Background Workers**
  - Workers run on schedule (check logs)
  - No worker crashes or panics
  - Validation queue decreasing over time

- [ ] **Data Quality**
  - Validation statuses updating correctly
  - Quota tracking accurate
  - No stuck records in pending state

---

## Success Metrics

After full rollout (Day 8), expect:

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Crawl time | 30 min | 2 min | 93% reduction |
| Network requests/crawl | 2000 | 10-50 | 99.5% reduction |
| Firestore reads/crawl | 200MB | 2MB | 99% reduction |
| Crawls per day (free tier) | 3 | 30+ | 10x increase |
| Validation response time | Blocks crawl | Independent | ∞ improvement |
| Re-validation capability | None | 90-day cycles | New feature |
| Error resilience | Fail entire batch | Individual retry | Improved |

---

## Post-Rollout Tasks

After Day 8:

1. **Remove feature flags** - Once stable, remove flags and make behavior default
2. **Monitor for 1 week** - Watch for any edge cases or issues
3. **Update documentation** - Mark refactoring as complete in CLAUDE.md
4. **Clean up old code** - Remove unused validation code from scraper (if any remains)
5. **Performance tuning** - Adjust worker intervals and quota limits based on usage patterns

---

## Emergency Contacts

If issues arise during rollout:

- Check Render logs: https://dashboard.render.com
- Check Firestore console: https://console.firebase.google.com
- Check API health: `curl http://localhost:8080/healthz`
- Review this plan: `/docs/ROLLOUT_PLAN.md`
