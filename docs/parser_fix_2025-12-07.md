# HTML Parser Fix - December 7, 2025

## Problem Summary

The crawler was fetching data from AnytimeMailbox but **all parsed fields were incorrect**:
- **Name**: All showing "YOUR NAME" instead of actual store names
- **Address**: Most fields empty or incorrect (street, city, state, zip)
- **Price**: All showing 0.00 instead of actual prices
- **Working fields**: Only `link`, `cmra`, and `rdi` were correct (validated by Smarty API)

## Root Cause

The CSS selectors in [parser.go](apps/api/internal/business/crawler/parser.go) did not match the actual HTML structure of AnytimeMailbox detail pages.

### Old (Incorrect) Selectors:
```go
name := doc.Find(".mailbox-name").First().Text()          // ❌ Class doesn't exist
street := doc.Find(".address .street").First().Text()     // ❌ Class doesn't exist
city := doc.Find(".address .city").First().Text()         // ❌ Class doesn't exist
priceRaw := doc.Find(".price, .t-price").First().Text()   // ❌ Wrong selector
```

### Actual HTML Structure:
Inspected from: `https://www.anytimemailbox.com/s/chicago-73-w-monroe-street`

```html
<h1>Chicago - Monroe St</h1>

<div class="t-text">
  <div>73 W Monroe St</div>
  <div>5th Floor #MAILBOX</div>
  <div>Chicago, IL 60603</div>
  <div>United States</div>
</div>

<div class="theme-loc-detail-plans">
  <div class="t-plan">
    <div class="t-title">Bronze</div>
    <div class="t-price">US$ 19.99 / month</div>
  </div>
  ...
</div>
```

## Solution

### 1. Updated CSS Selectors ([parser.go:20-62](apps/api/internal/business/crawler/parser.go#L20-L62))

```go
// ✅ Name: Simple h1 tag
name := doc.Find("h1").First().Text()

// ✅ Address: Parse .t-text > div children
var addressLines []string
doc.Find(".t-text > div").Each(func(_ int, s *goquery.Selection) {
    txt := strings.TrimSpace(s.Text())
    if txt != "" && txt != "United States" {
        addressLines = append(addressLines, txt)
    }
})

// ✅ First line = street, Last line = "City, State Zip"
street = addressLines[0]
cityStateZip := addressLines[len(addressLines)-1]
// Parse: "Chicago, IL 60603" → city="Chicago", state="IL", zip="60603"

// ✅ Price: Get first plan price
priceRaw := doc.Find(".t-plan .t-price").First().Text()
// Parses: "US$ 19.99 / month" → 19.99
```

### 2. Updated Test Data

- **[testdata/sample_page.html](apps/api/internal/business/crawler/testdata/sample_page.html)**: Replaced with actual AnytimeMailbox HTML structure
- **[parser_test.go](apps/api/internal/business/crawler/parser_test.go)**: Updated expected values to match new structure
- **[scraper_test.go](apps/api/internal/business/crawler/scraper_test.go)**: Updated mock data to match new structure

### 3. Test Results

```bash
$ go test ./...
ok  	.../internal/business/crawler	0.794s
ok  	.../internal/platform/smarty	(cached)
```

All tests passing ✅

## Verification Steps

To verify the fix works with real data:

### Option 1: Trigger New Crawl Job (Recommended)

1. **Clear existing malformed data** (optional but recommended):
   ```bash
   # In Firebase Console → Firestore → mailboxes collection
   # Delete all documents or just the malformed ones
   ```

2. **Start a new crawl job** via the web UI:
   - Navigate to **Crawler** page
   - Click **"Start New Job"**
   - Enter a few test URLs like:
     ```
     https://www.anytimemailbox.com/s/chicago-73-w-monroe-street
     https://www.anytimemailbox.com/s/new-york-wall-street
     ```
   - Click **Start**

3. **Verify data in Mailboxes page**:
   - Check that **Name** shows actual store names (e.g., "Chicago - Monroe St")
   - Check that **Address** shows correct street/city/state/zip
   - Check that **Price** shows actual prices (e.g., 19.99)

### Option 2: Re-crawl Existing Links

If you want to re-parse existing links without deleting data:

1. **Get existing links** from Firebase:
   ```bash
   # Export existing links from mailboxes collection
   # Or use the CSV export feature
   ```

2. **Trigger crawl with existing links**:
   - The hash-based change detection will recognize data has changed
   - Updated records will overwrite malformed ones

### Option 3: Local Testing

```bash
cd apps/api

# Set environment variables (use your .env.local)
export FIREBASE_PROJECT_ID=your-project-id
export FIREBASE_CREDS_FILE=service-account.json
export SMARTY_MOCK=true  # Avoid real Smarty calls during testing
export PORT=8080

# Run server
go run ./cmd/server

# In another terminal, trigger crawl via API:
curl -X POST http://localhost:8080/api/crawl/run \
  -H "Content-Type: application/json" \
  -d '{
    "links": [
      "https://www.anytimemailbox.com/s/chicago-73-w-monroe-street"
    ]
  }'

# Check crawl status
curl http://localhost:8080/api/crawl/status?runId=RUN_XXXXXXXXXX

# Verify parsed data
curl http://localhost:8080/api/mailboxes
```

## Files Modified

1. **[apps/api/internal/business/crawler/parser.go](apps/api/internal/business/crawler/parser.go)**
   - Updated name extraction: `h1` (line 21)
   - Updated address parsing: `.t-text > div` (lines 34-58)
   - Updated price extraction: `.t-plan .t-price` (line 62)

2. **[apps/api/internal/business/crawler/testdata/sample_page.html](apps/api/internal/business/crawler/testdata/sample_page.html)**
   - Replaced with actual AnytimeMailbox HTML structure

3. **[apps/api/internal/business/crawler/parser_test.go](apps/api/internal/business/crawler/parser_test.go)**
   - Updated expected values (lines 20-40)

4. **[apps/api/internal/business/crawler/scraper_test.go](apps/api/internal/business/crawler/scraper_test.go)**
   - Updated mock existing mailbox data (lines 51-63)
   - Updated test links (lines 72-82)

## Expected Outcome

After deploying this fix and running a new crawl:

| Field | Before | After |
|-------|--------|-------|
| Name | "YOUR NAME" | "Chicago - Monroe St" |
| Street | "" or incorrect | "73 W Monroe St" |
| City | "" or incorrect | "Chicago" |
| State | "" or incorrect | "IL" |
| Zip | "" or incorrect | "60603" |
| Price | 0.00 | 19.99 |
| CMRA | "Y" ✅ | "Y" ✅ |
| RDI | "Commercial" ✅ | "Commercial" ✅ |

## Deployment Checklist

- [x] Code changes completed
- [x] Tests updated and passing
- [ ] Deploy to backend (Render or local)
- [ ] Trigger test crawl with 2-3 URLs
- [ ] Verify data in Firebase/Mailboxes UI
- [ ] Run full crawl if test successful
- [ ] Update PRD documentation if needed

## Notes

- **Smarty validation still works**: The `cmra` and `rdi` fields were already correct because Smarty validates them after parsing
- **Incremental writes**: The previous fix (writing to DB every 100 items) is still in place
- **Hash-based detection**: Re-running existing URLs will update malformed records because the hash will change with correct data
