# Crawler Architecture Patterns

## Batch Processing Workflow

All three crawlers (ATMB, iPost1, Reprocess) follow the same pattern:

```
1. Collect items in buffer (max 20)
2. When buffer full:
   a. Batch validate with Smarty API (up to 100 addresses/request)
   b. BatchUpsert to Firestore (all 20 records in one write)
3. Continue until complete
4. Mark-and-sweep: Set active=false for missing records
```

### Critical Rule

**Never call Smarty API individually per address**. Always use batch validation.

❌ **Wrong** - Exceeds quota:
```go
for _, addr := range addresses {
    result, err := smarty.Validate(addr)  // 2000 API calls
}
```

✅ **Correct** - Batch validation:
```go
results, err := smarty.BatchValidate(addresses)  // 20 API calls (100 per batch)
```

### Why Batching Matters

| Approach | 2000 Addresses | API Calls |
|----------|----------------|-----------|
| Individual | 2000 | 2000 |
| Batch (100) | 2000 | 20 |

**Savings**: 99% reduction in API usage

---

## Parser Versioning System

Records store `parserVersion` and `rawHTML` to enable reprocessing without re-fetching.

### Workflow

1. **Parser bug found** → Increment `CurrentParserVersion` in `scraper.go`:
   ```go
   const CurrentParserVersion = "v1.2"  // Was v1.1
   ```

2. **Deploy code** to production

3. **Trigger reprocessing**:
   ```bash
   POST /api/crawl/reprocess
   {
     "outdatedOnly": true
   }
   ```

4. **System re-parses** all outdated records from stored HTML (~2 min vs ~30 min re-crawl)

### When to Use Reprocessing

✅ **Use for**:
- Parser bug fixes
- Parser improvements
- Testing parser changes
- Adding new extracted fields

❌ **Don't use for**:
- New data sources (need initial crawl)
- HTML structure changes (requires re-fetch)
- Changed URLs (need re-crawl)

### Implementation Pattern

```go
// In scraper.go
const CurrentParserVersion = "v1.2"

// When saving mailbox
mailbox := model.Mailbox{
    // ... fields ...
    RawHTML:        htmlContent,
    ParserVersion:  CurrentParserVersion,
    LastParsedAt:   time.Now(),
}
```

---

## Metadata-Only Fetching

`FetchAllMetadata()` loads only essential fields instead of full documents.

### Performance Impact

| Method | Data Size | Fields Loaded |
|--------|-----------|---------------|
| `FetchAllMap()` | ~200MB for 2000 docs | All fields (including rawHTML) |
| `FetchAllMetadata()` | ~2MB for 2000 docs | `id`, `link`, `dataHash`, `cmra`, `rdi` |

**Reduction**: 90% smaller, enables 30+ crawls/day within free tier (vs 3 before)

### Usage Pattern

```go
// At crawl start - deduplication check
metadata, err := repo.FetchAllMetadata(ctx)

// During reprocessing - need full documents
fullDocs, err := repo.FetchAllMap(ctx)
```

### Implementation

```go
// In mailbox_repo.go
func (r *MailboxRepository) FetchAllMetadata(ctx context.Context) (map[string]MetadataOnly, error) {
    iter := r.client.Collection("mailboxes").
        Select("id", "link", "dataHash", "cmra", "rdi").  // Only these fields
        Documents(ctx)

    // ... process ...
}
```

---

## Data Deduplication

Uses `dataHash` (MD5 of name + address) to skip unchanged records.

### Deduplication Logic

```go
existingHash := metadata[id].dataHash
newHash := computeHash(parsed.Name, parsed.Address)
existingCMRA := metadata[id].cmra

if existingHash == newHash && existingCMRA != "" {
    stats.skipped++
    continue  // Skip - already validated, no changes
}
```

### Critical Gotcha

**Cannot rely on `parsed.CMRA` from HTML parsing** - it's always empty after parsing. Must check stored values from database.

❌ **Wrong**:
```go
if parsed.CMRA == "Y" {  // Always false - CMRA not in HTML
    skip()
}
```

✅ **Correct**:
```go
existingCMRA := metadata[id].cmra  // From database
if existingHash == newHash && existingCMRA != "" {
    skip()
}
```

### Hash Computation

```go
import "crypto/md5"

func computeDataHash(name, street, city, state, zip string) string {
    raw := name + street + city + state + zip
    sum := md5.Sum([]byte(raw))
    return fmt.Sprintf("%x", sum)
}
```

---

## Worker Pool Pattern

ATMB crawler uses concurrent worker pool for parallel processing.

### Architecture

```
orchestrator.go:
- N concurrent workers (CRAWLER_CONCURRENCY env var)
- Each worker: fetch → parse → collect in buffer
- Shared channel for results aggregation
- Coordinator: batch validate → write to DB
```

### Configuration

| Environment | Workers | Reason |
|-------------|---------|--------|
| Local dev | 10+ | High CPU/memory available |
| Render free tier | 5 | CPU/memory limited |
| Production (paid) | 10-20 | Based on instance size |

### Implementation Pattern

```go
// In orchestrator.go
func (o *Orchestrator) Run(ctx context.Context, urls []string) error {
    concurrency := o.config.CrawlerConcurrency  // Default: 5

    workChan := make(chan string, len(urls))
    resultsChan := make(chan Result, concurrency)

    // Start workers
    for i := 0; i < concurrency; i++ {
        go o.worker(ctx, workChan, resultsChan)
    }

    // Feed work
    for _, url := range urls {
        workChan <- url
    }
    close(workChan)

    // Collect results
    // ...
}
```

