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

## Key Architecture Concepts

This project uses several critical patterns unique to this codebase:

### Crawler Workflow
**Batch processing**: Collect 20 items → Batch validate with Smarty (100/request) → BatchUpsert to Firestore. This prevents API quota exhaustion (2000 addresses = 20 API calls vs 2000 individual calls).

### Parser Versioning
Records store `parserVersion` + `rawHTML`. To fix parser bugs: increment `CurrentParserVersion` → deploy → call `/api/crawl/reprocess`. Re-parses from stored HTML (~2 min vs ~30 min re-crawl).

### Metadata-Only Fetching
`FetchAllMetadata()` loads only `{id, link, dataHash, cmra, rdi}` instead of full documents (2MB vs 200MB). Enables 30+ crawls/day within Firestore free tier vs 3 before.

### ATMB vs iPost1
- **ATMB**: goquery (static HTML), fast
- **iPost1**: chromedp (browser automation), slower, requires session establishment before AJAX calls (Cloudflare bypass)

### Critical Gotchas
- **Never** call Smarty API individually per address (use batch validation)
- **Never** check `parsed.CMRA` after HTML parsing (always empty, check DB values)
- **Always** increment `CurrentParserVersion` when fixing parser bugs

**For detailed implementation patterns**, see `.claude/rules/` directory.

## Project Structure

```
apps/api/                           # Go Backend
├── cmd/server/main.go              # HTTP server entrypoint
├── internal/
│   ├── business/crawler/           # Core crawling logic
│   │   ├── scraper.go              # ATMB scraper + CurrentParserVersion
│   │   ├── parser.go               # HTML parsing
│   │   ├── validation.go           # Smarty batch validation
│   │   ├── reprocess.go            # Re-parse from stored HTML
│   │   ├── orchestrator.go         # Worker pool (5 concurrent)
│   │   └── ipost1/                 # iPost1-specific (chromedp)
│   ├── platform/                   # External integrations
│   │   ├── firestore/              # Firestore client
│   │   ├── smarty/                 # Smarty API (multi-cred load balancing)
│   │   └── http/                   # Gin router + middleware
│   └── repository/                 # Data access layer
│       ├── mailbox_repo.go         # CRUD + FetchAllMetadata()
│       └── stats_repo.go           # Aggregate stats (singleton pattern)
├── pkg/model/                      # Shared types (always use these)
└── scripts/                        # Each in own subdirectory

apps/web/                           # React Frontend
└── src/
    ├── pages/                      # Mailboxes, Analytics, Crawler
    ├── components/                 # Reusable UI (Toast, Badges, StatCard)
    ├── hooks/                      # useCSVExport, etc.
    └── contexts/                   # ToastContext
```

**Key Types** (`pkg/model/`): `Mailbox`, `CrawlRun`, `AddressRaw`, `StandardizedAddress`, `Config`, `SystemStats`

**Rule**: Always use shared types from `pkg/model/`. Before creating a struct, run `Grep "type StructName struct"`.

## Development Workflow

### Making Changes

1. **Check relevant rules**: See `.claude/rules/` for topic-specific patterns
2. **Make changes**
3. **Pre-commit checks**: `go build ./... && go vet ./... && go test ./...`
4. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/) format (see `.claude/rules/git-conventions.md`)

### Common Tasks

| Task | Steps |
|------|-------|
| Fix parser bug | 1. Increment `CurrentParserVersion` in `scraper.go`<br>2. Deploy<br>3. `POST /api/crawl/reprocess` with `{"outdatedOnly": true}` |
| Add API endpoint | Follow patterns in `.claude/rules/api-design.md` |
| Add frontend component | See `.claude/rules/frontend-components.md` |
| Modify struct in `pkg/model/` | See `.claude/rules/go-development.md` § Struct Field Changes |
| Write tests | See `.claude/rules/testing.md` |

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

**Mock mode**: Set `SMARTY_MOCK=true` to skip real Smarty API calls. Returns dummy CMRA=Y, RDI=Commercial.

## Quick Reference

### API Endpoints

**Crawl Control**:
- `POST /api/crawl/run` - Start ATMB crawl
- `POST /api/crawl/ipost1/run` - Start iPost1 crawl
- `POST /api/crawl/reprocess` - Re-parse from stored HTML
- `GET /api/crawl/status?runId=X` - Poll job status
- `POST /api/crawl/runs/{runId}/cancel` - Cancel running job

**Data Access**:
- `GET /api/mailboxes` - List with filters (state, cmra, rdi, source, active)
- `GET /api/mailboxes/export` - CSV streaming download
- `GET /api/stats` - Dashboard metrics (reads 1 document)
- `POST /api/stats/refresh` - Recompute aggregates

### Common Pitfalls

1. **Forgetting to increment parser version** → Reprocess won't pick up changes
2. **Using individual Smarty calls** → Exceeds quota (use batch validation)
3. **Not running `go build ./...` before commit** → Breaks CI
4. **Creating duplicate types** → Should use `pkg/model/`
5. **Multiple `main.go` in `scripts/`** → Must use subdirectories
6. **Checking `parsed.CMRA` after HTML parse** → Always empty, check DB values
7. **Setting CRAWLER_CONCURRENCY too high on Render** → Free tier limit is 5

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
- **Metadata-only fetching**: 90% reduction in reads (2MB vs 200MB for 2000 docs)

## Detailed Rules

For detailed implementation patterns, see `.claude/rules/`:

- **[go-development.md](.claude/rules/go-development.md)** - Go patterns, struct changes, imports, error handling
- **[crawler-architecture.md](.claude/rules/crawler-architecture.md)** - Batch processing, parser versioning, worker pools, deduplication
- **[api-design.md](.claude/rules/api-design.md)** - RESTful naming, request/response patterns, CORS, streaming
- **[frontend-components.md](.claude/rules/frontend-components.md)** - Toast system, badges, hooks, CSV export
- **[database.md](.claude/rules/database.md)** - Repository pattern, batch operations, indexes, streaming queries
- **[testing.md](.claude/rules/testing.md)** - Test organization, mocks, table-driven tests, coverage
- **[git-conventions.md](.claude/rules/git-conventions.md)** - Conventional Commits format, commit workflow, message guidelines

For comprehensive architecture documentation, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).
