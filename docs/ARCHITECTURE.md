# MTGA-Companion Architecture

## Overview

MTGA-Companion uses a **REST API + Browser SPA** architecture (v1.4+) that decouples the backend from the frontend, enabling flexible deployment and easy testing. The system consists of three main components:

1. **API Server** - Go REST API with WebSocket support
2. **Frontend SPA** - React TypeScript application running in the browser
3. **Daemon Service** - Background log monitoring and real-time event broadcasting

## Architecture Diagram (v1.4+)

```
┌──────────────────────────────────────────────────────────────┐
│                    MTGA (Game Client)                        │
│                                                               │
│  Plays matches, generates game events, writes detailed logs  │
└──────────────────────┬───────────────────────────────────────┘
                       │ writes game events
                       ↓
┌──────────────────────────────────────────────────────────────┐
│                      Player.log File                         │
│                                                               │
│  JSON-formatted log entries (matches, drafts, inventory)     │
└──────────────────────┬───────────────────────────────────────┘
                       │ monitors (fsnotify or polling)
                       ↓
┌──────────────────────────────────────────────────────────────┐
│              CLI Daemon (cmd/mtga-companion)                 │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    Log Monitoring                      │ │
│  │  ┌─────────────┐                ┌─────────────┐       │ │
│  │  │   Poller    │────monitors───▶│ File Events │       │ │
│  │  │  (Goroutine)│                │  (fsnotify) │       │ │
│  │  └──────┬──────┘                └──────┬──────┘       │ │
│  │         └──────────────┬────────────────┘              │ │
│  │                        ↓                               │ │
│  │              ┌────────────────┐                        │ │
│  │              │  Log Processor │                        │ │
│  │              │  - Parses JSON │                        │ │
│  │              │  - Routes data │                        │ │
│  │              └────────┬───────┘                        │ │
│  └───────────────────────┼────────────────────────────────┘ │
│                          ↓                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              WebSocket Server (port 9999)              │ │
│  │  Events: stats:updated, match:new, draft:pick, etc.    │ │
│  └────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
                          │
                          │ WebSocket events
                          ↓
┌──────────────────────────────────────────────────────────────┐
│               REST API Server (cmd/apiserver)                │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              HTTP Router (Chi)                         │ │
│  │  GET  /api/matches      POST /api/decks               │ │
│  │  GET  /api/drafts       GET  /api/collection          │ │
│  │  GET  /api/stats        GET  /api/meta                │ │
│  │  POST /api/settings     WebSocket /api/ws             │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              Facade Layer (internal/gui/)              │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │ │
│  │  │MatchFac. │ │DraftFac. │ │ DeckFac. │ │ MetaFac. │ │ │
│  │  │          │ │          │ │          │ │          │ │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │ │
│  │  │ CardFac. │ │ LLMFac.  │ │ Collect. │ │ Settings │ │ │
│  │  │          │ │          │ │  Facade  │ │  Facade  │ │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              Storage Layer (internal/storage/)         │ │
│  │  Repository Pattern: Matches, Drafts, Decks, Cards,   │ │
│  │  Collection, Settings, DraftRatings, ML Models        │ │
│  │                          │                             │ │
│  │              ┌────────────────────────┐               │ │
│  │              │  SQLite Database       │               │ │
│  │              │  ~/.mtga-companion/    │               │ │
│  │              │  mtga.db               │               │ │
│  │              └────────────────────────┘               │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              ML/AI Services (v1.4+)                    │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐              │ │
│  │  │   ML     │ │  Ollama  │ │   Meta   │              │ │
│  │  │  Engine  │ │  Client  │ │ Service  │              │ │
│  │  └──────────┘ └──────────┘ └──────────┘              │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │            v1.4.1 Services                             │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │ │
│  │  │ Flight   │ │  Draft   │ │ Synergy  │ │ Standard │ │ │
│  │  │ Recorder │ │Analytics │ │ Sources  │ │ Legality │ │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐              │ │
│  │  │   CFB    │ │  Price   │ │   Set    │              │ │
│  │  │ Ratings  │ │ Service  │ │  Syncer  │              │ │
│  │  └──────────┘ └──────────┘ └──────────┘              │ │
│  └────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
                          │
                          │ REST API + WebSocket
                          ↓
┌──────────────────────────────────────────────────────────────┐
│                Browser (Default System Browser)              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              React SPA (frontend/)                     │ │
│  │                                                        │ │
│  │  ┌────────────────────────────────────────────────┐   │ │
│  │  │           API Client (services/api/)           │   │ │
│  │  │  matches.ts  drafts.ts  decks.ts  meta.ts     │   │ │
│  │  │  cards.ts    collection.ts  settings.ts       │   │ │
│  │  └────────────────────────────────────────────────┘   │ │
│  │                          │                            │ │
│  │  ┌────────────────────────────────────────────────┐   │ │
│  │  │           Pages & Components                   │   │ │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐    │   │ │
│  │  │  │  Match   │  │  Draft   │  │  Decks   │    │   │ │
│  │  │  │ History  │  │ Assistant│  │  Builder │    │   │ │
│  │  │  └──────────┘  └──────────┘  └──────────┘    │   │ │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐    │   │ │
│  │  │  │  Meta    │  │Collection│  │  Charts  │    │   │ │
│  │  │  │Dashboard │  │  Browser │  │  & Stats │    │   │ │
│  │  │  └──────────┘  └──────────┘  └──────────┘    │   │ │
│  │  └────────────────────────────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

### 1. CLI Daemon (Backend Service)

**Location**: `cmd/mtga-companion/daemon.go`

**Responsibilities**:
- Monitor MTGA `Player.log` file for changes
- Parse JSON log entries into structured data
- Store data in SQLite database
- Broadcast events to WebSocket clients
- Run as background service (24/7 operation)
- Automatic crash recovery via service manager

**Key Components**:

**Log Poller** (`internal/mtga/poller/poller.go`):
- Monitors log file for changes using fsnotify or polling
- Detects new entries and log rotation
- Handles file system events (create, write, rename, remove)
- Configurable poll interval

**Log Processor** (`internal/mtga/logprocessor/processor.go`):
- Shared component used by both daemon and standalone GUI
- Parses JSON log entries
- Routes data to appropriate storage repositories
- Handles match tracking, draft tracking, inventory updates

**WebSocket Server** (`internal/ipc/server.go`):
- Listens on port 9999 (configurable)
- Manages client connections
- Broadcasts events to all connected clients
- Handles client disconnection gracefully

**Storage Layer** (`internal/storage/`):
- Repository pattern for data access
- SQLite database with migration support
- Repositories: matches, drafts, statistics, settings

### 2. REST API Server

**Location**: `cmd/apiserver/`, `internal/api/`

**Responsibilities**:
- Serve REST API endpoints for frontend
- WebSocket endpoint for real-time updates
- Initialize and manage all backend services
- Open browser to frontend on startup (optional)

**Key Components**:

**HTTP Router** (`internal/api/router.go`):
- Chi-based HTTP router
- RESTful API endpoints for all features
- CORS configuration for browser access
- Health check endpoint

**API Handlers** (`internal/api/handlers/`):
- HTTP handlers for each domain (matches, drafts, decks, etc.)
- Request validation and response formatting
- Delegates to facade layer for business logic

**WebSocket Handler** (`internal/api/websocket/`):
- Real-time event broadcasting
- Client subscription management
- Event routing from daemon

### 3. Frontend SPA (Browser Application)

**Location**: `frontend/`

**Responsibilities**:
- Display match history, statistics, charts
- Handle user interactions and settings
- Real-time updates via WebSocket
- All data fetched via REST API

**Key Components**:

**API Client Modules** (`frontend/src/services/api/`):
- Typed REST API client modules
- `matches.ts`, `drafts.ts`, `decks.ts`, `cards.ts`
- `collection.ts`, `meta.ts`, `settings.ts`, `system.ts`
- Automatic error handling and response typing

**React Frontend** (`frontend/src/`):
- TypeScript + React 18
- Pages: Match History, Draft, Decks, Collection, Meta, Charts
- Components: Layout, tables, charts, status indicators
- Hooks for data fetching and real-time updates

### 4. Shared Components

**Log Processor** (`internal/mtga/logprocessor/`):
- Shared by both daemon and standalone GUI
- Single source of truth for log parsing logic
- Parses matches, drafts, inventory, rank progression

**Storage Repositories** (`internal/storage/`):
- Direct database access layer
- Used by both daemon and standalone GUI
- Consistent data access patterns

## Data Flow

### Normal Operation (Daemon Mode)

```
1. MTGA writes to Player.log
   │
   ↓
