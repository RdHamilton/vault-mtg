# VaultMTG Frontend

Modern React + TypeScript frontend for the VaultMTG desktop application, using a REST API + WebSocket architecture.

## Technology Stack

- **React 19** - UI library with hooks
- **TypeScript** - Type-safe JavaScript
- **React Router** - Client-side routing
- **Recharts** - Data visualization and charting
- **Vite** - Build tool and dev server
- **REST API** - Backend communication via HTTP endpoints
- **WebSocket** - Real-time event updates from the backend

## Project Structure

```
frontend/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── Layout.tsx      # Main app layout with navigation
│   │   ├── Footer.tsx      # Statistics footer bar
│   │   └── ToastContainer.tsx  # Toast notifications
│   ├── pages/              # Page components (routes)
│   │   ├── MatchHistory.tsx      # Match history table
│   │   ├── WinRateTrend.tsx      # Win rate over time chart
│   │   ├── DeckPerformance.tsx   # Deck stats visualization
│   │   ├── RankProgression.tsx   # Rank timeline chart
│   │   ├── FormatDistribution.tsx  # Format breakdown pie chart
│   │   ├── ResultBreakdown.tsx   # Detailed statistics
│   │   └── Settings.tsx          # App settings
│   ├── App.tsx             # Root component with routing
│   ├── App.css             # Global styles and theme
│   └── main.tsx            # Frontend entry point
├── index.html              # HTML entry point
├── package.json            # Dependencies and scripts
├── tsconfig.json           # TypeScript configuration
└── vite.config.ts          # Vite build configuration
```

## Development

### Prerequisites

- Node.js 20+
- npm or yarn

### Install Dependencies

```bash
npm install
```

### Development Mode

**Option 1: Standalone frontend dev server** (faster, but no Go backend):
```bash
npm run dev
```
Opens at `http://localhost:5173` - good for UI-only work

**Option 2: Full development mode** (recommended):
```bash
# Terminal 1: Start the REST API server
cd .. && go run ./cmd/apiserver

# Terminal 2: Start the frontend dev server
npm run dev
```
Runs both Go REST API backend and React frontend with hot reload

### Build for Production

```bash
npm run build
```
Output: `dist/` directory

### Type Checking

```bash
npm run type-check
```

### Linting

```bash
npm run lint
```

## REST API Integration

### Backend Communication

Call Go backend endpoints from TypeScript via the REST API:

```typescript
// Fetch matches from the REST API
const response = await fetch('/api/v1/matches?format=ranked');
const matches = await response.json();

// Fetch stats from the REST API
const statsResponse = await fetch('/api/v1/stats');
const stats = await statsResponse.json();
```

### Real-Time Events

Listen for backend events via WebSocket:

```typescript
useEffect(() => {
  const unsubscribe = websocket.on('stats:updated', (data) => {
    console.log('Stats updated:', data);
    // Refresh data
  });

  return () => {
    if (unsubscribe) {
      unsubscribe();
    }
  };
}, []);
```

## Design System

### Color Palette

```css
/* Dark Theme */
--background: #1e1e1e;           /* Main background */
--background-secondary: #2d2d2d; /* Cards, containers */
--background-tertiary: #3d3d3d;  /* Borders, dividers */
--primary: #4a9eff;              /* Primary accent */
--primary-hover: #3d8fe5;        /* Primary hover */
--primary-active: #357cd8;       /* Primary active */
--text: #ffffff;                 /* Primary text */
--text-secondary: #dddddd;       /* Secondary text */
--text-muted: #aaaaaa;           /* Muted text */
--text-disabled: #666666;        /* Disabled text */
--success: #4caf50;              /* Win, success */
--error: #f44336;                /* Loss, error */
--warning: #ff9800;              /* Warning */
```

### Spacing Scale

Use multiples of 4px or 8px for consistent spacing:
- 4px, 8px, 12px, 16px, 24px, 32px, 48px, 64px

### Typography

- **Page Title**: 24px, weight 600
- **Section Title**: 18px, weight 600
- **Body**: 14px, weight 400
- **Small**: 12px, weight 400

### Responsive Design

**Breakpoints**:
- Minimum: 800x600
- Small: 1024x768
- Medium: 1280x720
- Large: 1920x1080+

**Guidelines**:
- Use flexbox/grid for layouts
- Avoid fixed widths - use percentages or `fr` units
- Tables should scroll horizontally if needed
- Charts should use `ResponsiveContainer`
- Filter rows should wrap on small screens

