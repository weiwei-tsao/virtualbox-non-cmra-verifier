# Testing Guidelines

## Test File Naming

### Convention

```
package_file.go       → package_file_test.go
parser.go            → parser_test.go
validation.go        → validation_test.go
```

**Pattern**: Test file has same name as source file with `_test` suffix.

### Test Package Naming

```go
// Option 1: Same package (white-box testing)
package crawler

func TestInternalFunction(t *testing.T) {
    // Can access unexported functions
}

// Option 2: Separate package (black-box testing)
package crawler_test

import "github.com/.../internal/business/crawler"

func TestPublicAPI(t *testing.T) {
    // Only access exported functions
}
```

**Pattern**: Use same package for testing internal logic, separate package for testing public API.

---

## Test Data Location

### testdata Directory

```
internal/business/crawler/
├── parser.go
├── parser_test.go
└── testdata/
    ├── sample_page.html
    ├── atmb_location.html
    └── ipost1_response.json
```

**Pattern**: Store test fixtures in `testdata/` subdirectory (Go convention, excluded from builds).

### Loading Test Data

```go
func TestParser(t *testing.T) {
    htmlBytes, err := os.ReadFile("testdata/sample_page.html")
    if err != nil {
        t.Fatalf("Failed to load test data: %v", err)
    }

    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
    // ... test parsing ...
}
```

---

## Running Tests

### All Tests

```bash
# From apps/api/
go test ./...
```

### Specific Package

```bash
# Verbose output
go test ./internal/business/crawler -v

# Specific test function
go test ./internal/business/crawler -run TestParseATMBLocation -v

# With coverage
go test ./internal/business/crawler -cover
```

### Test Flags

| Flag | Purpose | Example |
|------|---------|---------|
| `-v` | Verbose output | `go test -v` |
| `-run` | Run specific test | `go test -run TestParser` |
| `-cover` | Show coverage | `go test -cover` |
| `-short` | Skip long tests | `go test -short` |
| `-timeout` | Set timeout | `go test -timeout 30s` |

---

## Mock Patterns

### Smarty API Mock

Use environment variable to enable mock mode:

```bash
# .env.local
SMARTY_MOCK=true
```

**Implementation**:

```go
// In smarty/client.go
func (c *Client) BatchValidate(ctx context.Context, addrs []AddressRaw) ([]Result, error) {
    if c.config.Mock {
        return c.mockValidate(addrs), nil
    }

    return c.realAPICall(ctx, addrs)
}

func (c *Client) mockValidate(addrs []AddressRaw) []Result {
    results := make([]Result, len(addrs))
    for i := range addrs {
        results[i] = Result{
            CMRA: "Y",
            RDI:  "Commercial",
            // ... other fields ...
        }
    }
    return results
}
```

**Pattern**: Mock returns deterministic data for testing without API calls/quota usage.

---

## Table-Driven Tests

### Pattern

```go
func TestParseATMBLocation(t *testing.T) {
    tests := []struct {
        name     string
        html     string
        expected model.Mailbox
        wantErr  bool
    }{
        {
            name: "valid location",
            html: `<html>...</html>`,
            expected: model.Mailbox{
                Name: "ABC Mailbox",
                AddressRaw: model.AddressRaw{
                    Street: "123 Main St",
                    City:   "Dover",
                    State:  "DE",
                },
            },
            wantErr: false,
        },
        {
            name:    "missing name",
            html:    `<html><div class="address">123 Main</div></html>`,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            doc, _ := goquery.NewDocumentFromReader(strings.NewReader(tt.html))

            result, err := parseATMBLocation(doc)

            if tt.wantErr {
                if err == nil {
                    t.Errorf("expected error, got nil")
                }
                return
            }

            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }

            if result.Name != tt.expected.Name {
                t.Errorf("name = %v, want %v", result.Name, tt.expected.Name)
            }
            // ... more assertions ...
        })
    }
}
```

**Pattern**: Test multiple cases in one function, each case gets its own subtest with `t.Run()`.

---

## Parser Testing Patterns

### Testing HTML Parsing

```go
func TestParseATMBLocation(t *testing.T) {
    // Load fixture
    htmlBytes, err := os.ReadFile("testdata/atmb_location.html")
    if err != nil {
        t.Fatalf("Failed to load test fixture: %v", err)
    }

    // Parse
    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }

    // Test parser
    result, err := parseATMBLocation(doc, "https://example.com/location/123")
    if err != nil {
        t.Fatalf("parseATMBLocation failed: %v", err)
    }

    // Assertions
    if result.Name == "" {
        t.Error("Name should not be empty")
    }

    if result.AddressRaw.State != "CA" {
        t.Errorf("State = %v, want CA", result.AddressRaw.State)
    }

    if result.Price <= 0 {
        t.Errorf("Price = %v, should be positive", result.Price)
    }
}
```

**Pattern**: Use real HTML fixtures from actual websites, test all extracted fields.

---

## Integration vs Unit Tests

