# VaultMTG Documentation

This directory contains all technical documentation for the VaultMTG project.

## 📚 Documentation Index

### Architecture & Design
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture overview and design patterns
- **[ARCHITECTURE_DECISIONS.md](ARCHITECTURE_DECISIONS.md)** - Architectural Decision Records (ADRs)
- **[MIGRATION_TO_SERVICE_ARCHITECTURE.md](MIGRATION_TO_SERVICE_ARCHITECTURE.md)** - Service architecture migration guide

### Architectural Decision Records (ADRs)
- **[ADR-008: Frontend Serving Model — S3+CloudFront Canonical, Vercel Preview-Only](adr/ADR-008-frontend-serving-model.md)** - Current source of truth: S3+CloudFront for all three frontend properties; Vercel is preview-only; EC2 nginx serves the API only
- **[ADR-007: Frontend Serving Model](adr/007-frontend-serving-model.md)** - Superseded by ADR-008. Previously declared Vercel canonical; kept for historical context
- **[ADR-006: Vercel BFF Connectivity](adr/006-vercel-bff-connectivity.md)** - How frontend SPAs connect to the Go BFF over CORS

### Deployment & Operations
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Production deploy model, SSM parameter inventory, rollback procedure, and CI workflow reference

### Development Guides
- **[DEVELOPMENT.md](DEVELOPMENT.md)** - Development setup and workflow
- **[DEVELOPMENT_STATUS.md](DEVELOPMENT_STATUS.md)** - Current development status and progress tracking
- **[TESTING.md](TESTING.md)** - Testing guide with component, integration, and E2E tests
- **[CLAUDE_CODE_GUIDE.md](CLAUDE_CODE_GUIDE.md)** - Guidelines for AI-assisted development (Claude Code)

### User Support Guides
- **[daemon-installation.md](support/daemon-installation.md)** - How to install the VaultMTG daemon (macOS and Windows)
- **[daemon-troubleshooting.md](support/daemon-troubleshooting.md)** - Daemon troubleshooting steps
- **[daemon-uninstall.md](support/daemon-uninstall.md)** - How to uninstall the daemon
- **[draft-live-view.md](support/draft-live-view.md)** - User guide for the live draft view (/draft/live)
- **[faq.md](support/faq.md)** - Frequently asked questions

### Feature Documentation
- **[COLLECTION.md](COLLECTION.md)** - Collection tracking, set completion, and missing cards analysis
- **[DECK_BUILDER.md](DECK_BUILDER.md)** - Comprehensive deck builder documentation with API reference

### Technical Specifications
- **[DAEMON_API.md](DAEMON_API.md)** - Daemon WebSocket API specification
- **[DAEMON_INSTALLATION.md](DAEMON_INSTALLATION.md)** - Daemon installation and configuration
- **[MTGA_LOG_EVENTS.md](MTGA_LOG_EVENTS.md)** - MTGA log event types and structures
- **[MTGA_LOG_RESEARCH.md](MTGA_LOG_RESEARCH.md)** - Research notes on MTGA log parsing
- **[MTGA_LOG_EVENT_ANALYSIS_UPDATED.md](MTGA_LOG_EVENT_ANALYSIS_UPDATED.md)** - Updated log event analysis

### UI & Design
- **[DRAFT_UI_REORGANIZATION.md](DRAFT_UI_REORGANIZATION.md)** - Draft UI structure and organization
- **[GUI_DESIGN_TEMPLATE.md](GUI_DESIGN_TEMPLATE.md)** - GUI design guidelines and templates

### Database & Migration
- **[backup.md](backup.md)** - Database backup procedures
- **[FLAG_MIGRATION.md](FLAG_MIGRATION.md)** - Feature flag migration guide

## 📖 Project Root Documentation

The following documentation files are kept in the project root for GitHub integration and discoverability:

- **[README.md](../README.md)** - Project overview and quick start
- **[CHANGELOG.md](../CHANGELOG.md)** - Version history and release notes
- **[CONTRIBUTING.md](../CONTRIBUTING.md)** - Contribution guidelines
- **[CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md)** - Community code of conduct
- **[SECURITY.md](../SECURITY.md)** - Security policies and vulnerability reporting

## 🔍 Finding Documentation

### By Topic
- **Getting Started**: [README.md](../README.md), [DEVELOPMENT.md](DEVELOPMENT.md)
- **Architecture**: [ARCHITECTURE.md](ARCHITECTURE.md), [ARCHITECTURE_DECISIONS.md](ARCHITECTURE_DECISIONS.md)
- **MTGA Integration**: [MTGA_LOG_EVENTS.md](MTGA_LOG_EVENTS.md), [MTGA_LOG_RESEARCH.md](MTGA_LOG_RESEARCH.md)
- **Daemon**: [DAEMON_API.md](DAEMON_API.md), [DAEMON_INSTALLATION.md](DAEMON_INSTALLATION.md)
- **Collection**: [COLLECTION.md](COLLECTION.md)
- **Deck Builder**: [DECK_BUILDER.md](DECK_BUILDER.md)
- **UI Development**: [DRAFT_UI_REORGANIZATION.md](DRAFT_UI_REORGANIZATION.md), [GUI_DESIGN_TEMPLATE.md](GUI_DESIGN_TEMPLATE.md)
- **Testing**: [TESTING.md](TESTING.md)
- **Database**: [backup.md](backup.md), [FLAG_MIGRATION.md](FLAG_MIGRATION.md)

### For Developers
- Start with: [DEVELOPMENT.md](DEVELOPMENT.md)
- Understand architecture: [ARCHITECTURE.md](ARCHITECTURE.md)
- Review decisions: [ARCHITECTURE_DECISIONS.md](ARCHITECTURE_DECISIONS.md)
- Check current status: [DEVELOPMENT_STATUS.md](DEVELOPMENT_STATUS.md)

### For AI-Assisted Development
- Claude Code users: See [CLAUDE_CODE_GUIDE.md](CLAUDE_CODE_GUIDE.md)
- **Note**: CLAUDE.md in the project root is .gitignored and not tracked in version control

## 📝 Documentation Standards

All technical documentation should:
- Be placed in the `docs/` directory (unless required in root for GitHub)
- Use Markdown format
- Include a clear title and purpose
- Be kept up-to-date with code changes
- Link to related documentation
- Include examples where appropriate

## 🔄 Updating Documentation

When making changes:
1. Update relevant documentation alongside code changes
2. Add ADRs for architectural decisions to [ARCHITECTURE_DECISIONS.md](ARCHITECTURE_DECISIONS.md)
3. Update [DEVELOPMENT_STATUS.md](DEVELOPMENT_STATUS.md) with progress
4. Keep this index up-to-date when adding new documentation files
