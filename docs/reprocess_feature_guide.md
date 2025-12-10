# Reprocess Feature - Usage Guide

## 🎯 Overview

The reprocess feature allows you to **re-parse all mailboxes from stored HTML without re-fetching** from AnytimeMailbox. This is essential for:

- 🐛 **Parser bug fixes** - Update all records after fixing parsing logic
- 🔄 **Parser improvements** - Apply new parsing strategies to existing data
- ⚡ **Fast iteration** - Test parser changes in ~2 minutes instead of 30+ minutes
- 🚫 **Avoid rate limits** - No need to re-fetch thousands of pages

## 📋 What Changed

### New Fields in Mailbox Model

```go
type Mailbox struct {
    // ... existing fields ...

    // New fields for reprocessing support
    RawHTML       string    `json:"-" firestore:"rawHTML,omitempty"`
    ParserVersion string    `json:"parserVersion,omitempty" firestore:"parserVersion,omitempty"`
    LastParsedAt  time.Time `json:"lastParsedAt,omitempty" firestore:"lastParsedAt,omitempty"`
}
```

- **RawHTML**: Original HTML from AnytimeMailbox (not exposed to API to save bandwidth)
- **ParserVersion**: Tracks which parser version processed this record (e.g., "v1.0")
- **LastParsedAt**: Timestamp of last parsing

### New API Endpoint

```
POST /api/crawl/reprocess
```

**Request body:**
```json
{
  "targetVersion": "v1.1",   // Optional: defaults to current version
  "onlyOutdated": true       // Optional: only reprocess old versions
}
```

**Response:**
```json
{
  "runId": "RUN_1733598765",
  "message": "Reprocessing started. Check status with GET /api/crawl/status?runId=RUN_1733598765"
}
```

## 🚀 Usage Scenarios

### Scenario 1: First Time Setup (Save HTML for Future)

**Important**: The first crawl after this update will save HTML. Existing records don't have HTML yet.

```bash
# 1. Deploy updated backend with reprocess feature
cd apps/api
go build -o server cmd/server/main.go
./server

# 2. Run ONE full crawl to save HTML
curl -X POST http://localhost:8080/api/crawl/run \
  -H "Content-Type: application/json" \
  -d '{"links": ["https://www.anytimemailbox.com/l/usa"]}'

# 3. Wait for completion (check status)
curl http://localhost:8080/api/crawl/status?runId=RUN_XXXXXXXXXX
```

**After this crawl completes**, all 2073 records will have `rawHTML` saved. Now you can reprocess anytime!

### Scenario 2: Fix Parser Bug and Update All Records

You fixed a bug in [parser.go](apps/api/internal/business/crawler/parser.go). Now update all records:

```bash
# 1. Update parser version in scraper.go
# Change: const CurrentParserVersion = "v1.0"
# To:     const CurrentParserVersion = "v1.1"

# 2. Deploy updated code
go build -o server cmd/server/main.go
./server

# 3. Reprocess all records with old version
curl -X POST http://localhost:8080/api/crawl/reprocess \
  -H "Content-Type: application/json" \
  -d '{"targetVersion": "v1.1", "onlyOutdated": true}'

# 4. Monitor progress
curl http://localhost:8080/api/crawl/status?runId=RUN_XXXXXXXXXX
```

**Result**: Only records with `parserVersion != "v1.1"` are reprocessed (no re-fetching).

### Scenario 3: Force Reprocess Everything

Reprocess ALL records regardless of version:

```bash
curl -X POST http://localhost:8080/api/crawl/reprocess \
  -H "Content-Type: application/json" \
  -d '{"onlyOutdated": false}'
```

### Scenario 4: Test Parser Changes Locally

**Before reprocess feature** (30+ minutes):
```bash
# Modify parser.go
# Run full crawl (2073 URLs × ~1s = 30+ minutes)
curl -X POST http://localhost:8080/api/crawl/run -d '{"links": [...]}'
# Wait 30 minutes...
```

**With reprocess feature** (~2 minutes):
```bash
# Modify parser.go
# Change version in scraper.go
const CurrentParserVersion = "v1.1-test"

# Rebuild and run
go run ./cmd/server &

# Reprocess from DB (2073 records × ~0.05s = ~2 minutes)
curl -X POST http://localhost:8080/api/crawl/reprocess \
  -H "Content-Type: application/json" \
  -d '{"targetVersion": "v1.1-test"}'
```

## 📊 Monitoring Reprocess Jobs

Check status using the same endpoint as regular crawls:

```bash
curl http://localhost:8080/api/crawl/status?runId=RUN_1733598765
```

**Response:**
```json
{
  "runId": "RUN_1733598765",
  "status": "running",
  "stats": {
    "found": 2073,        // Total records
    "validated": 1850,    // Processed so far
    "skipped": 50,        // No HTML or already up-to-date
    "failed": 5           // Parse errors
  },
  "startedAt": "2024-12-07T10:00:00Z",
  "finishedAt": "0001-01-01T00:00:00Z"
}
```

