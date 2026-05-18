# VaultMTG

A modern companion application for Magic: The Gathering Arena (MTGA). Track your matches, analyze your performance, and enhance your MTGA experience with real-time statistics, ML-powered recommendations, and metagame insights.

## Features

### Modern Web UI (v1.4)
- **Browser-Based Interface**: React SPA with REST API backend - opens in your default browser
- **Real-Time Updates**: Live statistics via WebSocket while you play MTGA
- **Dark Theme**: Easy on the eyes during long gaming sessions
- **Responsive Design**: Works on any screen size

### Match Tracking & Analytics
- **Match History**: View all your matches with filtering and sorting
- **Win Rate Trends**: Visualize performance over time
- **Deck Performance**: Track win rates by deck
- **Format Distribution**: See your play patterns across formats
- **Result Breakdown**: Detailed statistics by format and time period

### Data Management
- **Log Reading**: Automatically locates and reads MTGA Player.log files
- **Auto-Detection**: Cross-platform support for macOS and Windows log locations
- **Real-Time Monitoring**: Poll-based log watching for instant updates
- **Cloud Storage**: Per-account PostgreSQL on the BFF (matches, drafts, decks, collection, settings)
- **Export System**: Export statistics in CSV or JSON formats

### Draft Assistant (v1.1)
- **Real-Time Draft Assistant**: Live card recommendations and analysis during drafts
- **Type Synergy Detection**: Automatic detection of card type synergies with visual indicators
- **Card Suggestions**: Context-aware recommendations based on your picked cards
- **Draft Deck Win Rate Predictor**: AI-powered prediction with letter grades (A/B/C/D/F)
- **Format Meta Insights**: Archetype performance data, best color pairs, overdrafted colors
- **Archetype Performance Dashboard**: Interactive archetype selection with top cards per archetype
- **Draft Statistics Dashboard**: Real-time mana curve and color distribution
- **Missing Cards Detection**: Track cards you don't own from draft packs
- **Historical Draft Replay**: View and replay past draft pick sequences
- **Card Data Integration**: 17Lands public datasets and Scryfall metadata

### Deck Builder (v1.3)
- **Deck Creation & Management**: Build constructed and draft-based decks with full CRUD operations
- **AI-Powered Recommendations**: Intelligent card suggestions based on color fit, mana curve, synergy, and card quality
- **Multiple Import Formats**: Import from Arena, plain text, and other common formats
- **Multiple Export Formats**: Export to Arena, MTGO, MTGGoldfish, and plain text formats
- **Comprehensive Statistics**: Mana curve, color distribution, type breakdown, and land recommendations
- **Draft Pool Validation**: Draft decks restricted to cards from the associated draft
- **Format Legality Checking**: Validate decks for Standard, Historic, Explorer, Alchemy, Brawl, and Commander
- **Performance Tracking**: Automatic win rate tracking and performance metrics
- **Tagging & Organization**: Categorize and filter decks with custom tags
- **Deck Library**: Advanced filtering by format, source, tags, and performance

### Quest & Statistics Tracking (v1.3)
- **Quest Tracking**: Monitor daily and weekly quest progress
- **Accurate Gold Calculation**: Parses actual quest rewards instead of estimates
- **Real-Time Updates**: Live draft updates via `draft:updated` events
- **Set Symbol Display**: Card displays now show set symbols/icons

### ML-Powered Recommendations (v1.4+)
- **Machine Learning Engine**: Intelligent card recommendations using trained ML models
- **Personal Play Style Learning**: Adapts recommendations based on your deck building history and preferences
- **Meta-Aware Suggestions**: Incorporates tournament data and metagame trends into recommendations
- **Ollama Integration**: Optional local LLM support for natural language explanations of recommendations
- **Feedback Collection**: Records your card acceptance/rejection to improve future recommendations
- **Hybrid Scoring**: Combines ML predictions with rule-based analysis for best results
- **Combo/Chain Detection** (v1.4.1): Identify multi-card synergies and combo potential
- **Opponent Deck Analysis** (v1.4.1): Reconstruct opponent decks and match to meta archetypes
- **Performance-Based Recommendations** (v1.4.1): Suggestions based on historical win rates

