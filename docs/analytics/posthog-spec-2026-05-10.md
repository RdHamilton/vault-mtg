# PostHog Spec — Beta Launch Readiness

**Owner**: Product Manager (Najah)
**Audience**: Business Analyst (executes via PostHog API)
**Date**: 2026-05-10
**Beta launch**: 2026-08-18
**Status**: Ready for BA execution

---

## Executive summary

Audited the codebase (`frontend/src/services/analytics.ts`, all `trackEvent` call sites, `useFeatureFlag` usage), v0.3.1 kickoff/PRD docs, and existing analytics docs. Findings:

- **Typed event taxonomy in code defines 31 event names**; only 9 are actually instrumented. 22 events exist as type definitions but never fire.
- **One critical event referenced in v0.3.1 PRD/kickoff (`daemon_paired`) is NOT in the typed taxonomy and NOT instrumented anywhere.** It is named as the v0.3.1 primary success metric and Wave 7 release-tag exit gate. This is a beta-blocker if not closed.
- **Two feature flags are documented**: `daemon_download_enabled` (created at 0%) and `beta_invite_only` (flag ID 669797, created 2026-05-08, 0% rollout, no FE/BFF integration yet).
- BA must (a) ensure all 32 events listed in Section A exist in PostHog with matching property schemas, (b) ensure all flags in Section B exist with the rollout config specified.

---

## Section A — Events to verify / create in PostHog

Property names use `snake_case`. Boolean = `is_*` prefix. IDs = `*_id` suffix. Timestamps captured natively by PostHog — do not include.

Global super-properties registered on every event (via `posthog.register`): `app_version`, `is_signed_in`, `daemon_status` (`connected`|`disconnected`|`reconnecting`|`unknown`), `platform` (`desktop`|`web`).

### A1. Activation funnel — CRITICAL (defines D1/D7)

| # | Event name | Code status | Trigger | Properties |
|---|---|---|---|---|
| 1 | `funnel_landing_page_viewed` | NOT INSTRUMENTED (typed) | Anonymous user loads vaultmtg.app | `referrer` (string), `utm_source` (string), `utm_medium` (string), `utm_campaign` (string) |
| 2 | `funnel_sign_up_started` | NOT INSTRUMENTED (typed) | User clicks "Sign up" / "Get started" CTA | `entry_point` (`landing_page`\|`auth_bar`\|`protected_route_redirect`) |
| 3 | `funnel_sign_up_completed` | INSTRUMENTED in `usePostHogIdentity.ts` | Clerk first-render where `isSignedIn` becomes true (session-keyed) | `auth_method` (`email`\|`google`\|`apple`\|`facebook`), `user_id` (string) |
| 4 | `funnel_daemon_download_started` | INSTRUMENTED in `DaemonDownload.tsx` and `OnboardingModal.tsx` | User clicks download button | `os` (string), `download_source` (`download_page`\|`prompt_modal`\|`onboarding_modal`) |
| 5 | `funnel_daemon_installed` | INSTRUMENTED in `MatchHistory.tsx` (proxy fire) | First time user lands on a daemon-dependent page after install | `daemon_version` (string, optional), `source` (string, optional) |
| 6 | **`daemon_paired`** | **NOT IN TAXONOMY, NOT INSTRUMENTED** — v0.3.1 PRIMARY METRIC | Daemon completes PKCE pairing and writes API key to OS keychain (fired client-side from daemon or from BFF after `/v1/daemon/register` 200) | `device_id` (UUID), `platform` (`darwin`\|`windows`\|`linux`), `daemon_ver` (semver string), `account_id` (string), `time_since_signup_seconds` (number, optional) |
| 7 | `funnel_daemon_connected` | INSTRUMENTED in `OnboardingModal.tsx`, `DaemonHealthIndicator.tsx` | WebSocket transitions to `connected` for the first time | `time_since_signup_seconds` (number, optional), `source` (string, optional) |
| 8 | `funnel_first_game_played` | INSTRUMENTED in `MatchHistory.tsx` | First match row renders for the user | `format` (string, optional), `source` (string, optional) |
| 9 | `funnel_first_data_loaded` | INSTRUMENTED in `BffMatchHistory.tsx` | First non-empty match list returned from BFF | `match_count` (number) |
| 10 | `funnel_first_feature_used` | NOT INSTRUMENTED (typed) | First non-match-history page navigation | `feature` (`draft`\|`draft_analytics`\|`decks`\|`collection`\|`meta`\|`charts`\|`quests`) |

