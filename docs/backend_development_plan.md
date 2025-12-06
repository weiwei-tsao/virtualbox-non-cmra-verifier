## Backend Development Plan (MVP, Layered Design)

### Goals
- Minimal-effort MVP that unblocks the existing React UI.
- Clean layering: handler → service → repository/client.
- Runs on Render free tier; Firestore as primary store; Smarty for validation.

### Scope (must-have for UI)
- Mailbox listing with filters/pagination/search.
- Stats for totals/commercial/residential/byState/avgPrice.
- CSV export of mailboxes.
- Trigger crawl job + view recent crawl runs/status/errors.

### Architecture & Folders
```
backend/
  cmd/server/main.go      # Bootstrap, DI, HTTP server, routes, config
  internal/
    handler/              # HTTP handlers (Gin/Fiber)
    service/              # Business logic
    repository/           # Firestore persistence
    client/               # Smarty API, ATMB scraper
    job/                  # Crawl job orchestration
  pkg/
    model/                # Shared structs (Mailbox, CrawlRun, filters)
    logger/               # Logging helpers
```

### Data Models (align with frontend)
- `Mailbox`: id, name, street, city, state, zip, price (number), link, cmra ("Y"/"N"), rdi ("Residential"/"Commercial"), standardizedAddress { deliveryLine1, lastLine }, lastValidatedAt (ISO string), crawlRunId.
- `CrawlRun`: id, startedAt, finishedAt, status ("running"|"success"|"failed"), totalFound, totalValidated, totalFailed, errorsSample[] (link, reason).
- `StatsResponse`: totalMailboxes, commercialCount, residentialCount, avgPrice, byState[{ name, value }].

### Repositories (Firestore)
- `MailboxesRepo`: List(filter, pagination, search), UpsertBatch(mailboxes), ExportStream(writer).
- `CrawlRunsRepo`: Create(run), Update(run fields), ListRecent(limit).
- Index guidance: state+city, cmra+rdi, crawlRunId.

### Clients
- `SmartyClient`: ValidateAddress(raw) -> standardizedAddress, cmra, rdi. Include rate limiting + retry (3) + optional multi-key rotation.
- `AtmbScraper`: Fetch state listings + detail pages; yields raw mailbox candidates.

### Services
- `MailboxService`: orchestrates list/search, stats aggregation, export streaming.
- `CrawlService`: orchestrates crawl job: scrape → normalize → dedupe → validate via Smarty → upsert; updates CrawlRunsRepo for progress/final status.
- `StatsService`: derive stats from Firestore (or from MailboxesRepo aggregation).

### HTTP API (match frontend expectations)
- `GET /api/mailboxes?page=&pageSize=&state=&cmra=&rdi=&search=` -> `{ items, total, page }`
- `GET /api/stats` -> stats payload used by Analytics page.
- `GET /api/mailboxes/export` -> CSV stream.
- `POST /api/crawl/run` -> `{ runId }` (starts background job, return immediately).
- `GET /api/crawl/status` -> list of recent runs with status/errorsSample/timestamps.
- CORS: allow Vercel origin; JSON error envelope.

### Background Job Strategy (MVP-friendly)
- Single active crawl at a time; queue a new request by returning last active runId if running.
- Worker pool with bounded concurrency for scraping + validation; backoff on Smarty errors.
- Periodic progress writes to `crawl_runs` so restarts are visible (no full resume needed for MVP).

### Config & Deployment
- Env vars: `PORT`, `FIREBASE_PROJECT_ID`, `FIREBASE_CREDS_B64`, `SMARTY_KEYS` (comma), `ALLOWED_ORIGIN`.
- Dockerfile (multi-stage) for Render; health endpoint `/healthz`.
- `.env.example` documenting required variables.

### Testing
- Unit tests: SmartyClient (httpmock), repositories (Firestore emulator), services with fakes.
- Basic integration: start server against emulator, hit `/api/mailboxes` and `/api/stats`.
