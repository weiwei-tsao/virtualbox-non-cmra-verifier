# Firestore Database Patterns

## Repository Pattern Structure

All database operations go through repository layer (`internal/repository/`).

### Repository Interface Pattern

```go
// mailbox_repo.go
type MailboxRepository struct {
    client *firestore.Client
}

func NewMailboxRepository(client *firestore.Client) *MailboxRepository {
    return &MailboxRepository{client: client}
}

// Core operations
func (r *MailboxRepository) Get(ctx context.Context, id string) (*model.Mailbox, error)
func (r *MailboxRepository) List(ctx context.Context, query MailboxQuery) ([]model.Mailbox, int, error)
func (r *MailboxRepository) BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error
func (r *MailboxRepository) FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error)
func (r *MailboxRepository) FetchAllMetadata(ctx context.Context) (map[string]MetadataOnly, error)
```

**Pattern**: Repository encapsulates all Firestore operations, services never touch `firestore.Client` directly.

---

## Metadata-Only vs Full Document Fetching

### When to Use Each

| Method | Use Case | Fields Loaded | Data Size (2000 docs) |
|--------|----------|---------------|----------------------|
| `FetchAllMap()` | Reprocessing, full data needed | All (including `rawHTML`) | ~200MB |
| `FetchAllMetadata()` | Deduplication checks, crawl start | `id`, `link`, `dataHash`, `cmra`, `rdi` | ~2MB |

### Implementation

```go
// Full fetch
func (r *MailboxRepository) FetchAllMap(ctx context.Context) (map[string]model.Mailbox, error) {
    iter := r.client.Collection("mailboxes").Documents(ctx)
    // ... load all fields ...
}

// Metadata only (90% smaller)
func (r *MailboxRepository) FetchAllMetadata(ctx context.Context) (map[string]MetadataOnly, error) {
    iter := r.client.Collection("mailboxes").
        Select("id", "link", "dataHash", "cmra", "rdi").  // Only these fields
        Documents(ctx)
    // ... load selected fields ...
}
```

### Impact

**Firestore free tier**: 50K reads/day

| Approach | Reads per Crawl | Crawls per Day |
|----------|----------------|----------------|
| Full fetch | 2000 docs × full size | ~3 |
| Metadata only | 2000 docs × 90% smaller | ~30 |

**Pattern**: Always use `FetchAllMetadata()` at crawl start. Only use `FetchAllMap()` when you need `rawHTML` or other full fields.

---

## Batch Operations

### BatchUpsert Pattern

Write up to 500 documents in a single batch:

```go
func (r *MailboxRepository) BatchUpsert(ctx context.Context, mailboxes []model.Mailbox) error {
    const maxBatchSize = 500

    for i := 0; i < len(mailboxes); i += maxBatchSize {
        end := i + maxBatchSize
        if end > len(mailboxes) {
            end = len(mailboxes)
        }

        batch := r.client.Batch()
        for _, mb := range mailboxes[i:end] {
            ref := r.client.Collection("mailboxes").Doc(mb.ID)
            batch.Set(ref, mb)
        }

        if _, err := batch.Commit(ctx); err != nil {
            return err
        }
    }

    return nil
}
```

### Why Batching?

| Approach | 100 Documents | Writes | Latency |
|----------|--------------|--------|---------|
| Individual | 100 | 100 | ~10s |
| Batched (500) | 100 | 1 | ~100ms |

**Pattern**: Always batch writes. Firestore limit is 500 operations per batch.

---

## Query Patterns with Filters

### Building Filtered Queries

