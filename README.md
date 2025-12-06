# Project Evaluation & Backend System Design

## 1. Architecture Evaluation

The proposed **React (Frontend) + Go (Backend) + Firestore (DB)** architecture is **highly recommended** for this specific use case for the following reasons:

*   **Concurrency**: Go is ideal for writing high-concurrency scrapers (using Goroutines) to handle 50+ states and thousands of mailbox pages efficiently without blocking.
*   **Flexible Schema**: Firestore (NoSQL) is perfect for address data where fields might vary slightly between scrapers or standardized results.
*   **Free Tier Feasibility**:
    *   **Vercel** hosts the React app for free.
    *   **Render** offers free Go instances (note: they spin down after inactivity, which is fine for a dashboard, but you might need a "keep-alive" or use a Cron job trigger).
    *   **Firebase** offers a generous free tier (50k reads/day) which fits the volume of ATMB data (approx 2k-3k locations).

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
    *   When implementing the `scraper`, ensure you add random delays (2-5s) between requests to avoid IP bans from ATMB.
    *   For Smarty, implement a `TokenBucket` in Go to respect your plan's QPS limits.

2.  **State Management**:
    *   Since Render free instances might restart, do not store "Crawl State" in memory variables.
    *   Update the `crawl_runs` Firestore document frequently (e.g., every 50 items processed) so if the server restarts, it knows where it left off (or at least reports failure correctly).

3.  **Address Normalization**:
    *   Store the *raw* address scraped from ATMB separately from the *standardized* address returned by Smarty. This allows you to re-validate later without re-scraping if validation logic changes.

4.  **Deployment**:
    *   **Dockerfile**: Create a multi-stage Dockerfile to compile the Go binary into a scratch/alpine image (very small, <20MB) for faster Render deployments.

## 3. Frontend Features (Implemented)

The generated React code includes:
*   **Dashboard**: Filtering by State, CMRA, and RDI status.
*   **Analytics**: Visual breakdown of Residential vs Commercial addresses using Recharts.
*   **Crawler Control**: UI to trigger the backend job and view history.
*   **Mock Service**: A simulation layer so you can run this UI immediately to verify the UX before connecting the real Go backend.
