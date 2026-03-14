# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

US Virtual Mailbox Address Aggregator & Validator - scrapes addresses from ATMB (~2,000) and iPost1 (~4,000), validates them via Smarty API for CMRA/RDI classification, and provides a dashboard for filtering and export.

**Tech Stack**: Go 1.25 (Gin) + React 19 (TypeScript/Vite) + Firebase Firestore + Smarty API + chromedp

## Commands

### Development

```bash
# Backend (from apps/api/)
go run ./cmd/server                    # Start server (uses .env.local)
env $(cat .env.local | xargs) go run ./cmd/server  # Explicit env loading

# Frontend (from apps/web/)
npm run dev                            # Start dev server (port 5173)

# Testing
go test ./...                          # Run all tests
go test ./internal/business/crawler -v # Run specific package tests

# Build verification (run before commits)
go build ./...                         # Build all packages
go vet ./...                           # Static analysis
go test ./...                          # Run tests

# Utility scripts (from apps/api/)
go run ./scripts/batch_validate        # Batch validate addresses
go run ./scripts/check_cmra_rdi        # Check CMRA/RDI status
go run ./scripts/find_missing_html     # Find records without HTML
go run ./scripts/test_smarty_api       # Test Smarty API connection
```

### Deployment

```bash
# Backend build (Render)
go build -o server cmd/server/main.go

# Frontend (Vercel)
# Auto-deploys on push, set VITE_API_URL env var
```

## Frontend Component Patterns

### Toast Notification System

**Location**: `apps/web/contexts/ToastContext.tsx`, `apps/web/components/Toast.tsx`, `apps/web/components/ToastContainer.tsx`

**Usage**:
```typescript
import { useToast } from '../contexts/ToastContext';

const { showToast } = useToast();
showToast('Operation successful!', 'success');
showToast('Error occurred', 'error', 5000); // Custom duration
```

**Types**: `success`, `error`, `info`, `warning`

**Features**:
- Auto-dismiss (default 5s)
- Bottom-right positioning
- Max 3 visible toasts
- No external dependencies

### Reusable Badge Components

**Location**: `apps/web/components/badges/`

```typescript
import { RDIBadge, CMRABadge, SourceBadge } from '../components/badges';

<RDIBadge rdi="Commercial" />
<CMRABadge cmra="Y" />
<SourceBadge source="ATMB" />
```

### StatCard Component

**Location**: `apps/web/components/ui/StatCard.tsx`

```typescript
import { StatCard } from '../components/ui/StatCard';

<StatCard
  title="Total Mailboxes"
  value="2,045"
  icon={<TrendingUp />}
  color="bg-primary text-primary"
  onClick={() => setFilter('all')}
  isActive={filter === 'all'}
/>
```

### CSV Export Hook

**Location**: `apps/web/hooks/useCSVExport.ts`

```typescript
import { useCSVExport } from '../hooks/useCSVExport';

const { exportCSV } = useCSVExport();
exportCSV(filter); // Automatically shows toast notifications
```

**Features**:
- Fetch-based download (better error handling than window.open)
- Extracts dynamic filename from Content-Disposition header
- Shows toast notifications for lifecycle events
- Proper blob cleanup

### Export Filename Format

Exported CSV files use dynamic filenames based on active filters:

- **No filters**: `mailbox-20260313T142530Z.csv`
- **With filters**: `mailbox-CA-ATMB-Y-Commercial-20260313T142530Z.csv`
- **Format**: `mailbox-{state}-{source}-{cmra}-{rdi}-{timestamp}.csv`
- Empty filter segments are skipped
- Timestamp in UTC (RFC3339 basic format)

## Architecture Patterns

### Batch Processing Workflow

All three crawlers (ATMB, iPost1, Reprocess) use the same pattern:

```
1. Collect items in buffer (max 20)
2. When buffer full:
   a. Batch validate with Smarty API (up to 100 addresses/request)
   b. BatchUpsert to Firestore (all 20 records in one write)
3. Continue until complete
4. Mark-and-sweep: Set active=false for missing records
```

**Critical**: Never skip the batch validation step. Individual API calls would exhaust Smarty quota (2000 addresses = 20 batch calls vs 2000 individual calls).

### Parser Versioning System

Records store `parserVersion` and `rawHTML` to enable reprocessing without re-fetching:

1. Parser bug found → increment `CurrentParserVersion` in `scraper.go`
2. Deploy code
3. Call `POST /api/crawl/reprocess` with `outdatedOnly: true`
4. System re-parses all outdated records from stored HTML (~2 min vs ~30 min re-crawl)

**When to use reprocessing**:
- Parser bug fixes
- Parser improvements
- Testing parser changes