2. Daemon's poller detects change (fsnotify event)
   │
   ↓
3. Daemon reads new log entries
   │
   ↓
4. Log processor parses JSON entries
   │
   ↓
5. Parsed data validated and routed
   │
   ↓
6. Repository stores data in SQLite
   │
   ↓
7. WebSocket server broadcasts event
   │  Example: {"type": "match:new", "data": {...}}
   │
   ↓
8. GUI's IPC client receives event
   │
   ↓
9. Event handler triggers data refresh
   │
   ↓
10. GUI fetches updated data from database
    │  (via REST API)
    │
    ↓
11. React components re-render with new data
    │
    ↓
12. User sees updated statistics/match history
```

### Standalone Mode (Fallback)

```
1. MTGA writes to Player.log
   │
   ↓
2. GUI's embedded poller detects change
   │
   ↓
3. GUI's log processor parses entries
   │
   ↓
4. GUI writes directly to database
   │
   ↓
5. GUI triggers internal event
   │
   ↓
6. React components refresh
```

### Application Startup Flow (v1.4+)

```
1. User launches MTGA Companion app
   │
   ↓
2. API server starts (cmd/apiserver)
   │  - Initializes database connection
   │  - Initializes all facades and services
   │  - Starts HTTP server on port 8080
   │
   ↓
