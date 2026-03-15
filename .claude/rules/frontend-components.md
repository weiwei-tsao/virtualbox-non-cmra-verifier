# Frontend Component Patterns

## Toast Notification System

**Location**: `contexts/ToastContext.tsx`, `components/Toast.tsx`, `components/ToastContainer.tsx`

### Basic Usage

```typescript
import { useToast } from '../contexts/ToastContext';

function MyComponent() {
  const { showToast } = useToast();

  const handleClick = () => {
    showToast('Operation successful!', 'success');
    showToast('Error occurred', 'error', 5000); // Custom duration
  };
}
```

### Toast Types

| Type | Color | Icon | Use Case |
|------|-------|------|----------|
| `success` | Green | ✓ | Successful operations |
| `error` | Red | ✗ | Failed operations |
| `info` | Blue | ℹ | Informational messages |
| `warning` | Yellow | ⚠ | Warnings |

### Features

- **Auto-dismiss**: Default 5 seconds, configurable
- **Positioning**: Bottom-right corner
- **Max visible**: 3 toasts at once (oldest dismissed first)
- **No dependencies**: Pure React implementation

### Pattern: Operation Lifecycle

```typescript
const handleExport = async () => {
  showToast('Preparing export...', 'info');

  try {
    await exportData();
    showToast('Export completed!', 'success');
  } catch (error) {
    showToast('Export failed', 'error');
  }
};
```

---

## Reusable Badge Components

**Location**: `components/badges/`

### Available Badges

```typescript
import { RDIBadge, CMRABadge, SourceBadge } from '../components/badges';

// RDI Badge
<RDIBadge rdi="Commercial" />    // Blue badge
<RDIBadge rdi="Residential" />   // Green badge
<RDIBadge rdi="" />              // Gray "Unknown"

// CMRA Badge
<CMRABadge cmra="Y" />           // Blue "Yes"
<CMRABadge cmra="N" />           // Gray "No"
<CMRABadge cmra="" />            // Gray "Unknown"

// Source Badge
<SourceBadge source="ATMB" />    // Purple badge
<SourceBadge source="iPost1" />  // Orange badge
```

### Styling Patterns

Each badge component handles its own styling:

```typescript
// RDIBadge.tsx
export function RDIBadge({ rdi }: { rdi: string }) {
  const getColor = () => {
    if (rdi === 'Commercial') return 'bg-blue-100 text-blue-800';
    if (rdi === 'Residential') return 'bg-green-100 text-green-800';
    return 'bg-gray-100 text-gray-800';
  };

  return (
    <span className={`px-2 py-1 rounded text-sm ${getColor()}`}>
      {rdi || 'Unknown'}
    </span>
  );
}
```

**Pattern**: Encapsulate styling logic within component, consistent size/shape.

---

## StatCard Component

**Location**: `components/ui/StatCard.tsx`

### Usage

```typescript
import { StatCard } from '../components/ui/StatCard';

<StatCard
  title="Total Mailboxes"
  value="2,045"
  icon={<TrendingUp />}
  color="bg-primary text-primary"
  onClick={() => setFilter('all')}
  isActive={filter === 'all'}
/>
```

### Props

| Prop | Type | Description |
|------|------|-------------|
| `title` | string | Card header text |
| `value` | string \| number | Main display value |
| `icon` | ReactNode | Optional icon component |
| `color` | string | Tailwind color classes |
| `onClick` | () => void | Click handler (makes card interactive) |
| `isActive` | boolean | Visual active state |

### Pattern: Filter Cards

```typescript
const [filter, setFilter] = useState<string>('all');

<div className="grid grid-cols-4 gap-4">
  <StatCard
    title="All Mailboxes"
    value={stats.total}
    onClick={() => setFilter('all')}
    isActive={filter === 'all'}
  />
  <StatCard
    title="Commercial"
    value={stats.commercial}
    onClick={() => setFilter('commercial')}
    isActive={filter === 'commercial'}
  />
</div>
```

**Pattern**: Use `isActive` to highlight selected filter, consistent grid layout.

---

## CSV Export Hook

**Location**: `hooks/useCSVExport.ts`

### Usage

```typescript
import { useCSVExport } from '../hooks/useCSVExport';

function MailboxesPage() {
  const { exportCSV } = useCSVExport();

  const handleExport = () => {
    exportCSV({
      state: selectedState,
      cmra: selectedCMRA,
      rdi: selectedRDI,
      source: selectedSource,
    });
  };

  return <button onClick={handleExport}>Export CSV</button>;
}
```

### Features

1. **Fetch-based download**: Better error handling than `window.open`
2. **Dynamic filename extraction**: Reads `Content-Disposition` header
3. **Toast notifications**: Lifecycle feedback (preparing → success/error)
4. **Blob cleanup**: Proper memory management

### Implementation Pattern

```typescript
export function useCSVExport() {
  const { showToast } = useToast();

  const exportCSV = async (filters: FilterParams) => {
    showToast('Preparing export...', 'info');

    try {
      // Build query params
      const params = new URLSearchParams();
      if (filters.state) params.append('state', filters.state);
      if (filters.cmra) params.append('cmra', filters.cmra);
      // ...

      // Fetch CSV
      const response = await fetch(`${API_URL}/mailboxes/export?${params}`);
      if (!response.ok) throw new Error('Export failed');

      // Extract filename from Content-Disposition header
      const disposition = response.headers.get('Content-Disposition');
      const filenameMatch = disposition?.match(/filename="(.+)"/);
      const filename = filenameMatch?.[1] || 'mailbox.csv';

      // Download blob
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      a.click();

      // Cleanup
      window.URL.revokeObjectURL(url);

      showToast('Export completed!', 'success');
    } catch (error) {
      showToast('Export failed', 'error');
      console.error('Export error:', error);
    }
  };

  return { exportCSV };
}
```

