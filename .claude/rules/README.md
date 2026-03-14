# Rules Organization

This directory contains topic-specific development rules for the virtualbox-verifier project.

## File Structure

| File | Focus | Lines | Purpose |
|------|-------|-------|---------|
| [`go-development.md`](./go-development.md) | Go patterns | ~200 | Struct changes, type safety, imports, error handling |
| [`crawler-architecture.md`](./crawler-architecture.md) | Crawler workflows | ~250 | Batch processing, parser versioning, deduplication, worker pools |
| [`api-design.md`](./api-design.md) | HTTP API | ~200 | RESTful naming, request/response patterns, CORS, streaming |
| [`frontend-components.md`](./frontend-components.md) | React patterns | ~200 | Toast system, badges, hooks, CSV export, TypeScript |
| [`database.md`](./database.md) | Firestore | ~250 | Repository pattern, batch operations, indexes, streaming queries |
| [`testing.md`](./testing.md) | Testing | ~150 | Test organization, mocks, table-driven tests, coverage |
| [`git-conventions.md`](./git-conventions.md) | Git commits | ~250 | Conventional Commits, commit message format, workflow |

## How to Use

### For General Setup and Commands

See [CLAUDE.md](../../CLAUDE.md) in the project root for:
- Development commands (build, test, run)
- Environment variables
- API endpoints reference
- Quick start guide

### For Topic-Specific Patterns

Reference these rules files when working on:
- **Backend Go code** → `go-development.md`, `crawler-architecture.md`
- **API endpoints** → `api-design.md`
- **Frontend components** → `frontend-components.md`
- **Database queries** → `database.md`
- **Writing tests** → `testing.md`
- **Git commits** → `git-conventions.md`

## Organization Principles

1. **Small & Focused**: Each file ~150-250 lines, single responsibility
2. **Topic-Specific**: Clear boundaries between concerns
3. **Actionable**: Rules with code examples, not just descriptions
4. **No Duplication**: Each pattern documented once in the most relevant file
5. **Searchable**: Clear headings and consistent structure

## Quick Reference

### Most Common Patterns

- **Batch processing workflow** → `crawler-architecture.md` § Batch Processing Workflow
- **Parser version updates** → `crawler-architecture.md` § Parser Versioning System
- **API endpoint design** → `api-design.md` § RESTful Naming Conventions
- **Toast notifications** → `frontend-components.md` § Toast Notification System
- **Firestore batch operations** → `database.md` § Batch Operations
- **Table-driven tests** → `testing.md` § Table-Driven Tests

### Common Gotchas

- **Forgetting to increment parser version** → `crawler-architecture.md` § Common Pitfalls
- **Not using batch Smarty validation** → `crawler-architecture.md` § Batch Processing Workflow
- **Type duplication** → `go-development.md` § No Type Duplication
- **Missing composite indexes** → `database.md` § Required Composite Indexes
- **Checking `parsed.CMRA` after parsing** → `crawler-architecture.md` § Data Deduplication

## Maintenance

When adding new patterns:
1. Determine which file best fits the topic
2. Add to the most specific category (e.g., crawler pattern → `crawler-architecture.md`, not `go-development.md`)
3. Keep examples concise and actionable
4. Update this README if adding new files