### Metagame Dashboard (v1.4)
- **Live Meta Data**: Real-time metagame data from MTGGoldfish and MTGTop8
- **Archetype Tier Lists**: View Tier 1-4 archetypes with meta share and tournament performance
- **Archetype Detail View**: Click any archetype for detailed stats, trend analysis, and tier explanations
- **Format Support**: Standard, Historic, Explorer, Pioneer, and Modern formats
- **Tournament Tracking**: Recent tournament results with top decks and winner information

### Draft Enhancements (v1.4)
- **Enhanced Synergy Scoring**: Improved draft prediction with better synergy detection
- **Color Pair Archetypes**: Automatic archetype detection based on top color pairs in your draft
- **Card Type Categorization**: Set guide uses card types for better organization
- **Keyword Extraction**: Sophisticated keyword analysis for card recommendations

### Enhanced Deck Builder (v1.4.1)
- **Undo/Redo Support**: Full undo/redo functionality with Ctrl+Z/Ctrl+Y keyboard shortcuts
- **Build Around Mode**: Generate complete decks around key cards with archetype selection (Aggro, Midrange, Control)
- **Iterative Building**: Add cards one at a time with live suggestions that update based on your choices
- **Quick Generate**: Instantly generate a complete 60-card deck with optimal land distribution
- **Budget Mode**: Filter suggestions to cards you already own
- **Score Breakdown**: See why cards are recommended (color fit, curve fit, synergy, card quality)

### Play Tracking & Analysis (v1.4.1)
- **In-Game Play Tracking**: Track every play in real-time during matches
- **Deck Permutation Tracking**: Track every modification to your decks
- **Deck Notes & Suggestions**: Post-match notes with improvement suggestions
- **Game Play Timeline**: Visual timeline in Match Details showing game progression
- **Match Comparison**: Compare matches side-by-side for performance analysis

### Standard Format Features (v1.4.1)
- **Standard Legality Validation**: Real-time legality checking for Standard format
- **Set Rotation Notifications**: Alerts for upcoming set rotations
- **Banned Cards Detection**: Legality banners and warnings for banned cards
- **Automatic Set Metadata Sync**: Sync set data from Scryfall on startup

### Data Integration (v1.4.1)
- **Card Price Integration**: Scryfall price data for collection and deck valuation
- **ChannelFireball Ratings**: CFB card ratings as secondary data source for recommendations
- **17Lands JSON Export**: Export drafts to 17Lands format for analysis
- **External Platform Export**: Export decks to Moxfield and Archidekt

### Advanced Draft Analytics (v1.4.1)
- **Drafting Pattern Analysis**: Analyze your color and card type preferences
- **Archetype Performance**: Track win rates by color pair and archetype
- **Temporal Trend Analysis**: Weekly/monthly performance trends with learning curve visualization
- **Community Comparison**: Compare your performance vs 17Lands community averages
- **Draft Deck Suggester**: Build decks by archetype (Aggro/Midrange/Control) with Arena export

### Synergy Data Sources (v1.4.1)
- **Card Embeddings**: Semantic similarity for better card recommendations
- **MTGZone Archetype Data**: Integrate archetype data into recommendations
- **EDHREC Integration**: Commander synergy data for card suggestions
- **Archidekt Co-occurrence**: Card co-occurrence analysis for synergy detection
- **Tribal Database**: Comprehensive creature type database for tribal synergies
- **Oracle Pattern Detection**: Expanded oracle text pattern matching

### In-App Documentation (v1.4.1)
- **Contextual Help Icons**: Click "?" icons throughout the app for detailed feature explanations
- **Enhanced Tooltips**: Hover over stats, badges, and buttons for quick help
- **ML Suggestions Help**: Learn how ML recommendations work and how to use confidence scores
- **Archetype Explanations**: Understand Aggro, Midrange, and Control playstyles
- **Play Pattern Insights**: Documentation for improvement suggestions based on your matches

### Settings Improvements (v1.3)
- **Collapsible Accordion Navigation**: Organized settings into collapsible sections
- **URL Hash Navigation**: Direct links to settings sections (e.g., `#connection`, `#17lands`)
- **Keyboard Navigation**: Full keyboard support for accessibility
- **LoadingButton Component**: Consistent loading states across all async operations

## Screenshots

### Match History
![Match History](docs/images/match-history.png)

