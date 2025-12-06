# US Virtual Address Verification System

## Backend Architecture & Implementation Plan (v2.0)

### 1\. Architectural Goals

- **Zero-Cost Operation:** Optimized for Render (Go) and Firebase (NoSQL) free tiers.
- **Smarty API Conservation:** Aggressive caching and hashing strategies to minimize expensive API calls.
- **Resiliency:** Graceful handling of scraper blocks, API rate limits, and Render's ephemeral instance lifecycle.

### 2\. Project Structure (Refined)

Adopts a standard "Clean Architecture" layout to separate infrastructure from business logic.

```text
backend/
  cmd/
    server/
      main.go           # Entry point: Env loader, DI, Router setup
  internal/
    platform/           # Infrastructure & 3rd Party Clients
      firestore/        # DB initialization & raw client
      smarty/           # Smarty SDK wrapper (with rotation/retry logic)
      http/             # Web server config (Gin/Fiber)
    business/           # Core Logic
      crawler/          # Scraper logic, Worker Pools, Orchestration
      mailbox/          # CRUD service for mailboxes
      stats/            # Logic for aggregating and reading stats
    repository/         # DB Data Access Layer
      mailbox_repo.go
      run_repo.go
      stats_repo.go
  pkg/
    model/              # Structs & Entities
    util/               # Hashing, CSV streaming helpers
```

### 3\. Data Models (Firestore)

#### 3.1 Collection: `mailboxes`

Changes: Added `dataHash` for change detection and `active` for soft deletion.

```json
{
  "id": "auto-gen-id",
  "name": "ABC Store",
  "addressRaw": {
    "street": "123 Main St",
    "city": "Dover",
    "state": "DE",
    "zip": "19901"
  },
  "price": 12.99,
  "link": "https://anytimemailbox.com/...",
  // Validation Data
  "cmra": "Y",
  "rdi": "Commercial",
  "standardizedAddress": { ... },
  // Metadata
  "dataHash": "a1b2c3d4...", // MD5 of name + addressRaw
  "lastValidatedAt": "2025-01-01T12:00:00Z",
  "crawlRunId": "RUN_101",
  "active": true // Set to false if not found in current crawl
}
```

#### 3.2 Collection: `crawl_runs`

Tracks the lifecycle of the background job.

```json
{
  "runId": "RUN_101",
  "status": "running", // running, success, failed, partial_halt
  "stats": {
    "found": 2300,
    "validated": 50, // Actually called Smarty
    "skipped": 2250, // Skipped due to matching Hash
    "failed": 0
  },
  "startedAt": "...",
  "finishedAt": "..."
}
```

#### 3.3 Document: `system/stats` (Singleton)

**Critical for Free Tier:** Pre-calculated stats to avoid reading 2000+ docs for every dashboard refresh.

```json
{
  "lastUpdated": "2025-01-01T12:05:00Z",
  "totalMailboxes": 2300,
  "totalCommercial": 1500,
  "totalResidential": 800,
  "avgPrice": 14.50,
  "byState": { "CA": 200, "DE": 500, ... }
}
```

---

### 4\. Core Business Logic

#### 4.1 The "Smart" Crawler Service

To solve the "Smarty Cost" and "Data Consistency" problems, the crawler will follow this pipeline:

1.  **Initialization:** Generate `runId`. Fetch all existing mailboxes into a memory map (Key: Link, Value: Hash + ValidatedData) for quick lookup.
2.  **Scraping (Worker Pool):**
    - Limit concurrency to **5-10 workers** to prevent Render OOM (Out of Memory).
    - Scrape ATMB details.
3.  **Hash Comparison (The Cost Saver):**
    - Compute Hash of scraped name + address.
    - **IF** Hash matches existing DB record **AND** existing record has valid CMRA data:
      - **Skip Smarty.** Use existing CMRA/RDI data. Mark as "Skipped" in stats.
    - **ELSE:**
      - Send to `SmartyClient` for validation.
4.  **Circuit Breaker:**
    - If Smarty returns `402` (Payment Required) or `429` (Too Many Requests) \> 5 times consecutively, **pause** the validation phase but continue scraping (marking items as unvalidated) or halt entirely.
5.  **Batch Write:**
    - Write to Firestore in batches of 400-500 items to reduce network overhead.
6.  **Mark-and-Sweep:**
    - After scraping finishes, run a query: `UPDATE mailboxes SET active = false WHERE crawlRunId != currentRunId`. This handles "delisted" stores.
7.  **Pre-Aggregation:**
    - Calculate all dashboard stats (counts, averages) in memory.
    - Overwrite `system/stats` document.

#### 4.2 CSV Streaming Export

To prevent memory spikes when exporting data:

- **Do not** load all documents into a slice.
- Use `firestore.Query.Documents(ctx)` iterator.
- Wrap `http.ResponseWriter` in a `csv.NewWriter`.
- Flush the buffer every 50 rows to keep the connection alive.

---

### 5\. API Design (REST)