## 🔍 How It Works

### 1. Normal Crawl (Saves HTML)

```
Fetch URL → Read HTML → Parse HTML → Validate → Save to DB
                 ↓
          Also save to mailbox.RawHTML
          Set mailbox.ParserVersion = "v1.0"
```

### 2. Reprocess (From DB)

```
Load from DB → Read mailbox.RawHTML → Re-parse → (Optional) Re-validate → Update DB
                                            ↓
                                 Update mailbox.ParserVersion = "v1.1"
```

**No network requests to AnytimeMailbox!** 🎉

## 🎛️ Configuration

### Parser Version Management

Edit [scraper.go:15](apps/api/internal/business/crawler/scraper.go#L15):

```go
const CurrentParserVersion = "v1.0"  // Change this when parser logic changes
```

**Best practices**:
- Increment version when fixing parser bugs: `v1.0` → `v1.1`
- Use semantic versioning for major changes: `v1.x` → `v2.0`
- Use suffixes for testing: `v1.1-test`, `v1.1-fix-price`

### Batch Size

Default: 100 records per batch write.

To change, modify [reprocess.go:47](apps/api/internal/business/crawler/reprocess.go#L47):

```go
if opts.BatchSize <= 0 {
    opts.BatchSize = 100  // Change this value
}
```

## 📈 Performance Comparison

| Operation | Before | After | Speedup |
|-----------|--------|-------|---------|
| Fix parser bug + update all | 30+ min (re-crawl 2073 URLs) | ~2 min (reprocess from DB) | **15x faster** |
| Test parser change locally | 30+ min | ~2 min | **15x faster** |
| Risk of IP ban | High (2073 requests) | Zero (no requests) | ∞ |

## ⚠️ Important Notes

### First-Time Migration

**Existing records don't have HTML saved**. You must:

1. Deploy this update
2. Run ONE full crawl to populate `rawHTML` fields
3. After that, you can reprocess anytime

### Storage Implications

- Each HTML is ~50-100 KB
- 2073 records × 75 KB avg = ~155 MB total
- Firestore pricing: $0.18/GB/month = ~$0.03/month for HTML storage
- **Totally worth it** for 15x speedup! 💰

### Version Control

- Always increment `CurrentParserVersion` when changing parser logic
- Use `onlyOutdated: true` to avoid reprocessing unchanged records
- The system tracks versions automatically

## 🧪 Testing

Run tests to verify everything works:

```bash
cd apps/api
go test ./internal/business/crawler/... -v
```

**Test coverage**:
- ✅ Parser correctly saves RawHTML
- ✅ Reprocess updates only outdated records
- ✅ Reprocess preserves metadata (ID, CrawlRunID, etc.)
- ✅ Version filtering works correctly
- ✅ Batch writing works

## 🔗 Related Files

- [model.go](apps/api/pkg/model/model.go#L34-L36) - New Mailbox fields
- [scraper.go](apps/api/internal/business/crawler/scraper.go#L15) - CurrentParserVersion constant
- [scraper.go](apps/api/internal/business/crawler/scraper.go#L76-L114) - Save HTML logic
- [reprocess.go](apps/api/internal/business/crawler/reprocess.go) - Reprocess implementation
- [service.go](apps/api/internal/business/crawler/service.go#L154-L234) - Reprocess service method
- [router.go](apps/api/internal/platform/http/router.go#L47) - API endpoint
- [firestore.indexes.json](apps/api/firestore.indexes.json#L26-L33) - New index

## 📝 Example Workflow

### Day 1: Initial Setup
```bash
# Deploy with reprocess feature
# Run full crawl (saves HTML)
# Result: 2073 records with HTML
```

### Day 2: Fix Parser Bug
```bash
# Find bug in parser.go (e.g., price extraction wrong)
# Fix parser.go
# Update CurrentParserVersion = "v1.1"
# Deploy
# POST /api/crawl/reprocess with onlyOutdated=true
# Wait 2 minutes
# Result: All prices corrected, no re-crawling!
```

### Day 3: Another Fix
```bash
# Fix address parsing
# Update CurrentParserVersion = "v1.2"
# Deploy
# POST /api/crawl/reprocess with onlyOutdated=true
# Wait 2 minutes
# Result: All addresses corrected!
```

## 🎉 Benefits Summary

✅ **15x faster** parser iteration (2 min vs 30 min)
✅ **Zero risk** of IP bans (no external requests)
✅ **Version tracking** - know exactly what was processed when
✅ **Incremental updates** - only process what changed
✅ **Automatic** - just call one API endpoint
✅ **Tested** - full test coverage
✅ **Cost effective** - ~$0.03/month storage vs hours of dev time

---

**Ready to use!** Start by running one full crawl to save HTML, then enjoy instant reprocessing! 🚀
