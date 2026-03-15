# API Design Patterns

## RESTful Naming Conventions

### Resource Endpoints

```
/api/mailboxes          GET    - List mailboxes
/api/mailboxes/export   GET    - Export mailboxes as CSV
/api/stats              GET    - Get statistics
/api/stats/refresh      POST   - Recompute statistics

/api/crawl/run          POST   - Start ATMB crawl
/api/crawl/ipost1/run   POST   - Start iPost1 crawl
/api/crawl/reprocess    POST   - Reprocess from stored HTML
/api/crawl/status       GET    - Get crawl status
/api/crawl/runs         GET    - List crawl runs
/api/crawl/runs/:runId/cancel  POST  - Cancel specific run
```

### Naming Rules

1. **Use nouns for resources**: `/api/mailboxes`, not `/api/getMailboxes`
2. **Use plural forms**: `/api/mailboxes`, not `/api/mailbox`
3. **Use HTTP verbs**: `GET`, `POST`, not part of URL
4. **Nested resources**: `/api/crawl/runs/:runId/cancel`
5. **Actions as verbs**: `/api/stats/refresh`, `/api/crawl/run`

---

## Request Structure

### Query Parameters

Use for filtering, pagination, and sorting:

```
GET /api/mailboxes?state=CA&cmra=Y&page=1&pageSize=50
```

### Request Body

Use for complex operations:

```json
POST /api/crawl/reprocess
{
  "outdatedOnly": true,
  "forceRevalidate": false
}
```

### Path Parameters

Use for resource identification:

```
POST /api/crawl/runs/:runId/cancel
```

---

## Response Structure

### Success Response

```go
// Single resource
c.JSON(http.StatusOK, mailbox)

// Collection with pagination
c.JSON(http.StatusOK, gin.H{
    "items": items,
    "total": total,
    "page":  page,
})

// Action confirmation
c.JSON(http.StatusOK, gin.H{
    "runId":   runID,
    "message": "Crawl started successfully",
})
```

### Error Response

```go
// Client error (400)
c.JSON(http.StatusBadRequest, gin.H{
    "error": "runId is required",
})

// Server error (500)
c.JSON(http.StatusInternalServerError, gin.H{
    "error": "Failed to fetch mailboxes",
})
```

### Response Format Pattern

Always use `gin.H{}` for JSON responses:

```go
// ✅ Correct - Consistent format
c.JSON(http.StatusOK, gin.H{
    "key": "value",
})

// ❌ Avoid - Inconsistent
c.JSON(http.StatusOK, map[string]interface{}{
    "key": "value",
})
```

---

## Query Parameter Patterns

### Pagination

```go
func (r *Router) listMailboxes(c *gin.Context) {
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

    // Validate ranges
    if pageSize > 100 {
        pageSize = 100
    }
    if pageSize < 10 {
        pageSize = 10
    }

    // ...
}
```

**Pattern**: Always provide defaults and validate ranges.

### Boolean Parameters

```go
// Parse optional boolean
activeParam := c.Query("active")
var activePtr *bool
if activeParam != "" {
    val := activeParam == "true"
    activePtr = &val
}
```

**Pattern**: Use pointer for optional booleans (distinguish between `false` and not provided).

### Filter Parameters

```go
query := repository.MailboxQuery{
    State:    c.Query("state"),     // Optional
    CMRA:     c.Query("cmra"),      // Optional
    RDI:      c.Query("rdi"),       // Optional
    Source:   c.Query("source"),    // Optional
    Active:   activePtr,            // Optional boolean
    Page:     page,
    PageSize: pageSize,
}
```

**Pattern**: Empty string means "no filter", not an error.

---

## Dynamic Filename Generation

Export endpoints should generate descriptive filenames based on active filters.

### Pattern

```go
func generateExportFilename(query repository.MailboxQuery) string {
    parts := []string{"mailbox"}

    // Add filter segments in consistent order
    if query.State != "" {
        parts = append(parts, query.State)
    }
    if query.Source != "" {
        parts = append(parts, query.Source)
    }
    if query.CMRA != "" {
        parts = append(parts, query.CMRA)
    }
    if query.RDI != "" {
        parts = append(parts, query.RDI)
    }

    // Add timestamp in UTC (RFC3339 basic format)
    timestamp := time.Now().UTC().Format("20060102T150405Z")
    parts = append(parts, timestamp)

    return strings.Join(parts, "-") + ".csv"
}
```

### Examples

| Filters | Filename |
|---------|----------|
| None | `mailbox-20260313T142530Z.csv` |
| State=CA | `mailbox-CA-20260313T142530Z.csv` |
| State=CA, Source=ATMB | `mailbox-CA-ATMB-20260313T142530Z.csv` |
| Full filters | `mailbox-CA-ATMB-Y-Commercial-20260313T142530Z.csv` |

### Why This Pattern?

- **Descriptive**: User knows what's in the file
- **Sortable**: Timestamp allows chronological sorting
- **Consistent**: Always same order (state, source, cmra, rdi, timestamp)
- **Clean**: Empty filters are skipped

---

## CORS Middleware

### Implementation