3. Browser opens to frontend URL
   │  (http://localhost:3000 or bundled static files)
   │
   ↓
4. Frontend loads and connects to API
   │  - Fetches initial data via REST API
   │  - Establishes WebSocket for real-time updates
   │
   ↓
5. Daemon service (if running) broadcasts events
   │  - Log changes detected → events broadcast
   │  - Frontend receives updates via WebSocket
```

## WebSocket Event Protocol

### Connection

**URL**: `ws://localhost:9999`

**Connection handshake**:
1. Client connects via WebSocket
2. Server accepts connection
3. Client subscribes to events
4. Server broadcasts events to all clients

### Event Types

All events follow this structure:
```json
{
  "type": "event:name",
  "data": { ... },
  "timestamp": "2025-11-15T10:30:00Z"
}
```

**Available Events**:

**`stats:updated`** - Overall statistics changed
```json
{
  "type": "stats:updated",
  "data": {
    "totalMatches": 150,
    "totalGames": 300,
    "winRate": 0.63
  },
  "timestamp": "2025-11-15T10:30:00Z"
}
```

**`match:new`** - New match recorded
```json
{
  "type": "match:new",
  "data": {
    "matchID": "abc-123",
    "result": "Win",
    "format": "ConstructedRanked"
  },
  "timestamp": "2025-11-15T10:30:00Z"
}
```

**`draft:started`** - Draft session started
```json
{
  "type": "draft:started",
  "data": {
    "draftID": "draft-789",
    "setCode": "ONE"
  },
  "timestamp": "2025-11-15T10:30:00Z"
}
```

**`draft:pick`** - Card picked in draft
```json
{
  "type": "draft:pick",
  "data": {
    "draftID": "draft-789",
    "pack": 1,
    "pick": 3,
    "cardID": 89765
  },
  "timestamp": "2025-11-15T10:30:00Z"
}
```

**`connection:status`** - Connection state changed
```json
{
  "type": "connection:status",
  "data": {
    "status": "connected"
  },
  "timestamp": "2025-11-15T10:30:00Z"
}
```

### Error Handling

WebSocket errors are handled with automatic reconnection:

1. **Connection Lost**: Client attempts reconnection with exponential backoff
   - 1st retry: 1 second
   - 2nd retry: 2 seconds
   - 3rd retry: 4 seconds
   - Max: 30 seconds between retries

2. **Daemon Unavailable**: Client falls back to standalone mode
   - Embedded poller starts
   - No WebSocket connection maintained
   - GUI continues functioning normally

3. **Daemon Recovers**: Client automatically reconnects
   - Detects daemon is available again
   - Stops embedded poller
   - Reconnects to daemon WebSocket

## Database Schema

### Tables

**matches**
```sql
CREATE TABLE matches (
    id TEXT PRIMARY KEY,
    event_type TEXT,
    format TEXT,
    result TEXT,
    opponent_id TEXT,
    start_time DATETIME,
    end_time DATETIME,
    duration INTEGER,
    created_at DATETIME
);
```

**games**
```sql
CREATE TABLE games (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    match_id TEXT,
    game_number INTEGER,
    result TEXT,
    on_play BOOLEAN,
    created_at DATETIME,
    FOREIGN KEY (match_id) REFERENCES matches(id)
);
```

**drafts**
```sql
CREATE TABLE drafts (
    id TEXT PRIMARY KEY,
    event_id TEXT,
    set_code TEXT,
    status TEXT,
    created_at DATETIME,
    completed_at DATETIME
);
```

**draft_picks**
```sql
CREATE TABLE draft_picks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    draft_id TEXT,
    pack INTEGER,
    pick INTEGER,
    card_id INTEGER,
    created_at DATETIME,
    FOREIGN KEY (draft_id) REFERENCES drafts(id)
);
```

**settings**
```sql
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME
);
```

### Migrations

Database migrations are managed with `golang-migrate/migrate`.

**Migration files**: `internal/storage/migrations/`

**Naming convention**: `NNNN_description.up.sql` / `NNNN_description.down.sql`

**Running migrations**:
```bash
# Apply all pending migrations
./mtga-companion migrate up

# Rollback last migration
./mtga-companion migrate down

# Check migration status
./mtga-companion migrate status
```

## Security Considerations

### WebSocket Security

**Current**: WebSocket listens on `localhost:9999` only
- Not network-accessible by default
- Only accepts connections from same machine
- No authentication required (local-only)

**Future Enhancement**: For network access, consider:
- TLS encryption (wss://)
- Authentication tokens
- CORS configuration
- Rate limiting

### Database Access

**Protection**: SQLite database is local file with file system permissions
- Located at `~/.mtga-companion/data.db`
- Only accessible by user who owns the file
- No network exposure

**Concurrent Access**: Database locking handled by SQLite
- Only one writer at a time (daemon OR standalone GUI)
- Multiple readers allowed
- Lock timeout: 5 seconds

### Log File Access

**Read-only**: Application only reads `Player.log`, never writes
- No risk of corrupting MTGA game state
- Detection of log rotation and recovery

## Extension Points

### Adding New Event Types

1. **Define event in daemon** (`cmd/mtga-companion/daemon.go`):
   ```go
   server.Broadcast("new:event", map[string]interface{}{
       "data": eventData,
   })
   ```

2. **Handle event in GUI** (`internal/gui/app.go`):
   ```go
   func (a *App) handleNewEvent(event map[string]interface{}) {
       // Process event, update UI
       runtime.EventsEmit(a.ctx, "new:event", event)
   }
   ```

3. **Listen in frontend** (`frontend/src/components/Component.tsx`):
   ```typescript
   EventsOn('new:event', (data) => {
       // Update React state
   });
   ```

### Adding New Data Sources

To track additional MTGA data (e.g., inventory, collection):

1. **Update log processor** (`internal/mtga/logprocessor/processor.go`):
   ```go
   func (p *Processor) ProcessInventoryUpdate(entry JSONEntry) {
       // Parse inventory data
       // Store in database
       // Broadcast event
   }
   ```

2. **Add repository method** (`internal/storage/inventory_repository.go`):
   ```go
   func (r *InventoryRepository) SaveInventory(inv *Inventory) error {
       // Database insert/update
   }
   ```

3. **Create migration** (`internal/storage/migrations/0004_add_inventory.up.sql`):
   ```sql
   CREATE TABLE inventory (...);
   ```

4. **Add GUI method** (`app.go`):
   ```go
   func (a *App) GetInventory() (*Inventory, error) {
       return a.db.Inventory.GetLatest()
   }
   ```

### Adding New Frontend Clients

The daemon can support multiple frontend types:

**Web Frontend**:
- Connect to `ws://localhost:9999` from browser
- Use same WebSocket event protocol
- Implement own UI (React, Vue, Angular, etc.)

**Mobile App**:
- Connect via WebSocket from mobile device
- Daemon would need network binding (not just localhost)
- Implement authentication for security

**Third-Party Tools**:
- Any WebSocket client can connect
- Subscribe to specific events
- Build custom integrations (Discord bot, OBS overlay, etc.)

## Technology Stack

### Backend (Go)

- **Language**: Go 1.25+
- **HTTP Router**: Chi (lightweight, idiomatic)
- **Database**: SQLite3 via `modernc.org/sqlite` (pure Go, no CGo)
- **Migrations**: `golang-migrate/migrate`
- **WebSocket**: `gorilla/websocket`
- **File Watching**: `fsnotify/fsnotify`
- **Service Management**: `kardianos/service`
- **Runtime Tracing**: `runtime/trace.FlightRecorder` (v1.4.1+)

### Frontend (React SPA)

- **Architecture**: REST API + Browser SPA (v1.4+)
- **UI Library**: React 18
- **Language**: TypeScript
- **Build Tool**: Vite
- **Routing**: React Router
- **Charts**: Recharts
- **Testing**: Vitest (unit), Playwright (E2E)

### Platform Support

- **macOS**: Launch Agents (launchd)
- **Windows**: Windows Service (Service Control Manager)
- **Linux**: systemd units

## Performance Characteristics

### Resource Usage

**Daemon**:
- Memory: ~10-20 MB
- CPU: < 1% idle, ~5% during log processing
- Disk I/O: Minimal (reads log, writes database)

**GUI (Connected)**:
- Memory: ~50-100 MB (includes WebView)
- CPU: < 1% idle, ~10% during rendering
- Network: WebSocket only (localhost, negligible)

**GUI (Standalone)**:
- Memory: ~60-120 MB (includes WebView + poller)
- CPU: < 1% idle, ~10% during log processing + rendering

### Scalability

**Database**:
- SQLite handles millions of rows efficiently
- Indexed queries for fast lookups
- Database file size: ~1-5 MB per 1000 matches

**WebSocket**:
- Supports dozens of concurrent clients
- Broadcast overhead minimal (< 1ms per event)
- No performance degradation with multiple GUIs

## Monitoring and Debugging

### Daemon Logs

**macOS**: `~/Library/Logs/MTGACompanionDaemon.log`
**Windows**: Event Viewer → Application → MTGACompanionDaemon
**Linux**: `journalctl -u MTGACompanionDaemon -f`

### Debug Mode

Enable debug logging:
```bash
./mtga-companion daemon --debug-mode
```

Outputs:
- WebSocket connection events
- Database queries
- Log parsing details
- Error stack traces

### WebSocket Connection Testing

Test daemon connectivity:
```bash
curl http://localhost:9999/status
```

Expected response:
```json
{"status": "ok", "version": "1.0.0"}
```

## v1.4.1 Architecture Additions

### Flight Recorder

**Location**: `internal/daemon/flight_recorder.go`

Low-overhead execution tracing using Go 1.25's `runtime/trace.FlightRecorder`:
- Continuous trace capture into ring buffer
- Automatic trace dump on errors exceeding threshold
- Configurable buffer size and retention
- Trace files saved to temp directory for debugging

### Draft Analytics Services

**Location**: `internal/mtga/draft/analytics/`

Comprehensive draft performance analysis:
- `pattern_analyzer.go` - Color/type preferences, pick patterns
- `archetype_performance.go` - Win rates by color pair
- `temporal_trends.go` - Weekly/monthly performance trends
- `community_comparison.go` - Comparison vs 17Lands averages

### Synergy Data Sources

**Location**: Various internal packages

Multiple data sources for card synergy detection:
- **ChannelFireball Ratings** - `internal/mtga/cards/cfb/` - Card ratings A+ to F
- **EDHREC Integration** - Commander synergy data
- **Archidekt Co-occurrence** - Card co-occurrence analysis
- **MTGZone Archetypes** - Archetype classification data
- **Tribal Database** - Creature type synergies
- **Oracle Patterns** - Text pattern matching

### Standard Format Services

**Location**: `internal/mtga/cards/setcache/`

Standard format support:
- `set_sync.go` - Automatic set metadata sync from Scryfall
- Standard legality detection from whatsinstandard.com
- Set rotation tracking and notifications
- Banned card detection

### Price Service

**Location**: `internal/mtga/cards/scryfall/`

Card price integration:
- Scryfall price data fetching
- Collection value calculations
- Deck value estimations
- Price caching and refresh

### External Data Sources

The system integrates with multiple external APIs:

| Source | Purpose | API Type |
|--------|---------|----------|
| 17Lands | Draft ratings, community stats | REST |
| Scryfall | Card metadata, images, prices | REST |
| ChannelFireball | Card ratings | JSON import |
| MTGGoldfish | Meta data | Web scraping |
| MTGTop8 | Tournament results | Web scraping |
| whatsinstandard.com | Standard legality | REST |
| EDHREC | Commander synergies | REST |
| Archidekt | Co-occurrence data | REST |
| MTGZone | Archetype data | Web scraping |

## References

- [DAEMON_INSTALLATION.md](DAEMON_INSTALLATION.md) - Service installation guide
- [DAEMON_API.md](DAEMON_API.md) - WebSocket API reference
- [DEVELOPMENT.md](DEVELOPMENT.md) - Developer guide
- [MIGRATION_TO_SERVICE_ARCHITECTURE.md](MIGRATION_TO_SERVICE_ARCHITECTURE.md) - User migration guide
- [go-1.25-features.md](go-1.25-features.md) - Go 1.25 feature documentation