### Draft History
![Draft History](docs/images/draft-history.png)

### Deck Management
![Decks](docs/images/decks.png)

### Collection Browser
![Collection](docs/images/collection.png)

### Meta Dashboard
![Meta Dashboard](docs/images/meta-dashboard.png)

### Charts & Analytics
![Deck Performance](docs/images/charts-deck-performance.png)

![Format Distribution](docs/images/charts-format-distribution.png)

> **Note**: Screenshots are generated automatically using Playwright. Run `npm run screenshots` from the project root to regenerate them with your local data.

## Documentation

📚 **[Complete Documentation Wiki →](https://github.com/RdHamilton/vault-mtg/wiki)**

- **[Installation Guide](https://github.com/RdHamilton/vault-mtg/wiki/Installation)** - Setup instructions for macOS and Windows
- **[Usage Guide](https://github.com/RdHamilton/vault-mtg/wiki/Usage-Guide)** - How to use all features
- **[CLI Commands](https://github.com/RdHamilton/vault-mtg/wiki/CLI-Commands)** - Complete command reference
- **[Configuration](https://github.com/RdHamilton/vault-mtg/wiki/Configuration)** - Configuration options
- **[Troubleshooting](https://github.com/RdHamilton/vault-mtg/wiki/Troubleshooting)** - Common issues and solutions

### Technical Documentation

- **[Architecture](docs/ARCHITECTURE.md)** - Service-based system design and architecture
- **[Deck Builder Guide](docs/DECK_BUILDER.md)** - Comprehensive deck builder documentation with API reference
- **[Daemon API](docs/DAEMON_API.md)** - WebSocket API reference for daemon integration
- **[Development Guide](docs/DEVELOPMENT.md)** - Development setup and contributing guidelines
- **[Migration Guide](docs/MIGRATION_TO_SERVICE_ARCHITECTURE.md)** - Upgrading to service-based architecture
- **[Daemon Installation](docs/DAEMON_INSTALLATION.md)** - Complete daemon service installation guide
- **[Database Schema](https://github.com/RdHamilton/vault-mtg/wiki/Database-Schema)** - Database structure

## Prerequisites

- **MTG Arena** must be installed and configured to enable detailed logging
- **Go 1.25+** (for building from source)
- **Ollama** (optional) - For AI-powered natural language explanations

## Ollama Setup (Optional)

VaultMTG can use [Ollama](https://ollama.ai/) to provide natural language explanations for card recommendations. This feature is completely optional - the app works fully without it.

### Installing Ollama

**macOS**:
```bash
brew install ollama
# Or download from https://ollama.ai/download
```

**Windows**:
Download the installer from https://ollama.ai/download

**Linux**:
```bash
curl -fsSL https://ollama.ai/install.sh | sh
```

### Starting Ollama

```bash
# Start Ollama server (runs on port 11434 by default)
ollama serve
```

### Configuring in VaultMTG

1. Open VaultMTG
2. Go to **Settings** → **ML/AI Settings**
3. Enable **Ollama Integration**
4. Configure:
   - **Ollama URL**: `http://localhost:11434` (default)
   - **Model**: `qwen3:8b` (recommended) or any compatible model
5. Click **Test Connection** to verify

The app will automatically pull the model if it's not already downloaded.

### Supported Models

Any Ollama model works, but these are recommended:
- `qwen3:8b` - Default, good balance of quality and speed
- `llama3.2:3b` - Faster, smaller, good for older hardware
- `mistral:7b` - Alternative with different response style

**Note**: Without Ollama, VaultMTG uses template-based explanations which work well for most use cases.

## Enabling Detailed Logging in MTG Arena

**IMPORTANT**: You must enable detailed logging in MTG Arena for this companion app to work properly.

### Steps to Enable Detailed Logging:

1. Launch **Magic: The Gathering Arena**
2. Click the **Adjust Options** gear icon ⚙️ at the top right of the home screen
3. In the Options menu, click **View Account**
4. Find and check the **Detailed Logs** checkbox (may also be labeled "Enable Detailed Logs" or "Plugin Support")
5. **Restart** MTG Arena for the changes to take effect

### Why Enable Detailed Logging?

Detailed logging allows MTG Arena to output game events and data in JSON format to the Player.log file. This enables companion applications like VaultMTG to:
- Track your game statistics
- Analyze your collection
- Display deck information
- Monitor game state in real-time

**Note**: Detailed logging has no impact on game performance and is specifically designed to support third-party companion tools.

## Installation

### Quick Start (Recommended)

Download the latest release from the [Releases page](https://github.com/RdHamilton/vault-mtg/releases):

#### macOS (Currently Supported)

1. Download `VaultMTG-vX.X.X-macOS.dmg`
2. Open the DMG and drag `VaultMTG.app` to your Applications folder
3. **First launch**: Right-click the app → "Open" (to bypass Gatekeeper)
4. The app will start the API server and open your default browser

**What happens on launch:**
- The app starts a local REST API server (port 8080)
- Your default browser opens to the VaultMTG UI
- The app monitors MTGA logs in the background via the daemon service

#### Windows / Linux

Windows and Linux builds are planned for future releases. Currently, you can build from source (see below).

### Daemon Mode (Recommended)

**What is Daemon Mode?**

VaultMTG can run as a background service (daemon) that continuously monitors your MTGA log file and provides data to the GUI. This is the **recommended setup** because:

✅ **Always Running** - Data collection continues even when GUI is closed
✅ **Auto-Start** - Daemon starts automatically on system boot
✅ **Reliable** - Automatic restart if it crashes
✅ **Cleaner** - Separation of data collection (daemon) and display (GUI)

**Platform Support Status:**

- ✅ **macOS**: Service installation fully tested and verified
- ⚠️ **Windows**: Service code implemented but not yet verified on Windows
- ⚠️ **Linux**: Service code implemented but not yet verified on Linux

> **Note**: The service installation code uses the cross-platform [kardianos/service](https://github.com/kardianos/service) library which supports Windows, macOS, and Linux. While the implementation is complete for all platforms, testing has only been performed on macOS. Windows and Linux service installation should work but has not been verified yet.

**Installation**:

1. Download and extract VaultMTG for your platform (see Quick Start above)

2. Install the daemon service:

   **macOS/Linux**:
   ```bash
   cd /path/to/vaultmtg
   ./vaultmtg service install
   ./vaultmtg service start
   ```

   **Windows (as Administrator)**:
   ```powershell
   cd C:\Path\To\VaultMTG
   .\vaultmtg.exe service install
   .\vaultmtg.exe service start
   ```

3. Verify daemon is running:
   ```bash
   ./vaultmtg service status
   ```

   Expected output:
   ```
   Service Status:
     Status: ✓ Running
   ```

4. Launch the GUI normally - it will automatically connect to the daemon

**Service Management**:

```bash
# Check status
./vaultmtg service status

# Start/Stop
./vaultmtg service start
./vaultmtg service stop

# Restart
./vaultmtg service restart

# Uninstall
./vaultmtg service uninstall
```

📚 **For detailed daemon installation and troubleshooting, see [docs/DAEMON_INSTALLATION.md](docs/DAEMON_INSTALLATION.md)**

**Alternative: Standalone Mode**

If you prefer not to use daemon mode, the GUI includes an embedded log poller that works standalone. Simply launch the app and it will monitor logs automatically.

### Build From Source

**Prerequisites**:
- [Go 1.25+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/) (for frontend)

**Clone and Build**:
```bash
# Clone repository
git clone https://github.com/RdHamilton/vault-mtg.git
cd vault-mtg

# Build the Go services (BFF + daemon + sync)
(cd services/bff && go build ./...)
(cd services/daemon && go build ./...)
(cd services/sync && go build ./...)

# Install and build frontend
cd frontend
npm install
npm run build
cd ..
```

**Development Mode** (with hot reload):
```bash
# Terminal 1: Start the BFF
cd services/bff && go run ./cmd

# Terminal 2: Start the daemon
cd services/daemon && go run ./cmd/daemon

# Terminal 3: Start frontend dev server
cd frontend
npm run dev
```

The frontend dev server runs at `http://localhost:3000` and proxies API requests to the backend at `http://localhost:8080`.

## Player.log File Locations

The application automatically detects the Player.log location based on your platform:

### macOS
```
~/Library/Application Support/com.wizards.mtga/Logs/Logs/Player.log
```

**Tip**: If you can't see your Library folder, press `Command + Shift + .` (dot) to show hidden files in Finder.

### Windows
```
C:\Users\{username}\AppData\LocalLow\Wizards Of The Coast\MTGA\Player.log
```

**Tip**: You can paste this path directly into Windows Explorer's address bar (replace `{username}` with your Windows username).

### Previous Session Logs

MTGA also saves the previous session's log as `Player-prev.log` in the same directory, which can be useful for reviewing past games.

### Log File Rotation

MTGA may rotate log files during long gaming sessions when the log becomes large. VaultMTG automatically handles log rotation:

- **Detection**: Monitors for file size decreases, file removal/rename events (via fsnotify)
- **Recovery**: Automatically reopens the new log file and continues monitoring
- **State Preservation**: Maintains draft state and game tracking across rotation events
- **Logging**: Rotation events are logged with `[INFO]` messages for visibility

**Rotation scenarios handled:**
- Size-based rotation (when Player.log exceeds MTGA's size limit)
- File removal and recreation
- Manual log deletion/archival

The overlay and tracking features continue working seamlessly during and after log rotation.

## Usage

### GUI Application

Launch the VaultMTG desktop app:

**Windows**: Double-click `VaultMTG.exe`
**macOS**: Double-click `VaultMTG.app` from Applications
**Linux**: Run `./VaultMTG-linux-amd64`

The application will:
1. Automatically locate your MTGA Player.log file
2. Initialize the database (first run creates `~/.vaultmtg/data.db`)
3. Start monitoring the log file for new matches
4. Display your statistics and match history in real-time

### Navigation

- **Match History**: View and filter all your matches
- **Draft**: Real-time draft assistant with recommendations, synergy detection, and format insights
- **Decks**: Deck library with builder, import/export, and AI recommendations (v1.3)
- **Collection**: Browse and track your card collection (v1.3.1)
- **Meta**: Metagame dashboard with archetype tier lists and tournament data (v1.4)
- **Charts**: Visualize your performance data
  - Win Rate Trend: Performance over time
  - Deck Performance: Win rates by deck
  - Rank Progression: Track your ladder climbing
  - Format Distribution: Play patterns across formats
  - Result Breakdown: Detailed statistics
- **Settings**: Configure database path, ML settings, and Ollama integration

### Real-Time Updates

While MTGA is running and you're playing games:
- New matches are automatically detected and added
- Statistics update in real-time
- Footer shows at-a-glance stats (total matches, win rate, streak)
- Toast notifications confirm when data is updated

### CLI Mode (Advanced)

The CLI is still available for automation and advanced users:

```bash
# Read log and display basic info
./vaultmtg read

# Export statistics
./vaultmtg export stats -json

# Run draft overlay
./vaultmtg -draft-overlay-mode
```

See the [CLI Commands Wiki](https://github.com/RdHamilton/vault-mtg/wiki/CLI-Commands) for complete reference.

## Development

### Project Structure

```
vault-mtg/
├── cmd/                     # Application entry points
│   ├── apiserver/          # REST API server (v1.4+)
│   └── vaultmtg/           # CLI daemon for log monitoring
├── frontend/                # React + TypeScript SPA
│   ├── src/
│   │   ├── components/     # Reusable UI components
│   │   ├── pages/          # Page components (routes)
│   │   ├── services/api/   # REST API client modules
│   │   ├── App.tsx         # Root component
│   │   └── main.tsx        # Frontend entry point
│   ├── package.json
│   └── vite.config.ts
├── internal/                # Private application code
│   ├── api/                # REST API handlers & router (v1.4+)
│   │   ├── handlers/      # HTTP request handlers
│   │   ├── websocket/     # WebSocket for real-time updates
│   │   └── router.go      # API route definitions
│   ├── gui/                # Facade layer (business logic)
│   ├── ml/                 # Machine learning engine (v1.4+)
│   ├── llm/                # Ollama LLM client (v1.4+)
│   ├── meta/               # Metagame data service (v1.4+)
│   ├── daemon/             # Flight recorder & daemon services (v1.4.1+)
│   ├── mtga/               # MTGA-specific logic
│   │   ├── logreader/     # Log parsing
│   │   ├── draft/         # Draft overlay
│   │   │   └── analytics/ # Draft analytics services (v1.4.1+)
│   │   └── recommendations/ # Card recommendations
├── benchmarks/              # Performance benchmarks (v1.4.1+)
│   └── storage/            # Database and persistence
│       ├── models/        # Data models
│       └── repository/    # Data access layer
├── docs/                    # Documentation
├── scripts/                 # Development scripts
└── CLAUDE.md                # AI assistant guidance
```

### Development Workflow

**Full Stack Development** (recommended):
```bash
# Terminal 1: Start BFF (cloud data, port 8080)
cd services/bff && go run ./cmd

# Terminal 2: Start daemon (live MTGA log reader, port 9001)
cd services/daemon && go run ./cmd/daemon

# Terminal 3: Start frontend dev server
cd frontend && npm run dev

# Open browser to http://localhost:3000
```

**Go Development** (backend):
```bash
# Format code (per-service since each is its own module)
gofumpt -w services/ pkg/

# Run linter
golangci-lint run --timeout=5m

# Run tests with race detection (per service)
(cd services/bff && go test -race ./...)
(cd services/daemon && go test -race ./...)
(cd services/sync && go test -race ./...)
(cd pkg/draftalgo && go test -race ./...)

# Build all workspace services
./scripts/dev.sh build
```

**Frontend Development**:
```bash
# Install dependencies
cd frontend
npm install

# Run frontend dev server
npm run dev

# Build frontend for production
npm run build

# Type checking
npm run tsc

# Linting
npm run lint

# Run tests
npm run test:run
```

### Running Tests

**Go Tests**:
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...
```

**Frontend Tests** (when added):
```bash
cd frontend
npm test
```

## Troubleshooting

### "Player.log not found!"

If you see this error:
1. Verify MTG Arena is installed
2. Ensure you've enabled detailed logging (see instructions above)
3. Run MTG Arena at least once after enabling detailed logging
4. Check that the log file exists at the expected location for your platform

### macOS: Cannot Find Library Folder

Press `Command + Shift + .` in Finder to show hidden files and folders.

### Windows: Access Denied

Ensure you have read permissions for the MTGA log directory. Try running as administrator if needed.

## Technology Stack

VaultMTG is built with modern technologies for performance and cross-platform compatibility:

### Architecture (v1.4+)

- **REST API + Browser SPA** - Decoupled architecture for flexibility
  - Go REST API server with WebSocket support
  - React SPA served via Vite or static files
  - Opens in your default browser - no native app required

### Backend (Go)

- **[Go 1.25+](https://go.dev/)** - Programming language
- **[Chi Router](https://github.com/go-chi/chi)** - Lightweight HTTP router (BFF)
- **[PostgreSQL](https://www.postgresql.org/) / [pgx](https://github.com/jackc/pgx)** - Per-account cloud storage on the BFF
- **[golang-migrate/migrate](https://github.com/golang-migrate/migrate)** - Database migration management
- **[fsnotify](https://github.com/fsnotify/fsnotify)** - Cross-platform file system notifications (daemon log poller)

### Frontend (React + TypeScript)

- **[React 18](https://react.dev/)** - UI library with hooks
- **[TypeScript](https://www.typescriptlang.org/)** - Type-safe JavaScript
- **[React Router](https://reactrouter.com/)** - Client-side routing
- **[Recharts](https://recharts.org/)** - Data visualization and charting library
- **[Vite](https://vite.dev/)** - Fast build tool and dev server
- **[Vitest](https://vitest.dev/)** - Unit testing framework
- **[Playwright](https://playwright.dev/)** - E2E testing framework

### ML/AI Features (v1.4+)

- **Machine Learning Engine** - Custom ML model for card recommendations
- **[Ollama](https://ollama.ai/)** - Local LLM integration for natural language explanations

### Data Sources

- **[17Lands](https://www.17lands.com/)** - Draft statistics and card ratings
- **[Scryfall](https://scryfall.com/)** - Card metadata, images, and price data (v1.4.1)
- **[MTGGoldfish](https://www.mtggoldfish.com/)** - Metagame data
- **[MTGTop8](https://www.mtgtop8.com/)** - Tournament results
- **[ChannelFireball](https://www.channelfireball.com/)** (v1.4.1) - Card ratings and analysis
- **[EDHREC](https://edhrec.com/)** (v1.4.1) - Commander synergy data
- **[Archidekt](https://archidekt.com/)** (v1.4.1) - Deck co-occurrence analysis
- **[MTGZone](https://mtgazone.com/)** (v1.4.1) - Archetype data and analysis
- **[What's in Standard](https://whatsinstandard.com/)** (v1.4.1) - Standard legality and rotation data

For a complete list of dependencies, see [`go.mod`](go.mod) and [`frontend/package.json`](frontend/package.json).

## Deployment

For the full deploy model, infrastructure inventory, SSM parameters, and rollback steps, see [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md).

### Frontend Serving Model

Production traffic for `https://app.vaultmtg.app` is served from **S3 + CloudFront**. Each frontend property (`app.vaultmtg.app`, `vaultmtg.app`, `rhamiltoneng.com`) has its own S3 bucket, CloudFront distribution, and ACM certificate. Bucket names and distribution IDs are read from SSM at deploy time.

**Vercel** is wired up for **PR preview deploys only**. Production tags (`v*`) skip Vercel via the `vercel.json` `ignoreCommand`. Vercel does not serve any production hostname.

**EC2 nginx** serves the BFF/API on `api.vaultmtg.app` only. There is no `location /` static-serve block on EC2 in production — frontends are served by CloudFront, not by the instance.

See [ADR-008: Frontend Serving Model — S3+CloudFront Canonical, Vercel Preview-Only](docs/adr/ADR-008-frontend-serving-model.md) for the full decision record and rationale. ADR-008 supersedes ADR-001 (EC2 nginx canonical) and ADR-007 (Vercel canonical). See [ADR-006](docs/adr/006-vercel-bff-connectivity.md) for cross-origin BFF connectivity details.

### Backend (BFF)

The Go BFF runs on a single EC2 instance behind nginx, which proxies `/api/v1/` to the BFF on `127.0.0.1:8080`. The BFF reads its config (`DATABASE_URL`, `ALLOWED_ORIGINS`, `JWT_SECRET`, `DAEMON_JWT_SECRET`) from SSM Parameter Store at startup. The daemon binary ships via GitHub Releases for Windows (amd64) and macOS (arm64/amd64). See [Daemon Installation](docs/DAEMON_INSTALLATION.md) for setup instructions.

## Contributing

Contributions are welcome! Please ensure:
- Code follows Go best practices (see `CLAUDE.md`)
- All tests pass (`./scripts/test.sh`)
- Code is formatted (`./scripts/dev.sh fmt`)

See the [Development Guide](https://github.com/RdHamilton/vault-mtg/wiki/Development) for detailed contribution guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Disclaimer

VaultMTG is not affiliated with, endorsed by, or sponsored by Wizards of the Coast. Magic: The Gathering Arena and its associated trademarks are property of Wizards of the Coast LLC.

## Acknowledgments

- Wizards of the Coast for MTG Arena and its detailed logging support
- The MTGA community for documentation on log formats and structure

## CLI Flag Migration (v0.2.0)

As of v0.2.0, CLI flags have been standardized for consistency. Old flags are still supported but deprecated.

### Quick Reference

| Old Flag (Deprecated) | New Flag | Shorthand |
|-----------------------|----------|-----------|
| `-gui` | `-gui-mode` | `-g` |
| `-debug` | `-debug-mode` | `-d` |
| `-cache` | `-cache-enabled` | |
| `-poll-interval` | `-log-poll-interval` | |
| `-use-file-events` | `-log-use-fsnotify` | |
| `-draft-overlay` | `-draft-overlay-mode` | |
| `-set-file` | `-overlay-set-file` | |
| `-log-path` | `-log-file-path` | |
| `-overlay-set` | `-overlay-set-code` | |
| `-overlay-lookback` | `-overlay-lookback-hours` | |

**Note:** Deprecated flags will show a warning and will be removed in v2.0.0. See `FLAG_MIGRATION.md` for complete details.

### Examples

```bash
# Old syntax (still works, shows warning)
./bin/vaultmtg -debug -gui

# New syntax (recommended)
./bin/vaultmtg -debug-mode -gui-mode

# New syntax with shortcuts
./bin/vaultmtg -d -g
```