### Unit Test

Tests single function in isolation:

```go
func TestComputeDataHash(t *testing.T) {
    hash1 := computeDataHash("ABC Store", "123 Main", "Dover", "DE", "19901")
    hash2 := computeDataHash("ABC Store", "123 Main", "Dover", "DE", "19901")
    hash3 := computeDataHash("XYZ Store", "123 Main", "Dover", "DE", "19901")

    if hash1 != hash2 {
        t.Error("Same input should produce same hash")
    }

    if hash1 == hash3 {
        t.Error("Different input should produce different hash")
    }
}
```

### Integration Test

Tests multiple components together:

```go
func TestCrawlerFullWorkflow(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Setup
    ctx := context.Background()
    firestoreClient := setupTestFirestore(t)
    defer firestoreClient.Close()

    smartyClient := &smarty.Client{Config: smarty.Config{Mock: true}}
    repo := repository.NewMailboxRepository(firestoreClient)

    // Run crawler
    scraper := crawler.NewScraper(repo, smartyClient)
    err := scraper.Run(ctx, []string{"https://example.com/test"})

    if err != nil {
        t.Fatalf("Scraper failed: %v", err)
    }

    // Verify results in database
    results, err := repo.FetchAllMap(ctx)
    if err != nil {
        t.Fatalf("Failed to fetch results: %v", err)
    }

    if len(results) == 0 {
        t.Error("Expected at least one result")
    }
}
```

**Pattern**: Mark integration tests with `testing.Short()` check, skip with `go test -short`.

---

## Test Assertions

### Error Checking

```go
// Basic error check
if err != nil {
    t.Errorf("unexpected error: %v", err)
}

// Expected error
if err == nil {
    t.Error("expected error, got nil")
}

// Specific error
if !errors.Is(err, repository.ErrNotFound) {
    t.Errorf("expected ErrNotFound, got %v", err)
}
```

### Value Comparisons

```go
// Equality
if got != want {
    t.Errorf("got %v, want %v", got, want)
}

// Deep equality (for structs/slices)
if !reflect.DeepEqual(got, want) {
    t.Errorf("got %+v, want %+v", got, want)
}

// String contains
if !strings.Contains(result, "expected substring") {
    t.Errorf("result %q does not contain expected substring", result)
}
```

---

## Test Helpers

### Setup/Teardown Pattern

```go
func setupTestDB(t *testing.T) *firestore.Client {
    t.Helper()  // Marks this as helper function

    ctx := context.Background()
    client, err := firestore.NewClient(ctx, "test-project")
    if err != nil {
        t.Fatalf("Failed to create test client: %v", err)
    }

    // Cleanup
    t.Cleanup(func() {
        client.Close()
    })

    return client
}

func TestWithDatabase(t *testing.T) {
    db := setupTestDB(t)  // Automatic cleanup
    // ... use db ...
}
```

**Pattern**: Use `t.Helper()` in setup functions and `t.Cleanup()` for teardown.

---

## Test Coverage

### Running with Coverage

```bash
# Generate coverage report
go test ./... -coverprofile=coverage.out

# View coverage in browser
go tool cover -html=coverage.out

# Coverage by function
go tool cover -func=coverage.out
```

### Coverage Expectations

| Package Type | Target Coverage |
|-------------|----------------|
| Core business logic (crawler, parser) | 80%+ |
| Repository layer | 70%+ |
| HTTP handlers | 60%+ |
| Utilities | 90%+ |

**Pattern**: Focus on testing critical paths (parsers, validation) more than HTTP boilerplate.

---

## Common Test Patterns

### Testing Time-Dependent Code

```go
// Use fixed time for deterministic tests
func TestTimestamp(t *testing.T) {
    fixedTime := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)

    // Option 1: Pass time as parameter
    result := generateFilename(fixedTime)
    expected := "mailbox-20260313T120000Z.csv"

    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

### Testing Context Cancellation

```go
func TestContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()  // Cancel immediately

    err := longRunningOperation(ctx)

    if !errors.Is(err, context.Canceled) {
        t.Errorf("expected context.Canceled, got %v", err)
    }
}
```

---

## What to Test

### ✅ Should Test

1. **Parsers**: All HTML parsing logic
2. **Validation**: Address validation, deduplication logic
3. **Data transformations**: Hash computation, filename generation
4. **Error handling**: Edge cases, invalid input
5. **Business logic**: Batch processing, mark-and-sweep

### ❌ Don't Need to Test

1. **Third-party libraries**: goquery, chromedp (already tested)
2. **Framework code**: Gin handlers (integration tests instead)
3. **Simple getters/setters**: No logic to test
4. **Configuration parsing**: Use integration tests

---

## Pre-Commit Testing

Always run before committing:

```bash
# Full test suite
go test ./...

# With coverage
go test ./... -cover

# Verbose for debugging failures
go test ./... -v
```

**Pattern**: Tests should be fast (<30s total) to encourage frequent running.