**Critical action for BA**: `daemon_paired` is the v0.3.1 primary success metric (≥80% of installers paired within 10 minutes) and a release-tag exit gate. The event must exist in PostHog before the v0.3.1 release tag is cut. If instrumentation is also missing in code, file a ticket through project-manager — do not wait.

### A2. Page navigation

| # | Event name | Code status | Trigger | Properties |
|---|---|---|---|---|
| 11 | `page_viewed` | NOT INSTRUMENTED (typed) | React Router route change | `page` (slug), `previous_page` (slug or null) |

Route slug map (BA: confirm these are documented in PostHog descriptions): `match_history`, `quests`, `draft_advisor`, `draft_analytics`, `decks`, `deck_builder`, `collection`, `meta`, `chart_win_rate`, `chart_deck_performance`, `chart_rank_progression`, `chart_format_distribution`, `chart_result_breakdown`, `settings`, `bff_match_history`, `bff_draft_history`, `download`.

### A3. Feature usage

| # | Event name | Code status | Trigger | Properties |
|---|---|---|---|---|
| 12 | `feature_match_history_filtered` | NOT INSTRUMENTED (typed) | User applies filter on match history | `filter_type` (`format`\|`deck`\|`date_range`\|`result`), `filter_value` (string) |
| 13 | `feature_match_details_opened` | NOT INSTRUMENTED (typed) | Match details modal opens | `match_result` (`win`\|`loss`\|`draw`), `format` (string) |
| 14 | `feature_draft_advisor_pick_viewed` | NOT INSTRUMENTED (typed) | Live draft pack rendered (once per pack) | `set_code` (string), `pack_number` (1-3), `pick_number` (number) |
| 15 | `feature_draft_analytics_viewed` | NOT INSTRUMENTED (typed) | `/draft-analytics` with non-empty data | `draft_count` (number) |
| 16 | `feature_deck_builder_opened` | NOT INSTRUMENTED (typed) | `/deck-builder/:id` navigated | `entry_point` (`decks_list`\|`draft_build_around`\|`direct_link`) |
| 17 | `feature_deck_build_around_started` | NOT INSTRUMENTED (typed) | BuildAroundSeedModal opens | `seed_type` (`card`\|`archetype`\|`color_pair`) |
| 18 | `feature_collection_viewed` | NOT INSTRUMENTED (typed) | `/collection` with non-empty data | `card_count` (number) |
| 19 | `feature_meta_viewed` | NOT INSTRUMENTED (typed) | `/meta` data loaded | none |
| 20 | `feature_ml_suggestions_viewed` | NOT INSTRUMENTED (typed) | MLSuggestionsPanel renders with ≥1 suggestion | `suggestion_count` (number), `context` (`deck_builder`\|`draft`) |
| 21 | `feature_chart_interacted` | NOT INSTRUMENTED (typed) | Filter / setting changed on a `/charts/` page | `chart` (slug), `interaction` (`filter_applied`\|`time_range_changed`\|`format_changed`) |
| 22 | `feature_opponent_analysis_viewed` | NOT INSTRUMENTED (typed) | OpponentAnalysisPanel renders with data | `opponent_match_count` (number) |
| 23 | `feature_community_comparison_viewed` | NOT INSTRUMENTED (typed) | CommunityComparison renders with data | none |
| 24 | `feature_settings_changed` | NOT INSTRUMENTED (typed) | User saves a setting change | `setting_section` (`daemon_connection`\|`preferences`\|`display`), `setting_key` (string) |
| 25 | `feature_replay_started` | NOT INSTRUMENTED (typed) | `replay:started` WS event received | none |
| 26 | `feature_replay_completed` | NOT INSTRUMENTED (typed) | `replay:completed` WS event received | none |

### A4. Errors / friction

