# Go Development Rules

## Struct Field Changes

When modifying struct fields (especially in `pkg/model/`):

1. **Search for all usages**:
   ```bash
   Grep "model.StructName"
   ```

2. **Update all locations**:
   - Test files (`*_test.go`)
   - Scripts that use the struct
   - Internal packages referencing the type

3. **Verify builds**:
   ```bash
   go build ./...
   go vet ./...
   go test ./...
   ```

### Example

Changing `Config.AuthID string` to `Config.AuthIDs []string`:

**Files to update**:
- `internal/platform/config/config.go` - struct definition
- `internal/platform/smarty/client.go` - usage
- All test files constructing `Config{}`
- Any scripts using the config

**Common mistake**: Updating the struct but missing test file constructors → compilation errors.

---

## No Type Duplication

**Rule**: Always use shared types from `pkg/model/`. Before creating a struct, check if it exists.

### Quick Check

```bash
# Search for existing types
Grep "type Mailbox struct"
Grep "type CrawlRun struct"
Grep "type AddressRaw struct"
```

### Shared Types Reference

| Type | Location | Purpose |
|------|----------|---------|
| `Mailbox` | `pkg/model/mailbox.go` | Core record with HTML, parser version, hash |
| `CrawlRun` | `pkg/model/crawl_run.go` | Job tracking with status and stats |
| `AddressRaw` | `pkg/model/address.go` | User input address |
| `StandardizedAddress` | `pkg/model/address.go` | Smarty validated address |
| `Config` | `pkg/model/config.go` | Environment configuration |
| `SystemStats` | `pkg/model/stats.go` | Aggregate statistics |

### Common Violations

❌ **Wrong**:
```go
// In handler file
type Address struct {
    Street string
    City   string
}
```

✅ **Correct**:
```go
import "github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"

func handler() {
    addr := model.AddressRaw{
        Street: "123 Main",
        City:   "Dover",
    }
}
```

---

## Scripts Directory Structure

**Rule**: Each standalone script MUST be in its own subdirectory.

### Correct Structure

```
scripts/
├── batch_validate/
│   └── main.go          # package main
├── check_cmra_rdi/
│   └── main.go          # package main
├── find_missing_html/
│   └── main.go          # package main
└── test_smarty_api/
    └── main.go          # package main
```

### Why?

Prevents "main redeclared in this block" errors when multiple `package main` files exist in the same directory.

### Running Scripts

```bash
# From apps/api/
go run ./scripts/batch_validate
go run ./scripts/check_cmra_rdi
```

---

## Pre-Commit Verification

**Always run before committing Go code**:

```bash
# Build all packages
go build ./...

# Static analysis
go vet ./...

# Run all tests
go test ./...

# Optional: Run specific package tests
go test ./internal/business/crawler -v
```

### What Each Command Checks

| Command | Purpose |
|---------|---------|
| `go build ./...` | Verifies all packages compile |
| `go vet ./...` | Detects suspicious constructs |
| `go test ./...` | Runs unit and integration tests |

---

## Import Path Conventions

### Internal Packages

```go
import (
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/crawler"
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
)
```

### Import Organization

1. Standard library
2. External packages
3. Internal packages

```go
import (
    // Standard library
    "context"
    "fmt"
    "time"

    // External
    "github.com/gin-gonic/gin"
    "cloud.google.com/go/firestore"

    // Internal
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
    "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
)
```

---

## Error Handling Conventions

### Repository Layer

Return errors directly without wrapping:

```go
func (r *Repository) GetMailbox(ctx context.Context, id string) (*model.Mailbox, error) {
    doc, err := r.client.Collection("mailboxes").Doc(id).Get(ctx)
    if err != nil {
        return nil, err  // Return as-is
    }
    // ...
}
```

### Service Layer

Wrap errors with context:

```go
func (s *Service) ProcessMailbox(ctx context.Context, id string) error {
    mb, err := s.repo.GetMailbox(ctx, id)
    if err != nil {
        return fmt.Errorf("failed to get mailbox %s: %w", id, err)
    }
    // ...
}
```

### HTTP Handlers

Return user-friendly messages:

```go
func (r *Router) getMailbox(c *gin.Context) {
    mb, err := r.mailboxes.Get(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to retrieve mailbox",
        })
        return
    }
    c.JSON(http.StatusOK, mb)
}
```

---

## Context Usage

### Always Accept Context

All database operations and external API calls must accept `context.Context`:

```go
// ✅ Correct
func (r *Repository) FetchAll(ctx context.Context) ([]model.Mailbox, error)

// ❌ Wrong
func (r *Repository) FetchAll() ([]model.Mailbox, error)
```

### Pass Context Down

```go
func (s *Service) Start(ctx context.Context) error {
    // Pass to repository
    mailboxes, err := s.repo.FetchAllMetadata(ctx)
    if err != nil {
        return err
    }

    // Pass to validator
    results, err := s.validator.BatchValidate(ctx, addresses)
    // ...
}
```

---

## Common Patterns

### nil vs Empty Slice

Return `nil` for errors, empty slice for no results:

```go
func (r *Repository) List(ctx context.Context) ([]model.Mailbox, error) {
    results := []model.Mailbox{}  // Empty slice, not nil

    // ... populate results ...

    return results, nil  // Returns [] if no items
}
```

### Pointer vs Value Receivers

Use **pointer receivers** for:
- Methods that modify the receiver
- Large structs (avoid copying)
- Consistency (if one method uses pointer, all should)

```go
type Service struct {
    repo *repository.MailboxRepository
}

// ✅ Pointer receiver (modifies state, consistent)
func (s *Service) Start(ctx context.Context) error {
    // ...
}

// ❌ Don't mix
func (s Service) GetStats() Stats {  // Value receiver - inconsistent
    // ...
}
```
