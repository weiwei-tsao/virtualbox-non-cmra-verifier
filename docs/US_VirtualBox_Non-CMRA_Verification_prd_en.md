## US Virtual Address Verification System (React + Go + Firestore)

### 1. Background & Goals

This system automatically scrapes address data from AnytimeMailbox (ATMB), validates it via the Smarty API (especially CMRA / RDI), and delivers an admin dashboard plus CSV export.

Upgrade goals:

- Frontend: React + Vite + Tailwind + DaisyUI  
- Backend: Go + Firestore (NoSQL)  
- Rebuild API for a modern, lightweight, deploy-friendly stack  
- Use only free-tier services for online deployment  

### 2. System Architecture (New)

```
Frontend (React + Vite + Tailwind)
        ↓ REST API
Backend (Go + Gin/Fiber)
        ↓ Firestore SDK
Firestore (NoSQL)
```

Deployment targets:

- Frontend → Vercel (free)
- Backend → Render Free Instance (Go)
- Firestore → Firebase Free Tier

### 3. Features (rebuilt data flow, same functional scope)

#### 3.1 Automated address scraping
- Crawl ATMB store list by state  
- Parallel crawl store detail pages  
- Clean data: name, address, price, link, etc.  
- Deduplicate and update Firestore  

#### 3.2 Smarty address validation
- Call Smarty Address API  
- Returns: CMRA flag, RDI (Residential/Commercial), standardized ZIP+4 address  
- Rotate multiple accounts with quota cool-down  
- Automatic retry on validation failures  

#### 3.3 Admin UI (React)
- Address table with pagination, search, filters  
- View CMRA / RDI labels  
- Export CSV  
- Trigger/monitor crawler runs  
- Show crawler run logs  

### 4. Firestore Data Model (Document NoSQL)

#### 4.1 `mailboxes` collection
```json
{
  "name": "ABC Mailbox Store",
  "street": "123 Main St",
  "city": "San Francisco",
  "state": "CA",
  "zip": "94105",
  "price": 12.99,
  "link": "https://anytimemailbox.com/...",
  "cmra": "N",
  "rdi": "Residential",
  "standardizedAddress": {
    "deliveryLine1": "",
    "lastLine": ""
  },
  "lastValidatedAt": "2025-01-01T12:00:00Z",
  "crawlRunId": "RUN_2025_01_01_001"
}
```

Indexes: `state, city` / `cmra, rdi` / `crawlRunId`

#### 4.2 `crawl_runs` collection
```json
{
  "startedAt": "2025-01-01T00:00:00Z",
  "finishedAt": "2025-01-01T00:12:00Z",
  "status": "success",
  "totalFound": 2300,
  "totalValidated": 2200,
  "totalFailed": 100,
  "errorsSample": [
    { "link": "...", "reason": "Smarty 402" }
  ]
}
```

### 5. Backend Design (Go + Firestore)

#### 5.1 Directory layout (example)
```
backend/
  cmd/
    server/main.go
  internal/
    http/
      handler/
      router.go
    service/
      crawl/
      mailbox/
    repository/
      mailbox_repo.go
      crawl_repo.go
    client/
      smarty/
      atmb/
    config/
    model/
  pkg/
    logger/
    csv/
```

#### 5.2 REST API
1) List mailboxes  
`GET /api/mailboxes`

Parameters:

| Param    | Example | Description       |
| -------- | ------- | ----------------- |
| page     | 1       | Page number       |
| pageSize | 50      | Items per page    |
| state    | CA      | Filter by state   |
| cmra     | N       | Commercial flag   |
| rdi      | Residential | Residential/Commercial |

Response:
```json
{ "items": [...], "total": 2280, "page": 1 }
```

2) Export CSV  
`GET /api/mailboxes/export` → `text/csv`

3) Trigger crawler manually  
`POST /api/crawl/run`  
Response: `{ "runId": "RUN_001" }`

4) Get crawler status  
`GET /api/crawl/status`

#### 5.3 Backend components
- Firestore client (`firestore.NewClient(ctx, projectID)`)  
- Worker pool for crawler concurrency (state pages / detail pages; rate-limit Smarty)  
- Smarty multi-account rotation (cool-down on 429/402; max 3 retries)  
- Single binary deploy (Render): `main.go` starts HTTP server; includes Firestore SDK, optional Cron, crawler service  

### 6. Frontend Design (React + Vite + Tailwind + DaisyUI)

#### 6.1 Directory layout
```
frontend/
  src/
    api/
    components/
    pages/
    hooks/
    types/
    utils/
    main.tsx
    App.tsx
```

#### 6.2 UI stack

| Tech | Purpose |
| ---- | ------- |
| Vite | Fast dev/build |
| React + Hooks | UI framework |
| Tailwind CSS | Utility-first CSS |
| DaisyUI | Component library |
| React Icons | Icons |
| TanStack Table (optional) | High-performance table |

#### 6.3 Pages
- MailboxesPage: table, filters (state/cmra/rdi), pagination, optional by-state stats, CSV export  
- CrawlStatusPage: recent runs, error logs, manual trigger button  

### 7. Deployment (all free-tier)

#### 7.1 Frontend on Vercel
- Connect GitHub → auto deploy  
- Env vars point to Render backend API  

#### 7.2 Backend on Render (free Go instance)
- Build Go service on Render  
- Provide Firestore credential JSON (Base64)  

#### 7.3 Firestore (Firebase Free Tier)
- 5GB storage, 50k reads/day free; sufficient for this project  

### 8. Future Enhancements
- Global error monitoring (Sentry)  
- Distributed rate limiting (for multiple Smarty accounts)  
- Admin authentication (Firebase Auth)  
- Analytics dashboard (state-level aggregation)  