### Why Fetch over window.open?

| Approach | Error Handling | Filename Control | Toast Feedback |
|----------|---------------|------------------|----------------|
| `window.open()` | ❌ No | ❌ No | ❌ No |
| `fetch()` + blob | ✅ Yes | ✅ Yes | ✅ Yes |

---

## Export Filename Format

CSV exports use dynamic filenames based on active filters.

### Format

```
mailbox-{state}-{source}-{cmra}-{rdi}-{timestamp}.csv
```

- Empty filter segments are **skipped**
- Timestamp in **UTC** (RFC3339 basic format)

### Examples

| Active Filters | Filename |
|----------------|----------|
| None | `mailbox-20260313T142530Z.csv` |
| State=CA | `mailbox-CA-20260313T142530Z.csv` |
| State=CA, Source=ATMB | `mailbox-CA-ATMB-20260313T142530Z.csv` |
| State=CA, Source=ATMB, CMRA=Y | `mailbox-CA-ATMB-Y-20260313T142530Z.csv` |
| All filters | `mailbox-CA-ATMB-Y-Commercial-20260313T142530Z.csv` |

### Extraction Pattern

```typescript
const disposition = response.headers.get('Content-Disposition');
const filenameMatch = disposition?.match(/filename="(.+)"/);
const filename = filenameMatch?.[1] || 'mailbox.csv';
```

**Why**: Filename reflects what's in the CSV, making it self-documenting.

---

## Component Composition Patterns

### Container/Presenter Pattern

```typescript
// Container (pages/Mailboxes.tsx)
function MailboxesPage() {
  const [data, setData] = useState([]);
  const [filters, setFilters] = useState({});

  // Fetch data, handle state
  useEffect(() => {
    fetchMailboxes(filters).then(setData);
  }, [filters]);

  return <MailboxesTable data={data} onFilterChange={setFilters} />;
}

// Presenter (components/MailboxesTable.tsx)
function MailboxesTable({ data, onFilterChange }) {
  return (
    <table>
      {data.map(item => <MailboxRow key={item.id} {...item} />)}
    </table>
  );
}
```

**Pattern**: Pages handle state/data fetching, components handle presentation.

---

## TanStack Query Patterns

### Basic Query

```typescript
import { useQuery } from '@tanstack/react-query';
import { fetchMailboxes } from '../services/api';

function MailboxesPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['mailboxes', filters],
    queryFn: () => fetchMailboxes(filters),
  });

  if (isLoading) return <Loading />;
  if (error) return <Error message={error.message} />;

  return <MailboxesTable data={data} />;
}
```

### Mutation Pattern

```typescript
import { useMutation, useQueryClient } from '@tanstack/react-query';

function CrawlerPage() {
  const queryClient = useQueryClient();
  const { showToast } = useToast();

  const startCrawl = useMutation({
    mutationFn: () => api.startCrawl(),
    onSuccess: (data) => {
      showToast('Crawl started!', 'success');
      queryClient.invalidateQueries({ queryKey: ['crawl-runs'] });
    },
    onError: (error) => {
      showToast('Failed to start crawl', 'error');
    },
  });

  return <button onClick={() => startCrawl.mutate()}>Start Crawl</button>;
}
```

---

## Loading States

### Skeleton Pattern

```typescript
function MailboxesTable({ isLoading, data }) {
  if (isLoading) {
    return (
      <div className="animate-pulse">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="h-16 bg-gray-200 mb-2 rounded" />
        ))}
      </div>
    );
  }

  return <table>{/* actual data */}</table>;
}
```

**Pattern**: Show skeleton UI that matches final layout during loading.

---

## Error Boundaries

### Component-Level Error Handling

```typescript
function MailboxesPage() {
  const [error, setError] = useState<string | null>(null);

  const handleExport = async () => {
    try {
      await exportCSV(filters);
    } catch (err) {
      setError('Export failed. Please try again.');
      showToast('Export failed', 'error');
    }
  };

  return (
    <div>
      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}
      <button onClick={handleExport}>Export</button>
    </div>
  );
}
```

**Pattern**: Show inline errors near the action that failed, allow dismissal.

---

## TypeScript Patterns

### Props Interface

```typescript
interface StatCardProps {
  title: string;
  value: string | number;
  icon?: React.ReactNode;
  color?: string;
  onClick?: () => void;
  isActive?: boolean;
}

export function StatCard({
  title,
  value,
  icon,
  color = 'bg-gray-100',
  onClick,
  isActive = false
}: StatCardProps) {
  // ...
}
```

**Pattern**: Define props interface, use default values, mark optional with `?`.

### API Response Types

```typescript
// types.ts
export interface Mailbox {
  id: string;
  name: string;
  addressRaw: {
    street: string;
    city: string;
    state: string;
    zip: string;
  };
  cmra: string;
  rdi: string;
  source: string;
  price: number;
  link: string;
}

// api.ts
export async function fetchMailboxes(filters: FilterParams): Promise<Mailbox[]> {
  const response = await fetch(`${API_URL}/mailboxes?${buildParams(filters)}`);
  return response.json();
}
```

**Pattern**: Define shared types in `types.ts`, use in API services and components.
