# Virtual Box Verifier - Architecture & Technical Documentation

> A full-stack application for scraping, validating, and managing US virtual mailbox addresses.

## Table of Contents

1. [Overview](#1-overview)
2. [System Architecture](#2-system-architecture)
3. [Data Models](#3-data-models)
4. [API Reference](#4-api-reference)
5. [Crawler Workflows](#5-crawler-workflows)
6. [Address Validation](#6-address-validation)
7. [Performance Optimizations](#7-performance-optimizations)
8. [Deployment](#8-deployment)
9. [Development Guide](#9-development-guide)

---

## 1. Overview

### Purpose

This system automatically scrapes virtual mailbox address data from multiple providers, validates addresses via the Smarty API (CMRA/RDI classification), and delivers an admin dashboard with CSV export.

### Data Sources

| Source     | Description        | Locations            |
| ---------- | ------------------ | -------------------- |
| **ATMB**   | AnytimeMailbox.com | ~2,000+ US locations |
| **iPost1** | iPost1.com         | ~4,000 US locations  |

### Tech Stack

| Layer                  | Technology                                    |
| ---------------------- | --------------------------------------------- |
| **Frontend**           | React 19 + TypeScript + Vite + TanStack Query |
| **Backend**            | Go 1.25 + Gin Framework                       |
| **Database**           | Firebase Firestore (NoSQL)                    |
| **Validation**         | Smarty Street API                             |
| **Browser Automation** | chromedp (for Cloudflare bypass)              |

### Deployment Targets (Free Tier)

- **Frontend**: Vercel
- **Backend**: Render (Go)
- **Database**: Firebase Firestore (50K reads/day free)

---

## 2. System Architecture

### High-Level Architecture

```
+-------------------------------------------------------------------+
|                    FRONTEND (React + TypeScript)                   |
|                        apps/web/                                   |
|   +-------------+  +-------------+  +------------------+           |
|   | Mailboxes   |  | Analytics   |  | Crawler Control  |          |
|   | (Dashboard) |  | (Charts)    |  | (Jobs/Status)    |          |
+-------+---------------+------------------+--------------------+----+
        |               |                  |
        v               v                  v
+-------------------------------------------------------------------+
|                    GO BACKEND (Gin Framework)                      |
|                        apps/api/                                   |
|  +--------------------------------------------------------------+  |
|  |                 HTTP Router + CORS Middleware                |  |
|  +--------------------------------------------------------------+  |
|                              |                                     |
|  +--------------------------------------------------------------+  |
|  |                     BUSINESS LAYER                           |  |
|  |  +---------------+  +-----------------+  +----------------+  |  |
|  |  | Mailbox Svc   |  | Crawler Service |  | Stats Service  |  |  |
|  |  +---------------+  +-----------------+  +----------------+  |  |
|  +--------------------------------------------------------------+  |
|                              |                                     |
|  +--------------------------------------------------------------+  |
|  |                   CRAWLER ENGINE                              |  |
|  |  +------------+  +------------+  +-------------+              |  |
|  |  | ATMB       |  | iPost1     |  | Reprocessor |              |  |
|  |  | Scraper    |  | Scraper    |  |             |              |  |
|  |  | (goquery)  |  | (chromedp) |  |             |              |  |
|  |  +-----+------+  +-----+------+  +------+------+              |  |
|  |        |               |                |                     |  |
|  |        +-------+-------+----------------+                     |  |
|  |                v                                              |  |
|  |        +----------------+                                     |  |
|  |        | Smarty API     |                                     |  |
|  |        | Batch Validator|                                     |  |
|  |        +----------------+                                     |  |
|  +--------------------------------------------------------------+  |
|                              |                                     |
|  +--------------------------------------------------------------+  |
|  |                   REPOSITORY LAYER                           |  |
|  |  +--------------+  +------------+  +---------------+         |  |
|  |  | MailboxRepo  |  | RunRepo    |  | StatsRepo     |         |  |
|  |  +--------------+  +------------+  +---------------+         |  |
|  +--------------------------------------------------------------+  |
+-------------------------------------------------------------------+
                               |
                               v
              +--------------------------------+
              |        FIRESTORE DATABASE      |
              |  +----------+ +-------------+  |
              |  | mailboxes| | crawl_runs  |  |
              |  +----------+ +-------------+  |
              |       +---------------+        |
              |       | system/stats  |        |
              |       +---------------+        |
              +--------------------------------+
```

### Project Structure

```
virtualbox-verifier/
├── apps/
│   ├── api/                          # Go Backend
│   │   ├── cmd/server/main.go        # HTTP server entrypoint
│   │   ├── internal/
│   │   │   ├── business/crawler/     # Crawling engine
│   │   │   │   ├── scraper.go        # ATMB scraper
│   │   │   │   ├── parser.go         # HTML parsing
│   │   │   │   ├── validation.go     # Smarty interface
│   │   │   │   ├── reprocess.go      # Re-parse from DB
│   │   │   │   ├── orchestrator.go   # Worker pool
│   │   │   │   ├── service.go        # High-level service
│   │   │   │   ├── discovery.go      # Link discovery
│   │   │   │   └── ipost1/           # iPost1 crawler
│   │   │   │       ├── client.go     # chromedp automation
│   │   │   │       └── parser.go     # iPost1 HTML parser
│   │   │   ├── platform/             # External integrations
│   │   │   │   ├── config/           # Environment config
│   │   │   │   ├── firestore/        # Firestore client
│   │   │   │   ├── http/             # Gin router
│   │   │   │   └── smarty/           # Smarty API client
│   │   │   └── repository/           # Data persistence
│   │   ├── pkg/model/                # Shared models
│   │   └── scripts/                  # Utility scripts
│   │
│   └── web/                          # React Frontend
│       └── src/
│           ├── pages/                # Mailboxes, Analytics, Crawler
│           ├── components/           # Reusable UI
│           └── services/api.ts       # HTTP client
│
└── CLAUDE.md                         # Development guidelines
```

---

## 3. Data Models

### Firestore Collections

#### `mailboxes` Collection

```json
{
  "id": "ATMB_CA_a1b2c3d4",
  "source": "ATMB",
  "name": "ABC Mailbox Store",
  "addressRaw": {
    "street": "123 Main St",
    "city": "San Francisco",
    "state": "CA",
    "zip": "94105"
  },
  "standardizedAddress": {
    "deliveryLine1": "123 MAIN ST",
    "lastLine": "SAN FRANCISCO CA 94105-1234",
    "fullAddress": "123 MAIN ST, SAN FRANCISCO CA 94105-1234"
  },
  "price": 12.99,
  "link": "https://anytimemailbox.com/...",
  "cmra": "Y",
  "rdi": "Commercial",
  "dataHash": "a1b2c3d4...",
  "lastValidatedAt": "2025-01-01T12:00:00Z",
  "crawlRunId": "RUN_1704067200",
  "active": true,
  "rawHTML": "<html>...</html>",
  "parserVersion": "v1.1",
  "lastParsedAt": "2025-01-01T12:00:00Z"
}
```

| Field           | Purpose                                     |
| --------------- | ------------------------------------------- |
| `dataHash`      | MD5 of name + address for deduplication     |
| `rawHTML`       | Stored for reprocessing without re-fetching |
| `parserVersion` | Tracks parser logic version                 |
| `active`        | Soft delete flag (false = delisted)         |

#### `crawl_runs` Collection

```json
{
  "runId": "RUN_1704067200",
  "source": "ATMB",
  "status": "success",
  "stats": {
    "found": 2300,
    "validated": 50,
    "skipped": 2250,
    "failed": 0
  },
  "startedAt": "2025-01-01T00:00:00Z",
  "finishedAt": "2025-01-01T00:12:00Z",
  "errorsSample": [{ "link": "...", "message": "timeout", "timestamp": "..." }]
}
```

**Status Values**: `running` | `success` | `failed` | `partial_halt` | `timeout` | `cancelled`

#### `system/stats` Document (Singleton)

```json
{
  "lastUpdated": "2025-01-01T12:05:00Z",
  "totalMailboxes": 4108,
  "totalCommercial": 2500,
  "totalResidential": 1608,
  "avgPrice": 14.5,
  "byState": { "CA": 500, "TX": 400, "FL": 350 },
  "bySource": { "ATMB": 2073, "iPost1": 2035 }
}
```

---

## 4. API Reference

### Health Check

| Method | Endpoint   | Description    |
| ------ | ---------- | -------------- |
| GET    | `/healthz` | Liveness probe |

### Mailbox Management

| Method | Endpoint                | Description                    |
| ------ | ----------------------- | ------------------------------ |
| GET    | `/api/mailboxes`        | List with filters & pagination |
| GET    | `/api/mailboxes/export` | CSV streaming download         |

**Query Parameters for `/api/mailboxes`**:

| Param      | Example    | Description            |
| ---------- | ---------- | ---------------------- |
| `page`     | 1          | Page number            |
| `pageSize` | 50         | Items per page (10-50) |
| `state`    | CA         | Filter by state        |
| `cmra`     | Y          | CMRA flag (Y/N)        |
| `rdi`      | Commercial | RDI value              |
| `source`   | ATMB       | Data source            |
| `active`   | true       | Active status          |

### Crawl Control

| Method | Endpoint                         | Description               |
| ------ | -------------------------------- | ------------------------- |
| POST   | `/api/crawl/run`                 | Start ATMB crawl          |
| POST   | `/api/crawl/ipost1/run`          | Start iPost1 crawl        |
| POST   | `/api/crawl/reprocess`           | Re-parse from stored HTML |
| GET    | `/api/crawl/status?runId=X`      | Job status polling        |
| GET    | `/api/crawl/runs?limit=20`       | Recent job history        |
| POST   | `/api/crawl/runs/{runId}/cancel` | Cancel running job        |

**Reprocess Request Body**:

```json
{
  "outdatedOnly": true,
  "forceRevalidate": false
}
```

### Statistics

| Method | Endpoint             | Description                          |
| ------ | -------------------- | ------------------------------------ |
| GET    | `/api/stats`         | Dashboard metrics (reads 1 document) |
| POST   | `/api/stats/refresh` | Recompute aggregates                 |

---

## 5. Crawler Workflows

### ATMB Scraping Flow

```
1. POST /api/crawl/run
   |
2. Create CrawlRun (status=running)
   |
3. FetchAllMetadata() - Load existing hashes for deduplication
   |
4. Worker Pool (5 concurrent workers)
   |
   +---> For each location URL:
   |     a. Fetch HTML (20s timeout, 3 retries)
   |     b. Parse with goquery (name, address, price)
   |     c. Compute dataHash
   |     d. Skip if hash unchanged AND has CMRA
   |     e. Collect in buffer
   |
5. Every 20 items:
   |     a. Batch validate with Smarty API (up to 100/request)
   |     b. BatchUpsert to Firestore
   |
6. Mark-and-Sweep: Set active=false for missing records
   |
7. Aggregate system stats
   |
8. Update CrawlRun (status=success/failed)
```

### iPost1 Scraping Flow

Uses chromedp for Cloudflare bypass:

```
1. POST /api/crawl/ipost1/run
   |
2. Initialize chromedp (headless Chrome)
   |
3. Navigate to ipost1.com (establishes session)
   |
4. GET /locations_ajax.php?action=get_states_list
   |
5. For each state:
   |     GET /locations_ajax.php?action=get_mail_centers&state_id={id}
   |     Parse HTML fragment with goquery
   |
6. Same batch validation & upsert as ATMB
```

### Reprocessing Flow

Re-parse stored HTML without re-fetching (15x faster):

```
1. POST /api/crawl/reprocess
   |
2. Query mailboxes WHERE parserVersion < current
   |
3. For each batch (100 records):
   |     a. Load rawHTML from document
   |     b. Re-parse with latest parser logic
   |     c. Optionally re-validate with Smarty
   |     d. BatchUpsert with new parserVersion
```

**Use Cases**:

- Parser bug fixes
- Parser improvements
- Test changes in ~2 minutes vs 30+ minutes

---

## 6. Address Validation

### Smarty API Integration

**Endpoint**: `https://us-street.api.smarty.com/street-address`

**Key Fields**:

| Field | Location in Response | Values                            |
| ----- | -------------------- | --------------------------------- |
| CMRA  | `analysis.dpv_cmra`  | "Y" / "N" / ""                    |
| RDI   | `metadata.rdi`       | "Commercial" / "Residential" / "" |

### Batch Validation

Single API call validates up to 100 addresses:

```go
// POST request with array of addresses
[
  {"street": "123 Main", "city": "Dover", "state": "DE"},
  {"street": "456 Oak", "city": "Newark", "state": "DE"}
]
```

**Benefits**:

- 2000 addresses = 20 API calls (vs 2000 individual calls)
- 99% reduction in API usage

### Multi-Credential Load Balancing

```
Request
   |
   v
Smarty Client (round-robin)
   |
   +---> Credential 1 ----+
   +---> Credential 2 ----+---> Circuit Breaker
   +---> Credential 3 ----+     (skip on 429/402)
```

### API Cost Comparison

| Service    | CMRA | RDI | Free Tier | Cost      |
| ---------- | ---- | --- | --------- | --------- |
| **Smarty** | Yes  | Yes | 250/month | $20/month |
| Geocodio   | No   | Yes | 2,500/day | Free      |
| PostGrid   | Yes  | Yes | None      | $18/month |
| USPS       | No   | No  | 60/hour   | Free      |

**Recommendation**: Smarty for complete CMRA + RDI support.

---

## 7. Performance Optimizations

### 1. Incremental Database Writes

Writes every 20 processed items to:

- Reduce memory footprint
- Enable resume-on-failure
- Prevent data loss on crashes

### 2. Metadata-Only Fetching (90% Cost Reduction)

```go
// Full fetch: ~200MB for 2000 docs
FetchAllMap()

// Metadata only: ~2MB for 2000 docs
FetchAllMetadata()  // Only: id, link, dataHash, cmra, rdi
```

**Impact**: Can run 30+ crawls/day within free tier (vs 3 before)

### 3. Batch Smarty API Calls

| Approach    | 5000 Addresses |
| ----------- | -------------- |
| Individual  | 5000 API calls |
| Batch (100) | 50 API calls   |

### 4. Parser Versioning

Track `parserVersion` per record to:

- Enable selective reprocessing
- Support non-destructive parser updates
- Avoid re-scraping for bug fixes

### 5. Data Deduplication

Use `dataHash` (MD5 of name + address) to:

- Skip unchanged records
- Preserve existing CMRA/RDI values
- Minimize API calls

---

## 8. Deployment

### Environment Variables

```bash
# Server
PORT=8080
GIN_MODE=release

# Firebase
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDS_BASE64=...  # OR
FIREBASE_CREDS_FILE=/path/to/creds.json

# Smarty (comma-separated for multiple accounts)
SMARTY_AUTH_ID=id1,id2,id3
SMARTY_AUTH_TOKEN=token1,token2,token3
SMARTY_MOCK=false  # true for development

# Security
ALLOWED_ORIGINS=https://your-app.vercel.app

# Crawler
CRAWLER_CONCURRENCY=5
```

### Render (Backend)

- **Build**: `go build -o server cmd/server/main.go`
- **Start**: `./server`
- **Keep-Alive**: Frontend polls `/api/crawl/status` to prevent spin-down

### Vercel (Frontend)

- Connect GitHub repo
- Set `VITE_API_URL` environment variable

### Firestore Indexes

Required composite indexes:

- `(active, state)`
- `(active, rdi)`
- `(crawlRunId)`

---

## 9. Development Guide

### Local Setup

```bash
# Backend
cd apps/api
go mod download
SMARTY_MOCK=true go run cmd/server/main.go

# Frontend
cd apps/web
npm install
npm run dev
```

### Running Tests

```bash
cd apps/api
go test ./... -v
```

### Key Development Rules

1. **Struct Changes**: When modifying `pkg/model/`, search for all usages and update tests
2. **No Duplication**: Use shared types from `pkg/model/`
3. **Scripts**: Each script in its own subdirectory under `scripts/`
4. **Pre-commit**: Run `go build ./...` and `go test ./...`

### Parser Version Updates

When fixing parser bugs:

1. Update `CurrentParserVersion` in `scraper.go`
2. Deploy updated code
3. Call `POST /api/crawl/reprocess` with `outdatedOnly: true`
4. Records update in ~2 minutes without re-fetching

---

## Quick Reference

### Job Timeouts

| Layer                | Timeout    |
| -------------------- | ---------- |
| Job execution        | 30 minutes |
| HTTP requests        | 20 seconds |
| Zombie job detection | 45 minutes |

### Data Scale

| Metric           | Value  |
| ---------------- | ------ |
| ATMB locations   | ~2,073 |
| iPost1 locations | ~2,035 |
| Total mailboxes  | ~4,108 |
| HTML storage     | ~150MB |

### API Rate Limits

| Service          | Limit                         |
| ---------------- | ----------------------------- |
| Smarty (paid)    | Per subscription              |
| Firestore (free) | 50K reads/day, 20K writes/day |

---

_Last updated: 2026-01-10_
