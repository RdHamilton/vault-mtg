# Frontend Agent Changelog

<!-- Entries are appended newest-first. Format:
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file.tsx` — short description
**Summary**: One sentence summary of what was done and why.
-->

## 2026-05-11 — Bug fix: daemon download page (no issue)
**PR**: #1810
**Files changed**:
- `frontend/src/components/DaemonDownload.tsx` — updated artifact names to match actual GoReleaser output (vaultmtg-daemon-* prefix, darwin-universal replaces arm64/amd64 split); collapsed to 2 download options
- `frontend/src/components/DaemonDownload.test.tsx` — updated assertions for new artifact names and 2-option list
- `frontend/src/pages/Setup.tsx` — added target="_blank" rel="noopener noreferrer" to download page link; replaced Gatekeeper bypass steps with Apple notarization messaging; removed unused SVG imports
- `frontend/src/pages/Setup.test.tsx` — updated assertions for new-tab link and signed-software content
- `frontend/src/components/OnboardingModal.tsx` — Step 2 macOS/Windows install instructions updated to reflect signed software (no bypass needed)
- `frontend/src/components/OnboardingModal.test.tsx` — updated macOS/Windows install text assertions
**Summary**: Fixed three staging bugs: wrong download artifact names causing 404s, download page link opening in same tab, and Gatekeeper/SmartScreen bypass instructions that are no longer correct now that both macOS and Windows installers are signed.

## 2026-05-10 — Issues #1697, #1698, #1699, #1700: Wave 5 — daemon empty states + onboarding analytics
**PR**: #1723
**Files changed**:
- `frontend/src/components/DaemonEmptyState.tsx` — new component: wraps EmptyState with /setup CTA and PostHog error_empty_state_shown event
- `frontend/src/components/DaemonEmptyState.test.tsx` — 8 component tests
- `frontend/src/hooks/useDaemonStatus.ts` — new hook: polls BFF /health/daemon every 30s, returns daemonConnected+daemonChecked
- `frontend/src/hooks/useDaemonStatus.test.ts` — 5 hook tests
- `frontend/tests/e2e/daemon-empty-states.spec.ts` — 11 Playwright E2E tests for no-daemon empty state flow
- `frontend/src/pages/MatchHistory.tsx` — integrated useDaemonStatus; shows DaemonEmptyState when not connected; fires funnel_daemon_installed + funnel_first_game_played
- `frontend/src/pages/Collection.tsx` — integrated useDaemonStatus; shows DaemonEmptyState when not connected
- `frontend/src/pages/Decks.tsx` — integrated useDaemonStatus; shows DaemonEmptyState when not connected
- `frontend/src/services/analytics.ts` — added FUNNEL_DAEMON_INSTALLED and FUNNEL_FIRST_GAME_PLAYED to Events taxonomy and AnalyticsEvent union
- `frontend/src/test/setup.ts` — globally mock useDaemonStatus (connected=true) and getDaemonHealth so existing page tests are unaffected
**Summary**: Implemented Wave 5 first-run empty states for Match History, Collection, and Decks when the local daemon is not connected; added two new PostHog funnel events (funnel_daemon_installed, funnel_first_game_played) that fire at the correct activation-funnel checkpoints.

## 2026-05-07 — Fix PR #1464 playwright path + PR #1463 analytics module

**PR**: #1464 (path fix pushed), #1463 (analytics pushed)
**Files changed**:
- `frontend/playwright.config.ts` — fixed `../../` → `../` for bin and go run paths in webServer config
- `frontend/src/services/analytics.ts` — new PostHog analytics module with guarded init, event taxonomy constants, captureEvent/identifyUser/resetIdentity/registerSuperProperties
- `frontend/src/hooks/usePostHogIdentity.ts` — hook identifying Clerk user with PostHog, fires funnel_sign_up_completed once per session
- `frontend/src/services/__tests__/analytics.test.ts` — 8 tests covering init guard, captureEvent, identifyUser, resetIdentity, registerSuperProperties, Events constants
- `frontend/.env.example` — added VITE_POSTHOG_KEY and VITE_POSTHOG_HOST vars
- `frontend/package.json` — added posthog-js dependency
**Summary**: Fixed playwright webServer path bug that would cause CI to fail, and implemented the missing PostHog analytics module (posthog-js) and identity hook that were never pushed to the #1463 branch.

## 2026-05-06 — Issue #1397: feat(frontend): EmptyState component — heading/subtext/variant/CTA API
**PR**: #1413
**Files changed**:
- `frontend/src/components/EmptyState.tsx` — rebuilt with new props: heading, subtext, ctaLabel?, ctaHref?, variant (no-data | coming-soon)
- `frontend/src/components/EmptyState.css` — added heading/subtext/cta/variant CSS classes
- `frontend/src/components/EmptyState.test.tsx` — 21 new tests covering all ticket ACs
- `frontend/src/pages/MatchHistory.tsx` — migrated to new EmptyState API
- `frontend/src/pages/Decks.tsx` — replaced inline empty-state div with EmptyState component
- `frontend/src/pages/Draft.tsx` — replaced inline draft-empty div with EmptyState component
- `frontend/src/pages/Quests.tsx` — migrated 4 EmptyState call sites to new API
- `frontend/src/pages/WinRateTrend.tsx` — migrated EmptyState to new API
- `frontend/src/pages/DeckPerformance.tsx` — migrated EmptyState to new API
- `frontend/src/pages/FormatDistribution.tsx` — migrated EmptyState to new API
- `frontend/src/pages/RankProgression.tsx` — migrated EmptyState to new API
- `frontend/src/pages/ResultBreakdown.tsx` — migrated EmptyState to new API
**Summary**: Rebuilt EmptyState with ticket-specified API (heading, subtext, variant, CTA) and migrated all 9 existing call sites; loading states still show spinner, not EmptyState; 2561 tests pass.

## 2026-05-04 — Issue #1136 #1142: fix(frontend): add BFF draft-ratings and API key adapters
**PR**: #1177
**Files changed**:
- `frontend/src/services/api/bffDraftRatings.ts` — new adapter: getDraftRatings() targeting GET /api/v1/draft-ratings/{setCode}/{format} with cache-degraded header support
- `frontend/src/services/api/bffDraftRatings.test.ts` — 10 MSW tests covering URL, response shape, header parsing, URL encoding, error handling
- `frontend/src/services/api/bffAuth.ts` — new adapter: createAPIKey() targeting POST /api/keys with daemon JWT auth
- `frontend/src/services/api/bffAuth.test.ts` — 9 MSW tests covering URL, Authorization header, response shape, error handling
- `frontend/src/services/api/index.ts` — exported both new modules and their TypeScript types
**Summary**: Added two BFF-only adapter modules for the draft-ratings and API key endpoints; both use direct fetch (not apiClient wrappers) because the BFF returns raw JSON rather than the data-wrapped envelope shape.

## 2026-05-04 — Issue #1139: feat(frontend): add Authorization header to all BFF requests
**PR**: #1150
**Files changed**:
- `frontend/src/adapters/` — added Authorization header injection to all BFF fetch calls via the REST API adapter layer
**Summary**: Wired the auth token into every outbound BFF request so authenticated endpoints receive the Authorization header; implemented at the adapter layer to keep components free of auth concerns.
