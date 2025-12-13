# Improve Hash Function for Deduplication

## Summary

The current `hashMailbox` function uses hex encoding of the concatenated key rather than a proper cryptographic hash. While functionally correct for deduplication, it's misleading and could cause issues with very long addresses.

## Current Code

```go
// discovery.go:211-223
func hashMailbox(mb model.Mailbox) string {
    key := fmt.Sprintf("%s|%s|%s|%s|%s",
        mb.Name,
        mb.AddressRaw.Street,
        mb.AddressRaw.City,
        mb.AddressRaw.State,
        mb.AddressRaw.Zip,
    )
    return fmt.Sprintf("%x", []byte(key))  // This is hex encoding, NOT hashing
}
```

## Problems

1. **Misleading name**: Function is called `hashMailbox` but doesn't actually hash
2. **Long output**: For a typical address, output is 100+ characters (2x input length)
3. **No collision resistance**: Unlike a hash, collisions are impossible but output is verbose
4. **Inconsistent with ATMB**: If ATMB uses a different hashing approach, could cause issues

## Proposed Solutions

### Option 1: Use Proper SHA256 Hash (Recommended)

```go
import "crypto/sha256"

func hashMailbox(mb model.Mailbox) string {
    key := fmt.Sprintf("%s|%s|%s|%s|%s",
        mb.Name,
        mb.AddressRaw.Street,
        mb.AddressRaw.City,
        mb.AddressRaw.State,
        mb.AddressRaw.Zip,
    )
    hash := sha256.Sum256([]byte(key))
    return fmt.Sprintf("%x", hash[:16])  // First 16 bytes = 32 hex chars
}
```

### Option 2: Rename to Reflect Actual Behavior

```go
// mailboxKey generates a unique identifier for deduplication.
// NOT a cryptographic hash - just a deterministic key from address fields.
func mailboxKey(mb model.Mailbox) string {
    return fmt.Sprintf("%s|%s|%s|%s|%s",
        mb.Name,
        mb.AddressRaw.Street,
        mb.AddressRaw.City,
        mb.AddressRaw.State,
        mb.AddressRaw.Zip,
    )
}
```

### Option 3: Use Existing Utility (if available)

Check if there's an existing `util.HashMailboxKey` function (mentioned in comment) and use it for consistency:

```go
// discovery.go:221-222 comment suggests this exists
mb.DataHash = util.HashMailboxKey(mb)  // Use shared implementation
```

## Impact Assessment

- **Database migration**: May need to rehash existing iPost1 records
- **ATMB consistency**: Should verify ATMB uses same hashing approach
- **Performance**: SHA256 is fast, negligible impact

## Technical Details

**Affected File**: [internal/business/crawler/ipost1/discovery.go:210-223](../../apps/api/internal/business/crawler/ipost1/discovery.go#L210-L223)

## Acceptance Criteria

- [ ] Hash function uses proper cryptographic hash or is renamed appropriately
- [ ] Hash output is consistent length (e.g., 32-64 chars)
- [ ] Existing deduplication logic continues to work
- [ ] ATMB and iPost1 use consistent hashing approach
- [ ] Migration plan for existing records (if needed)

## Labels

- `enhancement`
- `tech-debt`
- `backend`

## Priority

Low - Current implementation works correctly, this is a code quality improvement.
