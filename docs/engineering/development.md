# VaultMTG Development Guide

This guide covers setting up your development environment, understanding the codebase, and contributing to VaultMTG.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Running the Application](#running-the-application)
- [Debugging](#debugging)
- [Testing](#testing)
- [Code Organization](#code-organization)
- [Adding New Features](#adding-new-features)
- [Contributing Guidelines](#contributing-guidelines)

## Development Setup

### Prerequisites

**Required**:
- **Go 1.23+** - [Download](https://go.dev/dl/)
- **Node.js 20+** - [Download](https://nodejs.org/)
- **Git** - Version control

**Optional but Recommended**:
- **GoLand** or **VS Code** - IDEs with Go support
- **golangci-lint** - Go linter
- **gofumpt** - Go code formatter

### Installing Development Tools

```bash
# golangci-lint (linter)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# gofumpt (formatter)
go install mvdan.cc/gofumpt@latest
```

### Cloning the Repository

```bash
git clone https://github.com/RdHamilton/vault-mtg.git
cd vault-mtg
```

### Installing Dependencies

**Go dependencies**:
```bash
go mod download
go mod tidy
```

**Frontend dependencies**:
```bash
cd frontend
npm install
cd ..
```

### Verifying Setup

```bash
# Verify Go installation
go version

# Verify Node.js installation
node --version

# Build the API server
go build ./cmd/apiserver

# Build the frontend
cd frontend && npm run build
```

## Project Structure

```
MTGA-Companion/
├── cmd/
│   ├── apiserver/
│   │   └── main.go         # REST API server entry point
│   └── mtga-companion/
│       ├── main.go         # CLI entry point
│       ├── daemon.go       # Daemon mode implementation
│       └── service.go      # Service management commands
├── frontend/                # React + TypeScript frontend
│   ├── src/
│   │   ├── components/     # Reusable UI components
│   │   ├── pages/          # Page components (routes)
│   │   ├── App.tsx         # Root component
│   │   └── main.tsx        # Frontend entry point
│   ├── package.json
│   └── vite.config.ts
├── internal/                # Private application code
│   ├── gui/                # GUI-specific backend code
│   ├── mtga/               # MTGA-specific logic
│   │   ├── logreader/     # Log parsing
│   │   ├── poller/        # Log file monitoring
│   │   ├── logprocessor/  # Shared log processing
│   │   └── draft/         # Draft overlay
│   ├── storage/            # Database and persistence
│   │   ├── models/        # Data models
│   │   └── repository/    # Data access layer
│   └── ipc/                # WebSocket IPC (client/server)
├── docs/                    # Documentation
├── scripts/                 # Development scripts
└── CLAUDE.md               # AI assistant guidance
```

### Key Files

**REST API Server**:
- `cmd/apiserver/main.go` - Entry point for the REST API server
- `frontend/` - React frontend code (communicates via REST API)

**CLI Application**:
- `cmd/mtga-companion/main.go` - Entry point for CLI
- `cmd/mtga-companion/daemon.go` - Daemon mode (background service)
- `cmd/mtga-companion/service.go` - Service installation/management

**Shared Code**:
- `internal/mtga/logprocessor/` - Log parsing (used by both API server and daemon)
- `internal/storage/` - Database access (shared)
- `internal/ipc/` - WebSocket IPC (client/server)

## Development Workflow

### Development Scripts

**Backend Development** (`./scripts/dev.sh`):
```bash
./scripts/dev.sh           # Run all checks and build
./scripts/dev.sh fmt       # Format code
./scripts/dev.sh vet       # Run go vet
./scripts/dev.sh lint      # Run golangci-lint
./scripts/dev.sh check     # Run fmt, vet, and lint
./scripts/dev.sh build     # Build CLI application
```

**Testing** (`./scripts/test.sh`):
```bash
./scripts/test.sh          # Run all tests with race detection
./scripts/test.sh coverage # Generate coverage report
./scripts/test.sh verbose  # Run with verbose output
```

### Git Workflow

**Branch naming**:
- Feature: `feature/feature-name`
- Bug fix: `fix/bug-description`
- Documentation: `docs/topic`

**Commit messages**:
- Follow conventional commits
- Examples:
  - `feat: Add draft overlay support`
  - `fix: Resolve database lock on GUI startup`
  - `docs: Update installation guide`
  - `refactor: Extract log processor into shared package`

**PR process**:
1. Create feature branch from `main`
2. Implement changes with tests
3. Run `./scripts/dev.sh check` - ensure all checks pass
4. Run `./scripts/test.sh` - ensure all tests pass
5. Push branch and create PR
6. Address review comments
7. Merge when CI passes and approved

## Running the Application

### Development Mode (Recommended)

Run the REST API server and frontend dev server in two terminals:

**Terminal 1 (API Server)**:
```bash
go run ./cmd/apiserver
```

**Terminal 2 (Frontend)**:
```bash
cd frontend && npm run dev
```

**Access**:
- Frontend runs on http://localhost:5173 (Vite dev server with hot reload)
- REST API server runs on http://localhost:8080
- Changes to frontend files reload instantly
- Changes to Go files require restarting the API server

### Daemon Development Mode

**Run daemon separately**:
```bash
# Build CLI first
go build -o bin/mtga-companion ./cmd/mtga-companion

# Run daemon
./bin/mtga-companion daemon

# With debug logging
./bin/mtga-companion daemon --debug-mode
```

**Run daemon + API server + frontend together**:

Terminal 1 (Daemon):
```bash
./bin/mtga-companion daemon --debug-mode
```

Terminal 2 (API Server):
```bash
go run ./cmd/apiserver
```

Terminal 3 (Frontend):
```bash
cd frontend && npm run dev
```

### CLI Development

**Build and run CLI**:
```bash
go build -o bin/mtga-companion ./cmd/mtga-companion
./bin/mtga-companion read
./bin/mtga-companion export stats -json
```

### GoLand/IDE Configuration

1. **API Server Run Configuration**:
   - Name: "API Server"
   - Type: Go Build
   - Package path: `github.com/ramonehamilton/MTGA-Companion/cmd/apiserver`
   - Working directory: `$PROJECT_DIR$`

2. **Daemon Run Configuration**:
   - Name: "Daemon"
   - Type: Go Build
   - Package path: `github.com/ramonehamilton/MTGA-Companion/cmd/mtga-companion`
   - Program arguments: `daemon --debug-mode`
   - Working directory: `$PROJECT_DIR$`

3. **Frontend Run Configuration**:
   - Name: "Frontend Dev"
   - Type: npm
   - Script: `dev`
   - Package.json: `$PROJECT_DIR$/frontend/package.json`

4. **Compound Configuration** (Run All):
   - Create compound configuration
   - Add "Daemon", "API Server", and "Frontend Dev"
   - Run simultaneously

## Debugging

### Backend Debugging (Go)

**Add debug logging**:
```go
import "log"

log.Printf("[DEBUG] Variable value: %+v", variable)
```

**Use debugger in IDE**:
- Set breakpoints in Go code
- Run the API server in debug mode from your IDE
- Debugger attaches to Go backend process

**Enable debug mode**:
```bash
# Daemon
./bin/mtga-companion daemon --debug-mode

# API Server (set environment variable)
DEBUG=true go run ./cmd/apiserver
```

### Frontend Debugging (React)

**Browser DevTools**:
- Right-click in app → "Inspect Element"
- Console tab shows `console.log()` output
- React DevTools available

**Console logging**:
```typescript
console.log('Debug info:', data);
console.error('Error occurred:', error);
```

**WebSocket events in frontend**:
```typescript
// Subscribe to real-time events via WebSocket
websocket.on('debug:event', (data) => {
  console.log('Event received:', data);
});
```

### WebSocket Debugging

**Test daemon WebSocket**:
```bash
# Check if daemon is listening
curl http://localhost:9999/status

# Expected: {"status":"ok"}
```

**Monitor WebSocket traffic**:
```go
// In daemon (internal/ipc/server.go)
log.Printf("[WS] Broadcasting event: %s", eventType)

// In GUI (internal/ipc/client.go)
log.Printf("[WS] Received event: %s", event.Type)
```

**Frontend WebSocket debugging**:
```typescript
// Check connection status
const status = await GetConnectionStatus();
console.log('Daemon connection:', status);
```

### Database Debugging

**Inspect SQLite database**:
```bash
# Open database
sqlite3 ~/.mtga-companion/data.db

# List tables
.tables

# Query matches
SELECT * FROM matches ORDER BY created_at DESC LIMIT 10;

# Exit
.quit
```

**Enable SQL query logging**:
```go
// In storage/repository code
log.Printf("[SQL] Query: %s, Args: %v", query, args)
```

## Testing

### Running Tests

**All tests**:
```bash
go test ./...
```

**With coverage**:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Specific package**:
```bash
go test ./internal/storage
go test ./internal/mtga/logprocessor -v
```

**With race detection**:
```bash
go test -race ./...
```

### Writing Tests

**Unit test example**:
```go
package storage

import (
    "testing"
)

func TestMatchRepository_GetMatches(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    defer db.Close()

    repo := NewMatchRepository(db)

    // Test
    matches, err := repo.GetMatches(Filter{})

    // Assert
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    if len(matches) != 5 {
        t.Errorf("Expected 5 matches, got: %d", len(matches))
    }
}
```

**Table-driven tests**:
```go
func TestParseLogEntry(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *LogEntry
        wantErr bool
    }{
        {
            name:  "valid entry",
            input: `{"type":"MatchCreated","data":{...}}`,
            want:  &LogEntry{Type: "MatchCreated"},
            wantErr: false,
        },
        {
            name:    "invalid JSON",
            input:   `{invalid`,
            want:    nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseLogEntry(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseLogEntry() error = %v, wantErr %v", err, tt.wantErr)
            }
            // ... more assertions
        })
    }
}
```

### Frontend Testing

Frontend testing uses **Vitest** for component tests and **Playwright** for E2E tests.

**Component Tests (Vitest + React Testing Library)**:
```bash
cd frontend

# Run in watch mode (development)
npm run test

# Run once (CI)
npm run test:run

# Run with UI
npm run test:ui

# Generate coverage report
npm run test:coverage
```

**E2E Tests (Playwright)**:
```bash
# Start the API server and frontend dev server first
# Terminal 1: go run ./cmd/apiserver
# Terminal 2: cd frontend && npm run dev

# In another terminal:
cd frontend

# Run E2E tests (headless)
npm run test:e2e

# Run with interactive UI
npm run test:e2e:ui

# Run in debug mode
npm run test:e2e:debug

# View test report
npx playwright show-report
```

**Mocking Strategy**:

- **Fetch-level mocking** (`fetchMock.mockResponseOnce`): Use when the component calls `fetch` directly and you want to test the full data-fetching path including parsing and error handling. Prefer this for simple components without a dedicated API service layer.
- **Module-level mocking** (`mockApi.matches.getStats.mockResolvedValue`): Use when the component calls a typed API service function (e.g., `api.matches.getStats`). This is the preferred approach for components using the REST API adapter, as it mocks at the service boundary and avoids coupling tests to HTTP details.

**Component Test Example (fetch-level mocking)**:
```typescript
import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { render } from '../test/utils/testUtils';
import Footer from './Footer';

describe('Footer', () => {
  it('should display match statistics', async () => {
    // Mock the REST API response at the fetch level
    fetchMock.mockResponseOnce(JSON.stringify({
      TotalMatches: 100,
      WinRate: 0.6,
    }));

    render(<Footer />);

    // Wait for async data to load
    await waitFor(() => {
      expect(screen.getByText('100')).toBeInTheDocument();
      expect(screen.getByText(/60%/)).toBeInTheDocument();
    });
  });
});
```

**Component Test Example (module-level mocking)**:
```typescript
import { describe, it, expect } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { render } from '../test/utils/testUtils';
import { mockApi } from '../test/mocks/apiMock';
import Footer from './Footer';

describe('Footer', () => {
  it('should display match statistics', async () => {
    mockApi.matches.getStats.mockResolvedValue({
      TotalMatches: 100,
      WinRate: 0.6,
    });

    render(<Footer />);

    await waitFor(() => {
      expect(screen.getByText('100')).toBeInTheDocument();
      expect(screen.getByText(/60%/)).toBeInTheDocument();
    });
  });
});
```

**E2E Test Example**:
```typescript
import { test, expect } from '@playwright/test';

test('should navigate to draft view', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('.app-container')).toBeVisible();

  await page.getByText('Draft').click();
  await expect(page).toHaveURL(/\/draft/);
});
```

**Testing Best Practices**:
- Test user behavior, not implementation details
- Use meaningful test descriptions: "should [do X] when [Y condition]"
- Mock REST API calls appropriately
- Use `waitFor` for async operations, not fixed timeouts
- Test loading states, error states, and empty states
- Keep tests isolated and independent

For comprehensive testing documentation, see [docs/TESTING.md](./TESTING.md).

## Code Organization

### Backend Patterns

**Repository Pattern**:
```go
// Define interface
type MatchRepository interface {
    GetMatches(filter Filter) ([]*Match, error)
    GetMatchByID(id string) (*Match, error)
    SaveMatch(match *Match) error
}

// Implement interface
type matchRepository struct {
    db *sql.DB
}

func NewMatchRepository(db *sql.DB) MatchRepository {
    return &matchRepository{db: db}
}
```

**Dependency Injection**:
```go
// App struct holds dependencies
type App struct {
    db         *storage.DB
    ipcClient  *ipc.Client
    poller     *poller.Poller
}

// Injected via constructor
func NewApp(db *storage.DB) *App {
    return &App{
        db: db,
    }
}
```

**Error Handling**:
```go
// Return errors, don't panic
func GetMatch(id string) (*Match, error) {
    match, err := repo.GetMatchByID(id)
    if err != nil {
        return nil, fmt.Errorf("failed to get match: %w", err)
    }
    return match, nil
}

// Wrap errors for context
if err != nil {
    return fmt.Errorf("parsing log entry: %w", err)
}
```

### Frontend Patterns

**Hooks for data fetching**:
```typescript
const [matches, setMatches] = useState<Match[]>([]);
const [loading, setLoading] = useState(true);

useEffect(() => {
    loadMatches();
}, []);

const loadMatches = async () => {
    try {
        const data = await GetMatches();
        setMatches(data);
    } catch (error) {
        console.error('Failed to load matches:', error);
    } finally {
        setLoading(false);
    }
};
```

**Event listeners (WebSocket)**:
```typescript
useEffect(() => {
    // Subscribe to real-time events via WebSocket
    const unsubscribe = websocket.on('match:new', () => {
        loadMatches(); // Refresh data
    });

    // Cleanup
    return () => {
        unsubscribe();
    };
}, []);
```

## Adding New Features

### Adding a New WebSocket Event

**1. Emit event from daemon** (`cmd/mtga-companion/daemon.go`):
```go
server.Broadcast("inventory:updated", map[string]interface{}{
    "gems": 1500,
    "gold": 5000,
})
```

**2. Handle in API server** (`cmd/apiserver/main.go` or handler):
```go
func (h *Handler) handleInventoryUpdate(data map[string]interface{}) {
    // Process data
    log.Printf("Inventory updated: %+v", data)

    // Broadcast to connected WebSocket clients
    h.wsHub.Broadcast("inventory:updated", data)
}
```

**3. Listen in frontend** (`frontend/src/pages/Inventory.tsx`):
```typescript
useEffect(() => {
    const unsubscribe = websocket.on('inventory:updated', (data: any) => {
        setInventory(data);
    });

    return () => {
        unsubscribe();
    };
}, []);
```

### Adding a New Database Table

**1. Create migration** (`internal/storage/migrations/0005_add_inventory.up.sql`):
```sql
CREATE TABLE inventory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    gems INTEGER NOT NULL,
    gold INTEGER NOT NULL,
    wildcards_common INTEGER NOT NULL,
    updated_at DATETIME NOT NULL
);
```

**2. Create down migration** (`0005_add_inventory.down.sql`):
```sql
DROP TABLE inventory;
```

**3. Create model** (`internal/storage/models/inventory.go`):
```go
type Inventory struct {
    ID              int
    Gems            int
    Gold            int
    WildcardsCommon int
    UpdatedAt       time.Time
}
```

**4. Create repository** (`internal/storage/inventory_repository.go`):
```go
type InventoryRepository interface {
    GetLatest() (*models.Inventory, error)
    Save(inv *models.Inventory) error
}

type inventoryRepository struct {
    db *sql.DB
}

func (r *inventoryRepository) GetLatest() (*models.Inventory, error) {
    // Implementation
}
```

**5. Run migration**:
```bash
./bin/mtga-companion migrate up
```

### Adding a New REST API Endpoint

**1. Add handler** (in `internal/api/handlers/`):
```go
// GetInventory returns current inventory
func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
    inv, err := h.facade.GetLatestInventory()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(inv)
}
```

**2. Register route** (in `internal/api/router.go`):
```go
r.Route("/api/v1", func(r chi.Router) {
    r.Get("/inventory", handler.GetInventory)
})
```

**3. Call from frontend**:
```typescript
const response = await fetch('/api/v1/inventory');
const inventory = await response.json();
```

## Contributing Guidelines

### Before Submitting PR

**Checklist**:
- [ ] Code follows Go best practices (see `CLAUDE.md`)
- [ ] All tests pass (`./scripts/test.sh`)
- [ ] Code is formatted (`./scripts/dev.sh fmt`)
- [ ] Linter passes (`./scripts/dev.sh lint`)
- [ ] No new compiler warnings
- [ ] Documentation updated if needed
- [ ] PR references issue number (e.g., "Closes #123")

### Code Style

**Go**:
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofumpt` for formatting
- Pass `golangci-lint` checks
- Keep functions small and focused
- Write tests for new functionality

**TypeScript/React**:
- Use TypeScript strict mode
- Functional components with hooks
- Props interfaces for all components
- Meaningful variable names
- Comments for complex logic

### Review Process

1. **Submit PR** - Create PR with clear description
2. **Automated Checks** - CI runs tests, linter, build
3. **Code Review** - Maintainer reviews code
4. **Address Feedback** - Make requested changes
5. **Approval** - PR approved by maintainer
6. **Merge** - PR merged to main

### Getting Help

**Resources**:
- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
- [DAEMON_API.md](DAEMON_API.md) - WebSocket API reference
- [CLAUDE.md](../CLAUDE.md) - Project conventions

**Support**:
- GitHub Issues - Report bugs or request features
- GitHub Discussions - Ask questions
- Discord - (Future: Community chat)

## Common Development Tasks

### Build Production Binary

**API server**:
```bash
go build -o bin/apiserver ./cmd/apiserver
```

**Frontend**:
```bash
cd frontend && npm run build
# Output: frontend/dist/
```

**CLI binary**:
```bash
go build -o bin/mtga-companion ./cmd/mtga-companion
```

### Update Dependencies

**Go modules**:
```bash
go get -u ./...
go mod tidy
```

**Frontend**:
```bash
cd frontend
npm update
cd ..
```

### Database Migrations

**Create new migration**:
```bash
# Manually create files:
# internal/storage/migrations/0006_description.up.sql
# internal/storage/migrations/0006_description.down.sql
```

**Apply migrations**:
```bash
./bin/mtga-companion migrate up
```

**Rollback**:
```bash
./bin/mtga-companion migrate down
```

**Check status**:
```bash
./bin/mtga-companion migrate status
```

## Performance Profiling

### CPU Profiling

```bash
go test -cpuprofile=cpu.prof -bench=. ./internal/mtga/logprocessor
go tool pprof cpu.prof
```

### Memory Profiling

```bash
go test -memprofile=mem.prof -bench=. ./internal/storage
go tool pprof mem.prof
```

### Race Detection

```bash
go test -race ./...
```

## Security Considerations

### Database

- SQLite file permissions (user-only access)
- Prepared statements to prevent SQL injection
- No sensitive data in database

### WebSocket

- Daemon listens on localhost only (not network-accessible)
- No authentication required (local-only access)
- Future: TLS for network access

### Log Files

- Read-only access to MTGA Player.log
- Never write to game files
- No sensitive data logged

## Troubleshooting Development Issues

### "Frontend dependencies missing"

```bash
cd frontend
rm -rf node_modules package-lock.json
npm install
cd ..
```

### "Database is locked"

Stop all running instances:
```bash
./bin/mtga-companion service stop
killall mtga-companion
```

### "Port 9999 already in use"

Find and kill process using port:
```bash
# macOS/Linux
lsof -ti:9999 | xargs kill -9

# Windows
netstat -ano | findstr :9999
taskkill /PID <PID> /F
```

## Next Steps

- Read [ARCHITECTURE.md](ARCHITECTURE.md) to understand the system design
- Review [DAEMON_API.md](DAEMON_API.md) for WebSocket events
- Check [CLAUDE.md](../CLAUDE.md) for coding principles
- Start contributing! Check [good first issues](https://github.com/RdHamilton/vault-mtg/labels/good%20first%20issue)
