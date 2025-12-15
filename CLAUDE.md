# Claude Code Rules for virtualbox-verifier

## Go Development Rules

### 1. Struct Field Changes
When modifying struct fields (especially in `pkg/model/`), always:
- Search for all usages: `Grep` for the struct type name across the codebase
- Update all test files that use the struct
- Run `go build ./...` and `go test ./...` to verify no compilation errors

Example: Changing `AuthID string` to `AuthIDs []string` requires updating all test files using `Config{}`.

### 2. Shared Types - No Duplication
- Use shared model types from `apps/api/pkg/model/` instead of defining local structs
- Before creating a new struct, check if one already exists: `Grep "type StructName struct"`
- Common shared types: `model.Mailbox`, `model.CrawlRun`, `model.AddressRaw`

### 3. Scripts Directory Structure
Each standalone script must be in its own subdirectory:
```
scripts/
├── script_name_a/
│   └── main.go
└── script_name_b/
    └── main.go
```
This prevents "redeclared in this block" errors when multiple `package main` files exist.

### 4. Pre-commit Verification
Before committing Go code changes, run:
```bash
go build ./...
go vet ./...
go test ./...
```

## Project Structure

```
apps/api/
├── cmd/              # Application entrypoints
├── internal/         # Private packages
│   └── platform/     # External service integrations (smarty, firestore)
├── pkg/model/        # Shared model types (Mailbox, CrawlRun, etc.)
└── scripts/          # Utility scripts (each in own subdirectory)
```
