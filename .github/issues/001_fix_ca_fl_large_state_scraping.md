# Fix CA/FL Large State Scraping Issues

## Summary

The iPost1 crawler currently fails for California (CA) and Florida (FL), the two largest states with the most mailbox locations. This results in a 96.2% success rate (51/53 states) but misses significant data.

## Problem

Large states like CA and FL have many more mailbox locations than other states. The current implementation likely times out or encounters rate limiting when processing these states.

## Proposed Solutions

### Option 1: Pagination Support
- Investigate if the iPost1 API supports pagination for location results
- Implement pagination handling to fetch locations in smaller batches

### Option 2: Increased Timeouts
- Increase the per-state timeout for large states
- Add retry logic with exponential backoff

### Option 3: City-Level Queries
- For large states, break down queries by city or region
- Aggregate results from multiple smaller queries

### Option 4: Parallel Processing
- Process multiple states concurrently (with rate limiting)
- Use goroutine pools to manage parallelism

## Technical Details

**Current Implementation**: [discovery.go](../../apps/api/internal/business/crawler/ipost1/discovery.go)

**Affected Code**:
```go
// discovery.go:49-56
response, err := client.GetLocationsByState(state.ID)
if err != nil {
    if logFn != nil {
        logFn(fmt.Sprintf("error fetching locations for %s: %v", state.Name, err))
    }
    continue  // Silently skips failed states
}
```

## Acceptance Criteria

- [ ] CA scraping succeeds with all locations captured
- [ ] FL scraping succeeds with all locations captured
- [ ] Overall success rate reaches 100% (53/53 states)
- [ ] No significant increase in total crawl time
- [ ] Error handling improved for large state failures

## Labels

- `enhancement`
- `crawler`
- `ipost1`

## Priority

Medium - Current implementation works for most states, but CA/FL represent significant data gaps.
