target: front-engineer
---
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
