# Project Evaluation & Backend System Design

## 1. Architecture Evaluation

The proposed **React (Frontend) + Go (Backend) + Firestore (DB)** architecture is **highly recommended** for this specific use case for the following reasons:

- **Concurrency**: Go is ideal for writing high-concurrency scrapers (using Goroutines) to handle 50+ states and thousands of mailbox pages efficiently without blocking.
- **Flexible Schema**: Firestore (NoSQL) is perfect for address data where fields might vary slightly between scrapers or standardized results.
- **Free Tier Feasibility**:
  - **Vercel** hosts the React app for free.
  - **Render** offers free Go instances (note: they spin down after inactivity, which is fine for a dashboard, but you might need a "keep-alive" or use a Cron job trigger).
  - **Firebase** offers a generous free tier (50k reads/day) which fits the volume of ATMB data (approx 2k-3k locations).

## 2. Backend Design Recommendations

### Folder Structure (Refined)

To keep the single binary deployment on Render simple yet professional:

```
backend/
├── cmd/
│   └── server/
│       └── main.go         # Entry point: Starts HTTP server & loads config
├── internal/
│   ├── api/
│   │   └── handlers.go     # Gin/Fiber handlers for /api/mailboxes
│   ├── scraper/
│   │   ├── collector.go    # Colly or Goquery logic for ATMB
│   │   └── worker.go       # Worker pool to manage concurrency
│   ├── smarty/
│   │   └── client.go       # Smarty API wrapper with rate limiting
│   └── repository/
│       └── firestore.go    # DB interactions
└── pkg/
    └── models/             # Shared structs (Mailbox, RunStatus)
```

### Critical Implementation Details

1.  **Rate Limiting & Rotation**:

    - When implementing the `scraper`, ensure you add random delays (2-5s) between requests to avoid IP bans from ATMB.
    - For Smarty, implement a `TokenBucket` in Go to respect your plan's QPS limits.

2.  **State Management**:

    - Since Render free instances might restart, do not store "Crawl State" in memory variables.
    - Update the `crawl_runs` Firestore document frequently (e.g., every 50 items processed) so if the server restarts, it knows where it left off (or at least reports failure correctly).

3.  **Address Normalization**:

    - Store the _raw_ address scraped from ATMB separately from the _standardized_ address returned by Smarty. This allows you to re-validate later without re-scraping if validation logic changes.

4.  **Deployment**:
    - **Dockerfile**: Create a multi-stage Dockerfile to compile the Go binary into a scratch/alpine image (very small, <20MB) for faster Render deployments.

## 3. Frontend Features (Implemented)

The generated React code includes:

- **Dashboard**: Filtering by State, CMRA, and RDI status.
- **Analytics**: Visual breakdown of Residential vs Commercial addresses using Recharts.
- **Crawler Control**: UI to trigger the backend job and view history.
- **Mock Service**: A simulation layer so you can run this UI immediately to verify the UX before connecting the real Go backend.

## 4. Backend quick start (local)

Minimal steps to run the Go server with Firestore:

```bash
cd apps/api
cat > .env.local <<'EOF'
# Server Configuration
PORT=8080

# 本地开发用 debug (日志全), 线上部署改为 release (性能高)
GIN_MODE=debug

# CORS 设置，本地开发通常允许前端 http://localhost:5173
ALLOWED_ORIGINS=http://localhost:5173,https://your-vercel-app.vercel.app

# Firebase / Firestore Configuration
FIREBASE_PROJECT_ID=your-project-id
# [线上/Render] 必须：将 service-account.json 内容转为 Base64 填入此处
# FIREBASE_CREDS_BASE64=
# [本地开发] 可选：虽然代码会优先找根目录的 service-account.json 文件，
# 但显式指定路径有时能避免路径错误问题 (需代码支持，若代码硬编码了文件名则忽略此行)
GOOGLE_APPLICATION_CREDENTIALS=./service-account.json

# Smarty Address Verification
SMARTY_AUTH_ID=your-smarty-id
SMARTY_AUTH_TOKEN=your-smarty-token
# [省钱开关] true: 不会真的调用 Smarty API，返回模拟数据; false: 真实计费调用
SMARTY_MOCK=true

# Crawler Tuning (Render Free Tier Safety)
# [关键] 爬虫并发 Worker 数量。
# 本地可开 10-20，Render 免费版建议设置为 5 以防内存溢出 (OOM)
CRAWLER_CONCURRENCY=5
# Smarty 调用重试次数
SMARTY_MAX_RETRIES=3
EOF

env $(cat .env.local | xargs) go run ./cmd/server
```

Notes:

- The backend expects either `FIREBASE_CREDS_BASE64` or `FIREBASE_CREDS_FILE` to be set (one is required).
- Keep `SMARTY_MOCK=true` if you do not want live Smarty calls during local development.
- Health check is available at `http://localhost:8080/healthz`.