## Component Patterns

### Page Component Template

```typescript
import { useState, useEffect } from 'react';
import './PageName.css';

interface Match {
  id: string;
  // ... other fields
}

const PageName = () => {
  const [data, setData] = useState<Match[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch('/api/v1/matches');
      const result = await response.json();
      setData(result || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Listen for real-time updates via WebSocket
  useEffect(() => {
    const unsubscribe = websocket.on('stats:updated', () => {
      loadData();
    });
    return () => {
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, []);

  if (loading) return <div className="no-data">Loading...</div>;
  if (error) return <div className="error">{error}</div>;
  if (data.length === 0) return <div className="no-data">No data available</div>;

  return (
    <div className="page-container">
      <h1 className="page-title">Page Name</h1>
      {/* Your content here */}
    </div>
  );
};

export default PageName;
```

### CSS Template

```css
.page-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 16px;
  overflow: hidden;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 16px;
  color: #ffffff;
}

/* Responsive table */
.table-container {
  overflow-x: auto;
  flex: 1;
}

table {
  width: 100%;
  border-collapse: collapse;
}

th {
  background-color: #2d2d2d;
  padding: 12px;
  text-align: left;
  font-weight: 600;
}

td {
  padding: 12px;
  border-bottom: 1px solid #3d3d3d;
}

tr:hover {
  background-color: #2d2d2d;
}
```

## Best Practices

### TypeScript
- ✅ Use proper types for API responses
- ✅ Define interfaces for component props
- ❌ Avoid `any` types
- ❌ Don't use `as` type assertions unless necessary

### React
- ✅ Use functional components with hooks
- ✅ Clean up event listeners in `useEffect` return
- ✅ Handle loading, error, and empty states
- ❌ Don't forget dependency arrays in `useEffect`
- ❌ Avoid prop drilling - lift state when needed

### Styling
- ✅ Use component-scoped CSS files
- ✅ Follow the color palette and spacing scale
- ✅ Test responsive behavior at different sizes
- ❌ Avoid inline styles (except dynamic values)
- ❌ Don't use fixed pixel widths for containers

### Performance
- ✅ Debounce expensive operations (filters, search)
- ✅ Memoize expensive calculations
- ✅ Virtualize long lists if needed
- ❌ Don't re-fetch data unnecessarily
- ❌ Avoid large bundle sizes - lazy load if needed

## Troubleshooting

### TypeScript errors after changing API response types

Ensure your frontend type definitions match the Go struct changes in the API server.

### Hot reload not working

Restart the Vite dev server:
```bash
npm run dev
```

### Chart not rendering

Ensure you're using `ResponsiveContainer`:
```tsx
<ResponsiveContainer width="100%" height={400}>
  <LineChart data={data}>
    {/* ... */}
  </LineChart>
</ResponsiveContainer>
```

## Deployment

Production traffic for `https://app.vaultmtg.app` is served from **S3 + CloudFront**. The build output (`frontend/dist/`) is synced to the `vaultmtg-app-spa` S3 bucket and a CloudFront invalidation is issued. Bucket name and distribution ID are read from SSM at deploy time.

Deploys run on `push: tags: ['v*']` (production tag) or `workflow_dispatch` — see [`.github/workflows/deploy-spa.yml`](../.github/workflows/deploy-spa.yml). Pushes to `main` do not deploy.

**Vercel** is wired up for **PR preview deploys only**. Production tags skip Vercel via the `vercel.json` `ignoreCommand` at the repo root.

For the full deploy model, infrastructure inventory, SSM parameters, and rollback steps, see [`docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md). For the architectural decisions:

- [ADR-008: Frontend Serving Model — S3+CloudFront Canonical, Vercel Preview-Only](../docs/adr/ADR-008-frontend-serving-model.md) — current source of truth
- [ADR-006: Vercel BFF Connectivity](../docs/adr/006-vercel-bff-connectivity.md) — CORS and `VITE_BFF_URL` semantics for cross-origin BFF
- [ADR-007: Frontend Serving Model](../docs/adr/007-frontend-serving-model.md) — superseded by ADR-008; kept for historical context

## Resources

- [React Documentation](https://react.dev/)
- [TypeScript Handbook](https://www.typescriptlang.org/docs/)
- [Recharts Documentation](https://recharts.org/en-US/)
- [Vite Documentation](https://vite.dev/)