```go
func (r *Router) corsMiddleware() gin.HandlerFunc {
    origins := strings.Split(r.origins, ",")
    trimmed := make([]string, 0, len(origins))
    for _, o := range origins {
        if t := strings.TrimSpace(o); t != "" {
            trimmed = append(trimmed, t)
        }
    }

    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        allowed := "*"
        for _, o := range trimmed {
            if o == "*" || o == origin {
                allowed = origin
                break
            }
        }

        c.Header("Access-Control-Allow-Origin", allowed)
        c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        c.Header("Access-Control-Expose-Headers", "Content-Disposition")

        if c.Request.Method == http.MethodOptions {
            c.Status(http.StatusNoContent)
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### Critical Header for Downloads

```go
c.Header("Access-Control-Expose-Headers", "Content-Disposition")
```

**Why**: Frontend needs to read `Content-Disposition` header to extract dynamic filename. Without this, the header is hidden by browser CORS policy.

---

## Streaming Responses

Use streaming for large datasets (CSV exports, bulk operations).

### Pattern

```go
func (r *Router) exportMailboxes(c *gin.Context) {
    // Set headers first
    c.Header("Content-Type", "text/csv")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

    // Create CSV writer that writes directly to response
    writer := csv.NewWriter(c.Writer)
    defer writer.Flush()

    // Write header
    if err := writer.Write([]string{"name", "street", "city", "state", "zip"}); err != nil {
        c.Status(http.StatusInternalServerError)
        return
    }

    // Stream records one by one
    err := r.mailboxes.StreamWithQuery(c.Request.Context(), query, func(mb model.Mailbox) error {
        row := []string{mb.Name, mb.AddressRaw.Street, mb.AddressRaw.City, mb.AddressRaw.State, mb.AddressRaw.Zip}
        return writer.Write(row)
    })

    if err != nil {
        c.Status(http.StatusInternalServerError)
        return
    }
}
```

### Why Streaming?

| Approach | Memory Usage | Time to First Byte | Max Records |
|----------|--------------|-------------------|-------------|
| Load all in memory | High (200MB+) | Slow (wait for all) | Limited by RAM |
| Streaming | Low (constant) | Fast (immediate) | Unlimited |

---

## Status Code Usage

### Standard Patterns

| Status | When to Use | Example |
|--------|-------------|---------|
| 200 OK | Successful GET/POST | `c.JSON(http.StatusOK, data)` |
| 204 No Content | Successful OPTIONS | `c.Status(http.StatusNoContent)` |
| 400 Bad Request | Invalid input | `c.JSON(http.StatusBadRequest, gin.H{"error": "runId required"})` |
| 404 Not Found | Resource not found | `c.JSON(http.StatusNotFound, gin.H{"error": "Mailbox not found"})` |
| 500 Internal Server Error | Server error | `c.JSON(http.StatusInternalServerError, gin.H{"error": msg})` |

### Pattern

```go
func (r *Router) handler(c *gin.Context) {
    // Validate input
    if c.Query("id") == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
        return
    }

    // Try to get resource
    item, err := r.repo.Get(c.Request.Context(), id)
    if err == repository.ErrNotFound {
        c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
        return
    }
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch item"})
        return
    }

    // Success
    c.JSON(http.StatusOK, item)
}
```

---

## Content-Disposition Header

For file downloads, use `Content-Disposition` header:

```go
filename := generateExportFilename(query)
c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
```

### Filename Escaping

Always quote filenames to handle special characters:

```go
// ✅ Correct - Quoted
fmt.Sprintf("attachment; filename=\"%s\"", filename)

// ❌ Wrong - Unquoted (breaks with spaces)
fmt.Sprintf("attachment; filename=%s", filename)
```

---

## Request Validation

### Struct Binding

```go
type reprocessReq struct {
    TargetVersion   string `json:"targetVersion"`
    OnlyOutdated    bool   `json:"onlyOutdated"`
    ForceRevalidate bool   `json:"forceRevalidate"`
}

func (r *Router) reprocessMailboxes(c *gin.Context) {
    var req reprocessReq
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
        return
    }

    // Use req.TargetVersion, req.OnlyOutdated, etc.
}
```

### Pattern

1. Define struct with JSON tags
2. Use `c.BindJSON(&req)` for automatic parsing
3. Return 400 Bad Request if binding fails
4. Struct fields are type-safe

---

## Router Organization

### Grouping by Resource

```go
api := router.Group("/api")
{
    // Mailbox endpoints
    api.GET("/mailboxes", r.listMailboxes)
    api.GET("/mailboxes/export", r.exportMailboxes)

    // Stats endpoints
    api.GET("/stats", r.getStats)
    api.POST("/stats/refresh", r.refreshStats)

    // Crawl endpoints
    api.POST("/crawl/run", r.startCrawl)
    api.POST("/crawl/reprocess", r.reprocessMailboxes)
    api.GET("/crawl/status", r.getCrawlStatus)
    api.GET("/crawl/runs", r.listCrawlRuns)
    api.POST("/crawl/runs/:runId/cancel", r.cancelCrawlRun)

    // Source-specific crawl endpoints
    api.POST("/crawl/ipost1/run", r.startIPost1Crawl)
}
```

**Pattern**: Group related endpoints together with comments for readability.

---

## Error Messages

### User-Friendly vs Developer-Friendly

**User-facing errors** (API responses):
```go
c.JSON(http.StatusInternalServerError, gin.H{
    "error": "Failed to start crawl",
})
```

**Developer logs** (for debugging):
```go
log.Printf("Failed to start crawl for source %s: %v", source, err)
```

**Pattern**: Never expose internal error details to API consumers (security risk). Log detailed errors server-side.