| # | Event name | Code status | Trigger | Properties |
|---|---|---|---|---|
| 27 | `error_daemon_connection_failed` | INSTRUMENTED in `DaemonHealthIndicator.tsx` (`fetchHealth` transition guard) | Daemon transitions to `error`/`disconnected` after being `connected` or `reconnecting` | `previous_status` (`connected`\|`reconnecting`), `duration_connected_seconds` (number) |
| 28 | `error_daemon_never_connected` | INSTRUMENTED in `OnboardingModal.tsx` | Signed in >5min, daemon still disconnected | `time_since_signin_seconds` (number, optional), `source` (string, optional) |
| 29 | `error_data_load_failed` | INSTRUMENTED in `apiClient.ts` (`request()` non-2xx branch); throttled 1 per (endpoint, status\_code) per 10s; skippable via `skipErrorAnalytics: true` for background pollers | Any non-2xx API response that surfaces an error UI | `page` (slug), `endpoint` (path, no IDs), `status_code` (number) |
| 30 | `error_auth_failed` | INSTRUMENTED in `apiClient.ts` (`getClerkToken` + `authHeaders` catch) | Clerk token provider throws during an API call | `reason_class` (`network`\|`invalid_credentials`\|`rate_limited`) — **NOTE: `context: string` replaced by `reason_class` enum per Ray Q2 amendment in #1839; only `network` emitted this PR** |
| 31 | `error_empty_state_shown` | INSTRUMENTED in `DaemonEmptyState.tsx` | Empty state component renders (no data, no error) | `page` (slug) |

### A5. Engagement / lifecycle

| # | Event name | Code status | Trigger | Properties |
|---|---|---|---|---|
| 32 | `app_session_started` | NOT INSTRUMENTED (typed) | `initializeServices()` resolves once on app boot | `services_init_ms` (number) |
| 33 | `app_user_identified` | NOT INSTRUMENTED (typed — but `identifyUser` IS called in `usePostHogIdentity.ts`) | Clerk `isSignedIn` flips false→true | `user_id` (string), `auth_method` (`email`\|`google`\|`apple`\|`facebook`) |
| 34 | `app_user_signed_out` | NOT INSTRUMENTED (typed) | Clerk sign-out completes; immediately `posthog.reset()` | none |

### A6. Setup page (added by Wave 6)

| # | Event name | Code status | Trigger | Properties |
|---|---|---|---|---|
| 35 | `setup_page_viewed` | INSTRUMENTED in `Setup.tsx` | `/setup` mounts | `platform` (`macos`\|`windows`\|`unknown`) |
| 36 | `setup_pairing_success` | INSTRUMENTED in `Setup.tsx` | Pairing completes successfully | `platform` (`macos`\|`windows`\|`unknown`) |
| 37 | `setup_pairing_timeout` | INSTRUMENTED in `Setup.tsx` | Pairing wait exceeds timeout | `platform` (`macos`\|`windows`\|`unknown`) |

### Section A summary for BA

Total events to confirm in PostHog: **37** (34 from event-taxonomy.md + 3 setup events shipped in Wave 6 + `daemon_paired` from v0.3.1 PRD).

| Status | Count |
|---|---|
| Instrumented in code (events fire today) | 9 |
| Typed in code but never fired (taxonomy ready, instrumentation pending) | 27 |
| Referenced as success metric but missing from both code AND taxonomy | 1 (`daemon_paired`) |

BA action: ensure all 37 event definitions exist in PostHog (descriptions + property schemas). For events where code is missing, BA should document the gap on the relevant ticket but NOT block PostHog setup — events defined in PostHog without instrumentation are harmless; events fired without a PostHog definition still capture but lack docs.

---

## Section B — Feature flags to create via API

| # | Flag key | Type | Production rollout (initial) | Staging rollout | Beta launch plan | Status |
|---|---|---|---|---|---|---|
| 1 | `daemon_download_enabled` | Boolean | 0% | 100% to `email contains @stablekernel.com` | Flip to 100% on 2026-08-18 (closed beta launch) | Documented; BA confirm exists in both projects |
| 2 | `beta_invite_only` | Boolean | 0% (per-user opt-in) | 100% to internal users | Per-user opt-in as invitations are sent (June 2 waitlist → August 18 beta); flip default to 100% on GA | Created 2026-05-08 (flag ID 669797); FE/BFF wire-up still pending |
| 3 | `waitlist_signup_enabled` | Boolean | NEW — needs creation | 0% (off until Ray approves go-live) | 100% in staging from creation | Flip to 100% in production on **2026-06-02** (waitlist opens) | Not yet created |
| 4 | `session_replay_enabled` | Boolean | NEW — needs creation | 100% (replay starts after sign-in via `startSessionReplay()` in `usePostHogIdentity.ts`) | 100% staging, 100% in beta | Stays at 100% until 1M-event PostHog free tier pressure forces sampling, then drop to 25% | Not yet created — kill switch for replay if PostHog quota spikes |
| 5 | `live_draft_advisor_enabled` | Boolean | NEW — needs creation | 0% in production | 100% in staging | Flip to 100% on 2026-08-18 OR earlier if SSE/live-draft tickets land sooner; this is a kill switch for the live draft feature in case of stability issues at beta scale | Not yet created |
| 6 | `match_replay_enabled` | Boolean | NEW — needs creation | 0% in production | 100% in staging | Stay at 0% in beta unless replay v1 ships in v0.4.0; kill switch for the replay UI | Not yet created |
| 7 | `community_comparison_enabled` | Boolean | NEW — needs creation | 0% in production | 100% in staging | Stay at 0% until enough beta users exist for the comparisons to be meaningful (~500 MAU). Re-evaluate post-beta | Not yet created |

