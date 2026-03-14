# Git Commit Conventions

This project follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification.

## Commit Message Format

```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

### Examples

```
feat(parser): add support for new ATMB address format
fix(api): correct CSV filename generation for filtered exports
docs(readme): update deployment instructions
refactor(crawler): extract deduplication logic into separate function
test(parser): add test cases for edge cases
```

## Type

Must be one of the following:

| Type | Description | Example |
|------|-------------|---------|
| `feat` | New feature | `feat(api): add CSV export endpoint` |
| `fix` | Bug fix | `fix(crawler): handle timeout in worker pool` |
| `docs` | Documentation only | `docs(architecture): update batch processing diagram` |
| `style` | Code style (formatting, missing semi-colons) | `style(frontend): fix lint errors in Toast component` |
| `refactor` | Code change that neither fixes bug nor adds feature | `refactor(repo): simplify query builder logic` |
| `perf` | Performance improvement | `perf(firestore): use metadata-only fetching` |
| `test` | Adding or updating tests | `test(validation): add batch validation tests` |
| `build` | Build system or dependencies | `build(deps): upgrade Go to 1.25.3` |
| `ci` | CI/CD changes | `ci(github): add automated testing workflow` |
| `chore` | Other changes (maintenance) | `chore(scripts): update batch validate script` |
| `revert` | Revert previous commit | `revert: revert "feat(api): add new endpoint"` |

## Scope

The scope is **optional** but recommended. It provides context about what part of the codebase is affected.

### Common Scopes

**Backend**:
- `parser` - HTML parsing logic
- `crawler` - Crawling engine
- `api` - HTTP endpoints
- `validation` - Smarty API integration
- `repo` - Repository layer
- `firestore` - Firestore operations
- `smarty` - Smarty client

**Frontend**:
- `components` - React components
- `hooks` - Custom hooks
- `contexts` - React contexts
- `pages` - Page components
- `api-client` - API service layer

**General**:
- `config` - Configuration
- `scripts` - Utility scripts
- `deps` - Dependencies
- `deployment` - Deployment config

### Multiple Scopes

If changes affect multiple scopes, use the most relevant one or omit the scope:

```bash
# Good - specific scope
feat(parser): add iPost1 HTML parsing

# Good - no scope when too broad
feat: add complete iPost1 crawler support
```

## Subject

- Use **imperative, present tense**: "add" not "added" or "adds"
- Don't capitalize first letter
- No period (.) at the end
- Keep under 72 characters

### ✅ Good Examples

```
feat(api): add mailbox export endpoint
fix(crawler): handle empty address fields
docs: update CLAUDE.md with new patterns
refactor(validation): extract batch logic
```

### ❌ Bad Examples

```
feat(api): Added mailbox export endpoint    # Wrong tense
Fix(Crawler): Handle empty address fields   # Wrong capitalization
docs: Update CLAUDE.md with new patterns.   # Has period
refactor(validation): Extracted the batch validation logic into a separate function for better code reuse  # Too long
```

## Body

The body is **optional** but recommended for non-trivial changes. Use it to explain:
- **What** changed
- **Why** it was changed
- **Context** or background

### Format

- Separate from subject with a blank line
- Use imperative, present tense
- Wrap at 72 characters
- Can have multiple paragraphs

### Example

```
feat(parser): add support for new ATMB address format

ATMB recently changed their HTML structure to include suite numbers
in a separate field. This updates the parser to extract and combine
the street address and suite number.

The parser version is incremented to v1.3 to trigger reprocessing
of existing records.
```

## Footer

The footer is **optional**. Use it for:

### Breaking Changes

```
feat(api): change mailbox endpoint response format

BREAKING CHANGE: The response format has changed from a flat array
to a paginated object with {items, total, page} fields. Clients must
update their API integration.
```

### Issue References

```
fix(crawler): prevent duplicate records during concurrent crawls

Closes #123
Fixes #456
```

### Co-authored By

```
feat(ipost1): add chromedp-based crawler

Co-authored-by: Claude Sonnet 4.5 <noreply@anthropic.com>
```

## Complete Examples

### Simple Feature

```
feat(api): add CSV export filtering
```

### Bug Fix with Context

```
fix(crawler): prevent race condition in worker pool

The orchestrator was not properly synchronizing access to the
shared results channel, causing occasional panics during high
concurrency crawls.

This adds a mutex to protect the channel and ensures proper
cleanup on error.

Fixes #789
```

### Breaking Change

```
refactor(validation)!: change batch validation interface