---

## Multi-Credential Load Balancing

Smarty client supports multiple credentials for quota distribution.

### Configuration

```bash
# .env.local
SMARTY_AUTH_ID=id1,id2,id3
SMARTY_AUTH_TOKEN=token1,token2,token3
```

### Load Balancing Strategy

- **Round-robin**: Rotate through credentials
- **Circuit breaker**: Skip credential on 429 (rate limit) or 402 (quota exceeded)
- **Recovery**: Retry failed credentials after cooldown period

### Implementation

```go
// In smarty/client.go
type Client struct {
    credentials []Credential
    currentIdx  int
    failedCreds map[int]time.Time  // Circuit breaker
}

func (c *Client) BatchValidate(ctx context.Context, addrs []AddressRaw) ([]Result, error) {
    cred := c.nextCredential()  // Round-robin with circuit breaker

    resp, err := c.callAPI(cred, addrs)
    if err != nil {
        if isRateLimitError(err) {
            c.markFailed(c.currentIdx)  // Circuit breaker
            return c.BatchValidate(ctx, addrs)  // Retry with next credential
        }
        return nil, err
    }

    return resp, nil
}
```

---

## ATMB vs iPost1 Differences

| Aspect | ATMB | iPost1 |
|--------|------|--------|
| **Scraping** | goquery (static HTML) | chromedp (browser automation) |
| **Discovery** | Scrape index page | AJAX endpoints |
| **Parser** | `crawler/parser.go` | `crawler/ipost1/parser.go` |
| **Speed** | Fast (~5 min) | Slower (~15 min) |
| **Cloudflare** | No protection | Requires bypass |
| **Complexity** | Simple HTTP requests | Headless browser required |

### ATMB Pattern (goquery)

```go
// Simple HTTP request
resp, err := http.Get(url)
doc, err := goquery.NewDocumentFromReader(resp.Body)

// Parse with CSS selectors
name := doc.Find(".location-name").Text()
```

### iPost1 Pattern (chromedp)

```go
// Must establish browser session first
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()

// Navigate to establish session
chromedp.Run(ctx, chromedp.Navigate("https://ipost1.com"))

// Then make AJAX calls
var htmlResponse string
chromedp.Run(ctx,
    chromedp.Navigate("https://ipost1.com/locations_ajax.php?action=get_states_list"),
    chromedp.InnerHTML("body", &htmlResponse),
)
```

### Critical for iPost1

**Must establish browser session before AJAX calls**, otherwise Cloudflare blocks the request.

---

## Mark-and-Sweep Deletion

Detects and soft-deletes records that no longer exist at the source.

### Workflow

```
1. Track all seen IDs during crawl
   seenIDs := map[string]bool{}

2. Query existing records for this source
   existing := repo.FetchAllBySource(ctx, "ATMB")

3. Find unseen records
   for id := range existing {
       if !seenIDs[id] {
           toDelete = append(toDelete, id)
       }
   }

4. Soft delete (set active=false)
   for _, id := range toDelete {
       repo.UpdateActive(ctx, id, false)
   }
```

### Why Soft Delete?

- Preserves historical data
- Enables recovery if source temporarily unavailable
- Supports analytics (trend analysis)
- Allows audit trail

### Implementation

```go
// In scraper.go
func (s *Scraper) markAndSweep(ctx context.Context, seenIDs map[string]bool) error {
    existing, err := s.repo.FetchAllMetadata(ctx)  // Only this source
    if err != nil {
        return err
    }

    var toDeactivate []string
    for id := range existing {
        if !seenIDs[id] {
            toDeactivate = append(toDeactivate, id)
        }
    }

    return s.repo.BulkSetActive(ctx, toDeactivate, false)
}
```

---

## Incremental Batch Writes

Write every 20 items instead of accumulating all records in memory.

### Benefits

| Approach | Memory Usage | Resume on Failure | Data Loss on Crash |
|----------|--------------|-------------------|-------------------|
| All at end | High (200MB+) | No | All records |
| Incremental (20) | Low (~2MB) | Yes | Last 20 records |

### Pattern

```go
buffer := []model.Mailbox{}
const batchSize = 20

for _, url := range urls {
    mailbox := processURL(url)
    buffer = append(buffer, mailbox)

    if len(buffer) >= batchSize {
        // Validate batch
        validated := validator.BatchValidate(ctx, buffer)

        // Write batch
        repo.BatchUpsert(ctx, validated)

        // Clear buffer
        buffer = []model.Mailbox{}
    }
}

// Write remaining
if len(buffer) > 0 {
    validated := validator.BatchValidate(ctx, buffer)
    repo.BatchUpsert(ctx, validated)
}
```

---

## Common Pitfalls

1. **Forgetting to increment parser version** → Reprocess won't pick up changes
2. **Using individual Smarty calls** → Exceeds quota
3. **Checking `parsed.CMRA` after HTML parse** → Always empty, check DB values
4. **Setting `CRAWLER_CONCURRENCY` too high on Render** → OOM errors (limit: 5)
5. **Not establishing chromedp session for iPost1** → Cloudflare blocks
6. **Loading full documents for deduplication** → Exceeds Firestore quota
