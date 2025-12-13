# Add Logging for Background Update Failures

## Summary

The auto-timeout detection in `ListRuns` updates stale jobs in a background goroutine but silently ignores any errors. This makes debugging production issues difficult.

## Current Code

```go
// run_repo.go:92-94
go func(r *RunRepository, run model.CrawlRun) {
    _ = r.UpdateRun(context.Background(), run)  // Error silently ignored
}(r, run)
```

## Problem

If the Firestore update fails (network issues, permission errors, etc.):
1. The error is completely ignored
2. No logs are generated
3. The job remains in "running" status in the database
4. Users see inconsistent status (UI shows timeout, DB shows running)
5. Debugging becomes extremely difficult

## Proposed Solution

### Option 1: Add Structured Logging

```go
go func(r *RunRepository, run model.CrawlRun) {
    if err := r.UpdateRun(context.Background(), run); err != nil {
        log.Printf("ERROR: failed to auto-update stale run %s to timeout: %v", run.RunID, err)
    } else {
        log.Printf("INFO: auto-marked run %s as timeout (started %v)", run.RunID, run.StartedAt)
    }
}(r, run)
```

### Option 2: Use Structured Logger (if available)

```go
go func(r *RunRepository, run model.CrawlRun) {
    if err := r.UpdateRun(context.Background(), run); err != nil {
        r.logger.Error("failed to auto-update stale run",
            "runID", run.RunID,
            "error", err,
            "startedAt", run.StartedAt,
        )
    }
}(r, run)
```

### Option 3: Retry with Backoff

For critical updates, implement a simple retry mechanism:

```go
go func(r *RunRepository, run model.CrawlRun) {
    var err error
    for attempt := 0; attempt < 3; attempt++ {
        if err = r.UpdateRun(context.Background(), run); err == nil {
            return
        }
        time.Sleep(time.Duration(attempt+1) * time.Second)
    }
    log.Printf("ERROR: failed to update stale run %s after 3 attempts: %v", run.RunID, err)
}(r, run)
```

## Additional Improvements

1. **Add metrics/telemetry** for auto-timeout events
2. **Create alerts** for high failure rates
3. **Consider synchronous update** with timeout context instead of fire-and-forget

## Technical Details

**Affected File**: [internal/repository/run_repo.go:87-95](../../apps/api/internal/repository/run_repo.go#L87-L95)

## Acceptance Criteria

- [ ] Failed background updates are logged with error details
- [ ] Successful auto-timeout updates are logged (debug level)
- [ ] Log includes run ID and relevant context
- [ ] No performance impact on ListRuns response time

## Labels

- `enhancement`
- `observability`
- `backend`

## Priority

Low - Does not affect functionality, but improves operational visibility.