### Feature flag creation contract for BA

For each flag in the table above:

1. **Create in both Staging and Production PostHog projects** (separate flags per project; do not share IDs across envs).
2. **Aggregation key**: `distinct_id` (per-user bucketing) for all flags except `session_replay_enabled` which uses anonymous distinct_id (no auth required for it to start working).
3. **Description**: copy the "Beta launch plan" column verbatim into the flag description so the flag is self-documenting.
4. **Default value when unauthenticated**: `false` for all flags. The `useFeatureFlag` hook in `frontend/src/hooks/useFeatureFlag.ts` returns `true` only when PostHog is not initialized (dev/test envs) — production behavior is fail-closed.
5. **Naming**: snake_case, no env prefix (`prod_*`, `staging_*` are forbidden — use the project boundary, not the key).

### Flags explicitly NOT needed for beta

- Auth-method flags (force-Google, force-email) — Clerk handles this in dashboard, no PostHog flag needed.
- Per-tenant pricing tier flags — beta is free/invite-only (per `project_beta_monetization.md`); revisit at GA.
- AI assistant / RAG flags — explicitly deferred post-beta.
- Stripe checkout flags — no Stripe in beta.

---

## Open items requiring follow-up by PM (not BA)

These do NOT block BA's PostHog setup — flagging here so they hit the next status rollup:

1. **`daemon_paired` instrumentation**: Wave 5 DoD claims this fires "after successful keychain write" but the typed taxonomy in `frontend/src/services/analytics.ts` does NOT include this event, and code grep finds zero references. Either (a) the event fires from the daemon binary directly via PostHog server-side capture (in which case BA should configure the daemon's PostHog project key), or (b) the BFF emits it from the `/v1/daemon/register` 200 response path (in which case backend instrumentation is missing). PM to confirm with backend-engineer + LE which path is intended, then file ticket through project-manager.
2. **22 typed-but-never-fired events**: Taxonomy is locked but instrumentation is far behind. PM to file a tracking ticket (likely v0.4.0 scope) for an instrumentation sweep. Beta-critical ones: `funnel_landing_page_viewed`, `funnel_sign_up_started`, `page_viewed`, `app_session_started`, `app_user_signed_out`. Without these, the activation funnel has gaps that distort D1/D7 reporting.
3. **`beta_invite_only` FE/BFF wiring**: Flag exists at 0% but no code reads it yet. PM to file a ticket through project-manager for FE gating before invitations go out (no later than 2026-06-02 waitlist open).

---

## Files referenced

- `/Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/frontend/src/services/analytics.ts` — typed event taxonomy (37 events defined)
- `/Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/frontend/src/hooks/usePostHogIdentity.ts` — identify + sign-up funnel firing point
- `/Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/frontend/src/hooks/useFeatureFlag.ts` — flag read hook (fail-open in dev/test, fail-closed in prod)
- `/Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/docs/analytics/event-taxonomy.md` — canonical event spec
- `/Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/docs/analytics/feature-flags.md` — existing flag docs (`daemon_download_enabled`, `beta_invite_only`)
- `/Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/docs/product/milestones/v0.3.1/prd.md` — primary metric is `daemon_paired`
- `/Users/ramonehamilton/Documents/Personal Projects/MTGA-Companion/docs/product/milestones/v0.3.1/kickoff.md` — Wave 5 DoD + release tag exit gate references `daemon_paired`