BREAKING CHANGE: BatchValidate() now returns ([]Result, error)
instead of []Result. Callers must handle the error return value.

This change improves error handling and allows the client to
distinguish between partial failures and complete failures.
```

### Documentation Update

```
docs(readme): update Quick Start section

Add missing environment variable SMARTY_MOCK and clarify that
service-account.json is required for local development.
```

### Performance Improvement

```
perf(firestore): use metadata-only fetching for deduplication

Reduces memory usage from 200MB to 2MB for 2000 records by loading
only {id, link, dataHash, cmra, rdi} fields instead of full documents
including rawHTML.

This enables 30+ crawls per day within Firestore free tier limits.
```

## Commit Workflow

### 1. Stage Changes

```bash
# Stage specific files (preferred)
git add apps/api/internal/business/crawler/parser.go
git add apps/api/internal/business/crawler/parser_test.go

# Avoid staging everything unless necessary
git add -A  # Only if you're sure
```

### 2. Write Commit Message

```bash
# Method 1: Using editor
git commit

# Method 2: Inline (for simple commits)
git commit -m "feat(parser): add support for PO Box addresses"

# Method 3: With body
git commit -m "feat(parser): add support for PO Box addresses" \
           -m "ATMB locations now include PO Box addresses. Updated parser to extract box numbers."
```

### 3. Common Patterns

**Multiple related files changed**:
```bash
git add apps/api/pkg/model/mailbox.go
git add apps/api/internal/repository/mailbox_repo.go
git add apps/api/internal/business/crawler/scraper.go
git commit -m "refactor(model): rename AddressRaw.Street to AddressRaw.Street1"
```

**Test file with implementation**:
```bash
git add apps/api/internal/business/crawler/parser.go
git add apps/api/internal/business/crawler/parser_test.go
git commit -m "feat(parser): add suite number extraction"
```

## Pre-Commit Checklist

Before committing:

1. ✅ **Run tests**: `go test ./...`
2. ✅ **Run build**: `go build ./...`
3. ✅ **Run linter**: `go vet ./...`
4. ✅ **Check commit message format**: Follows conventional commits
5. ✅ **Review changes**: `git diff --staged`

## Tools and Automation

### Commit Message Template

Create `.gitmessage` in project root:

```
# <type>(<scope>): <subject>
#
# <body>
#
# <footer>

# Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
# Scope: parser, crawler, api, validation, repo, components, hooks, etc.
# Subject: imperative, present tense, lowercase, no period, < 72 chars
# Body: wrap at 72 chars, explain what and why
# Footer: BREAKING CHANGE, Closes #123, Co-authored-by
```

Configure Git to use it:
```bash
git config commit.template .gitmessage
```

### Git Hooks (Optional)

Create `.git/hooks/commit-msg` to validate format:

```bash
#!/bin/bash
commit_msg=$(cat "$1")
pattern="^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?: .{1,72}$"

if ! echo "$commit_msg" | head -1 | grep -qE "$pattern"; then
    echo "Error: Commit message does not follow Conventional Commits format"
    echo "Expected: <type>(<scope>): <subject>"
    echo "Example: feat(parser): add new field extraction"
    exit 1
fi
```

## Real-World Examples from This Project

Based on recent commits:

```
feat(csv-export): improve filename with user-friendly labels and local time
fix(toast): improve toast animations and filename handling
docs(claude): update CLAUDE.md with new component patterns
refactor(filename): update format with underscore dividers
test(backend): add backend tests for filename generation logic
```

## FAQ

### Q: When to use `feat` vs `fix`?
- `feat`: Adds new functionality (e.g., new endpoint, new component)
- `fix`: Fixes existing broken functionality (e.g., bug fix, error handling)

### Q: When to use `refactor` vs `chore`?
- `refactor`: Changes code structure without changing behavior
- `chore`: Maintenance tasks (update deps, scripts, config)

### Q: Should I include file paths in subject?
No, the scope should be sufficient. File paths go in the body if needed.

### Q: How to handle commits with Claude Code?
Claude Code automatically adds co-authored footer. Keep the conventional commit format in subject and body.

### Q: What about WIP commits?
Avoid committing work-in-progress. If necessary:
```bash
chore: WIP - implementing new parser logic
```
Then squash before merging.

## References

- [Conventional Commits Specification](https://www.conventionalcommits.org/en/v1.0.0/)
- [Semantic Versioning](https://semver.org/)
- [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/)