```go
type MailboxQuery struct {
    State    string
    CMRA     string
    RDI      string
    Source   string
    Active   *bool    // Pointer for optional boolean
    Page     int
    PageSize int
}

func (r *MailboxRepository) List(ctx context.Context, q MailboxQuery) ([]model.Mailbox, int, error) {
    collection := r.client.Collection("mailboxes")
    query := collection.Query

    // Apply filters
    if q.State != "" {
        query = query.Where("addressRaw.state", "==", q.State)
    }
    if q.CMRA != "" {
        query = query.Where("cmra", "==", q.CMRA)
    }
    if q.RDI != "" {
        query = query.Where("rdi", "==", q.RDI)
    }
    if q.Source != "" {
        query = query.Where("source", "==", q.Source)
    }
    if q.Active != nil {
        query = query.Where("active", "==", *q.Active)
    }

    // Count total (before pagination)
    total := r.count(ctx, query)

    // Apply pagination
    query = query.Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize)

    // Execute
    iter := query.Documents(ctx)
    // ... collect results ...

    return results, total, nil
}
```

### Optional Boolean Pattern

```go
Active *bool  // Can distinguish: true, false, or nil (not set)

// Usage
if q.Active != nil {
    query = query.Where("active", "==", *q.Active)
}
```

**Why pointer**: Distinguish between `false` and "not provided".

---

## Required Composite Indexes

Firestore requires composite indexes for queries with multiple filters.

### Create in Firebase Console

```
Collection: mailboxes
Fields:
1. (active, state)         - For active records by state
2. (active, rdi)           - For active records by RDI
3. (active, source)        - For active records by source
4. (crawlRunId)            - For filtering by crawl run
5. (parserVersion, source) - For reprocessing outdated records
```

### How to Create

1. Run query that needs index
2. Firestore error message includes direct link to create index
3. Click link, create index (takes ~2 minutes)

### Index Requirements

| Query Filters | Index Required |
|---------------|----------------|
| Single field | ❌ No (auto-indexed) |
| Multiple equality filters | ✅ Yes |
| Range + equality | ✅ Yes |
| `orderBy` + `where` | ✅ Yes |

---

## Streaming Queries for Large Datasets

For CSV exports, stream results instead of loading all in memory.

### Pattern

```go
func (r *MailboxRepository) StreamWithQuery(
    ctx context.Context,
    query MailboxQuery,
    callback func(model.Mailbox) error,
) error {
    iter := r.buildQuery(query).Documents(ctx)
    defer iter.Stop()

    for {
        doc, err := iter.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            return err
        }

        var mb model.Mailbox
        if err := doc.DataTo(&mb); err != nil {
            return err
        }

        // Process immediately, don't accumulate
        if err := callback(mb); err != nil {
            return err
        }
    }

    return nil
}
```

### Usage in HTTP Handler

```go
func (r *Router) exportMailboxes(c *gin.Context) {
    writer := csv.NewWriter(c.Writer)
    defer writer.Flush()

    err := r.mailboxes.StreamWithQuery(ctx, query, func(mb model.Mailbox) error {
        row := []string{mb.Name, mb.AddressRaw.Street, mb.AddressRaw.City}
        return writer.Write(row)
    })
}
```

### Why Streaming?

| Approach | 4000 Documents | Memory Usage | Time to First Row |
|----------|----------------|--------------|-------------------|
| Load all | 4000 | ~200MB | 5s (wait for all) |
| Streaming | 4000 | ~2MB | <100ms (immediate) |

---

## Soft Delete Pattern

Use `active` boolean field instead of deleting documents.

### Pattern

```go
// Mark as inactive (soft delete)
func (r *MailboxRepository) BulkSetActive(ctx context.Context, ids []string, active bool) error {
    batch := r.client.Batch()

    for _, id := range ids {
        ref := r.client.Collection("mailboxes").Doc(id)
        batch.Update(ref, []firestore.Update{
            {Path: "active", Value: active},
            {Path: "updatedAt", Value: time.Now()},
        })
    }

    _, err := batch.Commit(ctx)
    return err
}
```

### Why Soft Delete?

1. **Historical data**: Preserves records for analytics
2. **Recovery**: Can restore if source temporarily unavailable
3. **Audit trail**: Know when location was removed
4. **Debugging**: Investigate why location disappeared

### Querying Active Records