| Method   | Endpoint                | Purpose           | Notes                                                                 |
| :------- | :---------------------- | :---------------- | :-------------------------------------------------------------------- |
| **GET**  | `/api/mailboxes`        | List addresses    | Supports paging, filtering (`active=true` by default).                |
| **GET**  | `/api/stats`            | Dashboard metrics | **Reads only 1 document** (`system/stats`).                           |
| **GET**  | `/api/mailboxes/export` | Download CSV      | Uses stream processing.                                               |
| **POST** | `/api/crawl/run`        | Start Job         | Asynchronous. Returns `{ runId }` immediately.                        |
| **GET**  | `/api/crawl/status`     | Job Status        | Front-end must poll this every 30s-60s to keep Render instance awake. |

---

### 6\. Deployment & Configuration

#### 6.1 Render (Go Service)

- **Build Command:** `go build -o server cmd/server/main.go`
- **Start Command:** `./server`
- **Keep-Alive Strategy:** The React frontend "Status Page" will poll `/api/crawl/status` periodically. This prevents Render from spinning down the service during a long crawl job.

#### 6.2 Firestore Indexes (required via `firestore.indexes.json`)

- `active` (Asc) + `state` (Asc)
- `active` (Asc) + `rdi` (Asc)
- `crawlRunId` (Asc)

#### 6.3 Environment Variables

```bash
PORT=8080
GIN_MODE=release
# Firebase
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDS_BASE64=... # Base64 encoded service account JSON
# Smarty
SMARTY_AUTH_ID=...
SMARTY_AUTH_TOKEN=...
# Security
ALLOWED_ORIGINS=https://your-vercel-app.vercel.app
```

### 7\. Implementation Roadmap (MVP)

1.  **Phase 1: Setup & Foundation:** Init Go module, Firestore connection, and basic Router.

    - [ ] Initialize Go module and scaffold the clean layout from `docs/backend_development_plan.md` (`cmd/server`, `internal/{platform,business,repository}`, `pkg/{model,util}`) matching Firestore data models from the PRD.
    - [ ] Add config loader for env vars (`PORT`, `FIREBASE*PROJECT_ID`, `FIREBASE_CREDS_BASE64`, `SMARTY_*`, `ALLOWED_ORIGINS`, `GIN_MODE`) with dual Firestore auth modes: local JSON file vs base64 env.
    - [ ] Implement Firestore client bootstrap in `internal/platform/firestore` with context handling and verify connection in a minimal server start.
    - [ ] Define `pkg/model` structs for `mailboxes`, `crawl_runs`, and `system/stats` including fields like `dataHash`, `active`, `standardizedAddress`, stats schema.

2.  **Phase 2: Scraper Kernel & Hashing:** Implement ATMB scraping with GoColly or basic HTML parsing.
    - [ ] Create `testdata/sample_page.html` and a parser unit test to assert extracted `name/address/price/link` shape.
    - [ ] Build scraper (`GoColly` or `net/html`) to crawl state pages then detail pages, normalizing data to the mailbox model.
    - [ ] Add `MD5` hashing utility in `pkg/util` for name + addressRaw, and wire hash computation into scrape results.
    - [ ] Implement repository batch upsert (400–500 per batch) and in-memory lookup map keyed by link to reuse existing CMRA/RDI.
    - [ ] Wire initial flow: scrape → hash compare → write to Firestore with `crawlRunId` and `active=true`.
3.  **Phase 3: Crawler Orchestration**
    - [ ] Implement worker pool (5–10 workers) with context-aware cancellation/shutdown and backoff for scraper errors.
    - [ ] Load existing mailboxes into memory map at run start; mark new/updated items; accumulate stats counters for found/skipped/validated/failed.
    - [ ] Add mark-and-sweep pass to set `active=false` where `crawlRunId != currentRunId`.
          Track run lifecycle in `crawl_runs` via repository: `runId`, status transitions, timestamps, counters.
4.  **Phase 4: Smarty Integration:** Connect Smarty API with the circuit breaker.
    - [ ] Build Smarty client wrapper with auth/host config, 3x retry, and account rotation hooks; support `SMARTY_MOCK` short-circuit for local dev.
    - [ ] Implement circuit breaker that pauses/halts validation after 5 consecutive 402/429 while allowing scraping to continue.
    - [ ] Integrate into pipeline: on hash mismatch or missing CMRA/RDI, call Smarty, merge standardized address, update `lastValidatedAt`, and record validation stats.
5.  **Phase 5: API & UI Integration:** Connect React frontend to the endpoints.
    - [ ] Implement handlers (`/api/crawl/run`, `/api/crawl/status`, `/api/stats`, `/api/mailboxes`, `/api/mailboxes/export`) using `Gin/Fiber` with CORS; export uses streaming iterator + `csv.Writer` flush every ~50 rows.
    - [ ] Add `/healthz` for Render probes; ensure pagination/filtering on mailboxes (state/cmra/rdi/active).
    - [ ] Precompute and store `system/stats` document after each crawl; ensure indexes (`active+state`, `active+rdi`, `crawlRunId`) are defined.
    - [ ] Provide Render build/start commands and README notes (env examples, `SMARTY_MOCK` usage, keep-alive polling guidance).
