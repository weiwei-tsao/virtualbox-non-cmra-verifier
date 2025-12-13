# Implement Proper Job Cancellation

## Summary

The current "Cancel" functionality only marks the job as cancelled in the database but does not actually stop the running goroutine. The crawler continues running in the background until it finishes or times out naturally.

## Current Behavior

1. User clicks "Cancel" button on a running job
2. Backend marks the job status as `cancelled` in Firestore
3. **Problem**: The crawler goroutine continues running, consuming resources
4. The crawler only stops when it naturally completes or hits the 45-minute timeout

## Expected Behavior

1. User clicks "Cancel" button
2. Backend sends cancellation signal to the running goroutine
3. Goroutine receives signal and gracefully stops
4. Job is marked as `cancelled` with proper cleanup

## Proposed Solution

### 1. Use Context Cancellation

Store a cancel function for each running job and invoke it on cancel request.

```go
// In crawler service
type runningJob struct {
    cancel context.CancelFunc
    runID  string
}

var activeJobs = sync.Map{} // map[runID]*runningJob

func StartCrawl(ctx context.Context, runID string) {
    ctx, cancel := context.WithCancel(ctx)
    activeJobs.Store(runID, &runningJob{cancel: cancel, runID: runID})
    defer activeJobs.Delete(runID)

    // Pass ctx to crawler - it already checks ctx.Done()
    DiscoverAll(ctx, logFn)
}

func CancelCrawl(runID string) error {
    if job, ok := activeJobs.Load(runID); ok {
        job.(*runningJob).cancel()
        return nil
    }
    return errors.New("job not found or already completed")
}
```

### 2. Update Discovery Loop

The discovery loop already checks for context cancellation:

```go
// discovery.go:39-43 - Already implemented!
select {
case <-ctx.Done():
    return allMailboxes, ctx.Err()
default:
}
```

### 3. Add Cancel Endpoint Handler

Update the HTTP handler to call the cancel function:

```go
func (s *Service) HandleCancelRun(w http.ResponseWriter, r *http.Request) {
    runID := chi.URLParam(r, "runId")

    // Cancel the context
    if err := s.CancelCrawl(runID); err != nil {
        // Job may have already finished - still update DB
    }

    // Update database status
    if err := s.runRepo.CancelRun(r.Context(), runID); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
```

## Technical Details

**Affected Files**:
- [internal/business/crawler/service.go](../../apps/api/internal/business/crawler/service.go)
- [internal/business/crawler/ipost1/discovery.go](../../apps/api/internal/business/crawler/ipost1/discovery.go)
- [internal/repository/run_repo.go](../../apps/api/internal/repository/run_repo.go)

## Acceptance Criteria

- [ ] Cancel button actually stops the running crawler
- [ ] Resources (chromedp browser) are properly cleaned up
- [ ] Partial results are preserved before cancellation
- [ ] Job status correctly reflects cancellation reason
- [ ] No orphaned goroutines after cancellation

## Labels

- `enhancement`
- `crawler`
- `backend`

## Priority

Medium - Current workaround (timeout) exists, but proper cancellation improves UX and resource management.