```go
// Default to active only
query := collection.Where("active", "==", true)

// Include all (active + inactive)
query := collection.Query  // No filter
```

---

## Document Structure Conventions

### Timestamps

Always use `time.Time` for timestamps, stored as Firestore Timestamp:

```go
type Mailbox struct {
    CreatedAt       time.Time `firestore:"createdAt"`
    UpdatedAt       time.Time `firestore:"updatedAt"`
    LastValidatedAt time.Time `firestore:"lastValidatedAt"`
    LastParsedAt    time.Time `firestore:"lastParsedAt"`
}
```

### Nested Objects

Use structs for related fields:

```go
type Mailbox struct {
    AddressRaw struct {
        Street string `firestore:"street"`
        City   string `firestore:"city"`
        State  string `firestore:"state"`
        Zip    string `firestore:"zip"`
    } `firestore:"addressRaw"`
}

// Query nested field
query.Where("addressRaw.state", "==", "CA")
```

### Arrays

Store arrays for multi-value fields:

```go
type CrawlRun struct {
    ErrorsSample []ErrorRecord `firestore:"errorsSample"`
}

// Limit array size to avoid large documents
const maxErrorSamples = 10
```

---

## Singleton Documents

For system-wide stats, use singleton document in `system` collection.

### Pattern

```go
// Single document at system/stats
func (r *StatsRepository) GetSystemStats(ctx context.Context) (*model.SystemStats, error) {
    doc, err := r.client.Collection("system").Doc("stats").Get(ctx)
    if err != nil {
        return nil, err
    }

    var stats model.SystemStats
    if err := doc.DataTo(&stats); err != nil {
        return nil, err
    }

    return &stats, nil
}

func (r *StatsRepository) SaveSystemStats(ctx context.Context, stats *model.SystemStats) error {
    stats.LastUpdated = time.Now()
    _, err := r.client.Collection("system").Doc("stats").Set(ctx, stats)
    return err
}
```

### Why Singleton?

**Efficiency**: Read 1 document instead of aggregating 4000+ documents on every dashboard load.

| Approach | Dashboard Load Time | Firestore Reads |
|----------|-------------------|-----------------|
| Aggregate on read | ~3s | 4000 |
| Singleton stats doc | ~50ms | 1 |

---

## Transaction Patterns

Use transactions for atomic updates:

```go
func (r *MailboxRepository) IncrementStats(ctx context.Context, runID string) error {
    return r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
        ref := r.client.Collection("crawl_runs").Doc(runID)

        doc, err := tx.Get(ref)
        if err != nil {
            return err
        }

        var run model.CrawlRun
        if err := doc.DataTo(&run); err != nil {
            return err
        }

        run.Stats.Found++

        return tx.Set(ref, run)
    })
}
```

**Pattern**: Use transactions when you need read-modify-write atomicity.

---

## Firestore Free Tier Limits

| Resource | Free Tier Limit | Notes |
|----------|----------------|-------|
| Reads | 50K/day | Use metadata-only fetches |
| Writes | 20K/day | Use batch operations |
| Deletes | 20K/day | Use soft delete to reduce |
| Storage | 1GB | Monitor `rawHTML` field size |

### Optimization Tips

1. **Reads**: Use `FetchAllMetadata()` instead of full fetch (90% savings)
2. **Writes**: Batch upserts (500 per batch)
3. **Deletes**: Soft delete pattern (reduces delete operations)
4. **Storage**: Consider compressing `rawHTML` if approaching limit

---

## Error Handling

### Document Not Found

```go
doc, err := r.client.Collection("mailboxes").Doc(id).Get(ctx)
if status.Code(err) == codes.NotFound {
    return nil, ErrNotFound  // Custom error
}
if err != nil {
    return nil, err
}
```

### Timeout Handling

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

results, err := r.repo.FetchAll(ctx)
if err == context.DeadlineExceeded {
    return ErrTimeout
}
```

**Pattern**: Always set context timeouts for long-running queries.