**Do not use for**: New data sources, HTML structure changes (requires re-fetch)

### Metadata-Only Fetching

`FetchAllMetadata()` loads only `{id, link, dataHash, cmra, rdi}` instead of full documents:

- Full fetch: ~200MB for 2000 docs
- Metadata only: ~2MB for 2000 docs
- **Impact**: 90% reduction in Firestore reads, enables 30+ crawls/day within free tier

**Usage**: Called at crawl start for deduplication checks. Full documents only loaded during reprocessing.

### Data Deduplication

Uses `dataHash` (MD5 of name + address) to skip unchanged records:

```go
if existingHash == newHash && existingCMRA != "" {
    skip() // Already validated, no changes
}
```

**Important**: Cannot rely on `parsed.CMRA` from HTML parsing - it's always empty after parsing. Must check stored values from DB.

### Worker Pool Pattern

ATMB crawler uses concurrent worker pool:

```
orchestrator.go:
- 5 concurrent workers (configurable via CRAWLER_CONCURRENCY)
- Processes location URLs in parallel
- Each worker: fetch → parse → collect in buffer
- Shared channel for results aggregation
```

**Render free tier**: Keep at 5 workers (CPU/memory limited). Local dev can use 10+.

### Multi-Credential Load Balancing

Smarty client supports multiple credentials for load distribution:

```bash
SMARTY_AUTH_ID=id1,id2,id3
SMARTY_AUTH_TOKEN=token1,token2,token3
```

- Round-robin across credentials
- Circuit breaker: skip credential on 429/402 errors
- Prevents quota exhaustion on single account

### ATMB vs iPost1 Differences

| Aspect | ATMB | iPost1 |
|--------|------|--------|
| Scraping | goquery (static HTML) | chromedp (Cloudflare bypass) |
| Discovery | Scrape index page | AJAX endpoints (/locations_ajax.php) |
| Parser | `crawler/parser.go` | `crawler/ipost1/parser.go` |
| Speed | Fast | Slower (browser automation) |

**Critical for iPost1**: Must establish browser session before AJAX calls, otherwise Cloudflare blocks.

### Mark-and-Sweep Deletion

After crawl completes, system detects removed locations:

1. Track all seen IDs during crawl
2. Query existing records for this source
3. Set `active=false` for unseen records
4. Preserves data (soft delete) for historical analysis

## Project Structure

```
apps/api/
├── cmd/
│   ├── server/main.go              # HTTP server entrypoint
│   ├── check-firestore/            # Utility commands
│   └── migrate-*/                  # One-time migrations
├── internal/
│   ├── business/crawler/           # Core crawling logic
│   │   ├── scraper.go              # ATMB scraper + CurrentParserVersion
│   │   ├── parser.go               # ATMB HTML parsing
│   │   ├── validation.go           # Smarty batch validation
│   │   ├── reprocess.go            # Re-parse from stored HTML
│   │   ├── orchestrator.go         # Worker pool management
│   │   ├── job_manager.go          # Job status tracking
│   │   ├── stats.go                # Aggregate statistics
│   │   └── ipost1/                 # iPost1-specific
│   │       ├── client.go           # chromedp automation
│   │       └── parser.go           # iPost1 HTML parsing
│   ├── platform/                   # External integrations
│   │   ├── config/                 # Environment config
│   │   ├── firestore/              # Firestore client
│   │   ├── smarty/                 # Smarty API (multi-cred)
│   │   └── http/                   # Gin router + middleware
│   └── repository/                 # Data access layer
│       ├── mailbox_repo.go         # CRUD + FetchAllMetadata()
│       ├── run_repo.go             # Job tracking
│       └── stats_repo.go           # Aggregate stats
├── pkg/model/                      # Shared types (see below)
└── scripts/                        # Each in own subdirectory
    ├── batch_validate/
    ├── check_cmra_rdi/
    └── ...

apps/web/                           # React frontend
└── src/
    ├── pages/                      # Mailboxes, Analytics, Crawler
    ├── components/                 # Reusable UI
    └── services/api.ts             # HTTP client
```

### Key Shared Types (`pkg/model/`)

- `Mailbox` - Core record (includes rawHTML, parserVersion, dataHash)
- `CrawlRun` - Job tracking (status, stats, errors)
- `AddressRaw` - User input address
- `StandardizedAddress` - Smarty validated address
- `Config` - Environment configuration

**Always use shared types**. Before creating a struct, run `Grep "type StructName struct"` to check for existing types.

## Go Development Rules

### 1. Struct Field Changes

When modifying `pkg/model/` structs:

1. Search for all usages: `Grep "model.StructName"` across codebase
2. Update all test files (e.g., `*_test.go`)
3. Update scripts that use the struct
4. Run `go build ./...` and `go test ./...` to verify

**Example**: Changing `Config.AuthID string` to `Config.AuthIDs []string` requires updating:
- `internal/platform/config/config.go`
- `internal/platform/smarty/client.go`
- All test files that construct `Config{}`

### 2. No Type Duplication

Use shared types from `pkg/model/`. Before creating a struct:

```bash
# Check if type exists
Grep "type Mailbox struct"
Grep "type CrawlRun struct"
```

Common violations:
- ❌ Defining local `Address` struct when `model.AddressRaw` exists
- ❌ Creating `MailboxData` when `model.Mailbox` should be used
- ✅ Import `github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model`

### 3. Scripts Directory Structure

Each standalone script MUST be in its own subdirectory:

```
scripts/
├── script_name_a/
│   └── main.go          # package main
└── script_name_b/
    └── main.go          # package main
```

**Why**: Prevents "main redeclared in this block" errors when multiple `package main` files exist in same directory.

### 4. Pre-Commit Verification

Before committing Go code:

```bash
go build ./...    # Verify builds
go vet ./...      # Static analysis
go test ./...     # Run tests
```

### 5. Parser Version Updates

When fixing parser bugs:

1. Update `CurrentParserVersion` constant in `scraper.go`
2. Commit and deploy
3. Test reprocessing: `POST /api/crawl/reprocess` with `{"outdatedOnly": true}`
4. Records update in ~2 minutes

### 6. Batch Validation is Critical

Never call Smarty API individually per address. Always use batch validation:

```go
// ❌ WRONG - exceeds quota
for _, addr := range addresses {
    smarty.Validate(addr)
}

// ✅ CORRECT - batch up to 100
smarty.BatchValidate(addresses)
```

## Environment Variables

Required for local development (`apps/api/.env.local`):

```bash
PORT=8080
GIN_MODE=debug
ALLOWED_ORIGINS=http://localhost:5173

# Firebase
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDS_FILE=service-account.json  # Local only
FIREBASE_CREDS_BASE64=...                 # Production (Render)

# Smarty (comma-separated for multiple accounts)
SMARTY_AUTH_ID=id1,id2,id3
SMARTY_AUTH_TOKEN=token1,token2,token3
SMARTY_MOCK=true                          # Skip real API calls in dev

# Crawler
CRAWLER_CONCURRENCY=5                     # Render free tier limit
```

**Mock mode**: Set `SMARTY_MOCK=true` to skip real Smarty API calls during development. Returns dummy CMRA=Y, RDI=Commercial.

## API Endpoints

### Crawl Control

| Endpoint | Purpose |
|----------|---------|
| `POST /api/crawl/run` | Start ATMB crawl |
| `POST /api/crawl/ipost1/run` | Start iPost1 crawl (chromedp) |
| `POST /api/crawl/reprocess` | Re-parse from stored HTML |
| `GET /api/crawl/status?runId=X` | Poll job status |
| `POST /api/crawl/runs/{runId}/cancel` | Cancel running job |

### Data Access

| Endpoint | Purpose |
|----------|---------|
| `GET /api/mailboxes` | List with filters (state, cmra, rdi, source, active) |
| `GET /api/mailboxes/export` | CSV streaming download |
| `GET /api/stats` | Dashboard metrics (reads 1 document) |
| `POST /api/stats/refresh` | Recompute aggregates |

## Common Pitfalls

1. **Forgetting to increment parser version** - Reprocess won't pick up changes
2. **Using individual Smarty calls** - Exceeds quota
3. **Not running `go build ./...` before commit** - Breaks CI
4. **Creating duplicate types** - Should use `pkg/model/`
5. **Multiple `main.go` in `scripts/`** - Must use subdirectories
6. **Checking `parsed.CMRA` after HTML parse** - Always empty, check DB values
7. **Setting CRAWLER_CONCURRENCY too high on Render** - Free tier limit is 5

## Firestore Indexes

Required composite indexes (create in Firebase Console):

- `(active, state)`
- `(active, rdi)`
- `(crawlRunId)`
- `(parserVersion, source)` - for reprocessing queries

## Performance Notes

- **Firestore free tier**: 50K reads/day, 20K writes/day
- **Typical crawl**: ~2K reads (metadata), ~2K writes (upserts)
- **Batch size**: Write every 20 items (balances memory vs failure recovery)
- **Job timeout**: 30 minutes max execution time
- **HTTP timeout**: 20 seconds per request (3 retries)

For detailed architecture documentation, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).
