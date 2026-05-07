# VaultMTG PostHog Event Taxonomy

**Status**: Finalized pre-beta — lock names before any instrumentation lands  
**Last updated**: 2026-05-06  
**Owner**: Business Analyst  
**Scope**: All PostHog events for beta launch across the SPA

---

## Naming conventions

- All event names: `snake_case`, no spaces, no hyphens
- Prefix rules:
  - `app_` — lifecycle (init, auth, session boundaries)
  - `page_` — navigation/page views
  - `feature_` — meaningful user actions inside a feature
  - `error_` — failure states visible to the user
  - `funnel_` — activation/conversion checkpoints
- Property names: `snake_case` throughout
- Boolean properties: `is_` prefix (e.g., `is_signed_in`)
- IDs: `_id` suffix (e.g., `user_id`, `draft_id`, `deck_id`)
- Timestamps: ISO 8601, omit from event properties — PostHog captures these natively
- Never include PII (email, display name, raw Clerk user data) in event properties. Use the opaque Clerk `user_id` only.

---

## Global properties (sent on every event via `posthog.register`)

| Property | Type | Description |
|---|---|---|
| `app_version` | string | SPA version from `package.json` |
| `is_signed_in` | boolean | Clerk `isSignedIn` at time of event |
| `daemon_status` | string | `connected` \| `disconnected` \| `reconnecting` \| `unknown` |
| `platform` | string | `desktop` \| `web` (from adapter mode) |

---

## 1. Activation funnel events

These are the critical path events. Every step must fire exactly once per user journey. They map directly to our D1/D7 funnel.

### `funnel_landing_page_viewed`
**Trigger**: User loads vaultmtg.app for the first time (anonymous)  
**Instrumented**: No — needs adding  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `referrer` | string | `document.referrer` (blank if direct) |
| `utm_source` | string | from URL param |
| `utm_medium` | string | from URL param |
| `utm_campaign` | string | from URL param |

---

### `funnel_sign_up_started`
**Trigger**: User clicks "Sign up" or "Get started" on the landing page / auth bar  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `entry_point` | string | `landing_page` \| `auth_bar` \| `protected_route_redirect` |

---

### `funnel_sign_up_completed`
**Trigger**: Clerk fires `user.created` — user successfully creates account  
**Instrumented**: No  
**Note**: Instrument inside the Clerk `afterSignUp` callback or a BFF webhook, not in component state. This is the acquisition denominator for all retention calculations.  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `auth_method` | string | `email` \| `google` \| `apple` \| `facebook` |
| `user_id` | string | Clerk user ID (opaque) |

---

### `funnel_daemon_download_started`
**Trigger**: User clicks download button on `/download` page  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `os` | string | `mac` \| `windows` \| `linux` (inferred from user agent) |
| `download_source` | string | `download_page` \| `prompt_modal` |

---

### `funnel_daemon_connected`
**Trigger**: `daemon:connected` WebSocket event fires and UI transitions to `connected` state  
**Instrumented**: No — hook this in `DaemonHealthIndicator` on first `connected` transition  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `time_since_signup_seconds` | number | seconds between `funnel_sign_up_completed` timestamp and this event — approximated client-side |

---

### `funnel_first_data_loaded`
**Trigger**: Match history page renders at least one match row for the first time (non-empty state)  
**Instrumented**: No  
**Note**: This is the "aha moment" — user sees their data for the first time. Track only the first occurrence per user (`posthog.capture` guarded by a persisted flag in localStorage).  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `match_count` | number | number of matches loaded on first view |

---

### `funnel_first_feature_used`
**Trigger**: User navigates to any non-match-history page for the first time  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `feature` | string | page slug: `draft` \| `draft_analytics` \| `decks` \| `collection` \| `meta` \| `charts` \| `quests` |

---

## 2. Page view events

One event per meaningful page load. Do NOT fire on every render or re-render — fire on route change.

### `page_viewed`
**Trigger**: React Router route change (wrap in `useEffect` on `location.pathname`)  
**Instrumented**: No — single implementation in `Layout.tsx`  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `page` | string | route slug — see table below |
| `previous_page` | string | previous `location.pathname` or `null` |

Route slug mapping:

| Path | `page` value |
|---|---|
| `/match-history` | `match_history` |
| `/quests` | `quests` |
| `/draft` | `draft_advisor` |
| `/draft-analytics` | `draft_analytics` |
| `/decks` | `decks` |
| `/deck-builder/:id` | `deck_builder` |
| `/collection` | `collection` |
| `/meta` | `meta` |
| `/charts/win-rate-trend` | `chart_win_rate` |
| `/charts/deck-performance` | `chart_deck_performance` |
| `/charts/rank-progression` | `chart_rank_progression` |
| `/charts/format-distribution` | `chart_format_distribution` |
| `/charts/result-breakdown` | `chart_result_breakdown` |
| `/settings` | `settings` |
| `/history/matches` | `bff_match_history` |
| `/history/drafts` | `bff_draft_history` |
| `/download` | `download` |

---

## 3. Feature usage events

### `feature_match_history_filtered`
**Trigger**: User applies a filter on the match history page  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `filter_type` | string | `format` \| `deck` \| `date_range` \| `result` |
| `filter_value` | string | value applied (no PII — deck name is fine) |

---

### `feature_match_details_opened`
**Trigger**: User opens a match details modal  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `match_result` | string | `win` \| `loss` \| `draw` |
| `format` | string | match format |

---

### `feature_draft_advisor_pick_viewed`
**Trigger**: User sees a card rating in the live draft advisor (CurrentPackPicker renders with cards)  
**Instrumented**: No  
**Note**: Fire once per pack, not once per card, to avoid event explosion.  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `set_code` | string | e.g., `BLB`, `DSK` |
| `pack_number` | number | 1, 2, or 3 |
| `pick_number` | number | pick within pack |

---

### `feature_draft_analytics_viewed`
**Trigger**: User lands on `/draft-analytics` with non-empty data  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `draft_count` | number | number of completed drafts shown |

---

### `feature_deck_builder_opened`
**Trigger**: User navigates to `/deck-builder/:id` (any deck)  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `entry_point` | string | `decks_list` \| `draft_build_around` \| `direct_link` |

---

### `feature_deck_build_around_started`
**Trigger**: User opens the BuildAroundSeedModal  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `seed_type` | string | `card` \| `archetype` \| `color_pair` |

---

### `feature_collection_viewed`
**Trigger**: User lands on `/collection` with non-empty data  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `card_count` | number | total cards in collection |

---

### `feature_meta_viewed`
**Trigger**: User lands on `/meta` with data loaded  
**Instrumented**: No  
**Properties**: none (page itself is the signal)

---

### `feature_ml_suggestions_viewed`
**Trigger**: MLSuggestionsPanel renders with at least one suggestion  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `suggestion_count` | number | number of suggestions shown |
| `context` | string | `deck_builder` \| `draft` |

---

### `feature_chart_interacted`
**Trigger**: User applies a filter or changes a setting on any `/charts/` page  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `chart` | string | chart slug matching `page_viewed.page` |
| `interaction` | string | `filter_applied` \| `time_range_changed` \| `format_changed` |

---

### `feature_opponent_analysis_viewed`
**Trigger**: OpponentAnalysisPanel renders with data  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `opponent_match_count` | number | matches analyzed |

---

### `feature_community_comparison_viewed`
**Trigger**: CommunityComparison renders with data  
**Instrumented**: No  
**Properties**: none

---

### `feature_settings_changed`
**Trigger**: User saves a setting change  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `setting_section` | string | `daemon_connection` \| `preferences` \| `display` |
| `setting_key` | string | which setting was changed (no value — avoid logging sensitive config) |

---

### `feature_replay_started`
**Trigger**: App receives `replay:started` WebSocket event  
**Instrumented**: No — hook in `ReplayEventHandler` in `App.tsx`  
**Properties**: none

---

### `feature_replay_completed`
**Trigger**: App receives `replay:completed` WebSocket event  
**Instrumented**: No  
**Properties**: none

---

## 4. Error and friction events

These measure where users hit walls. High volume on any of these is a direct PM flag.

### `error_daemon_connection_failed`
**Trigger**: `DaemonHealthIndicator` transitions to `error` or `disconnected` state after previously being `connected`  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `previous_status` | string | `connected` \| `reconnecting` |
| `duration_connected_seconds` | number | approximate time daemon was connected before failure |

---

### `error_daemon_never_connected`
**Trigger**: User has been signed in for > 5 minutes and daemon status is still `disconnected` or `error`  
**Instrumented**: No  
**Note**: Implement as a timed check, not a polling event. Fire once per session max.  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `time_since_signin_seconds` | number | how long since sign-in |

---

### `error_data_load_failed`
**Trigger**: Any API call in the SPA returns a non-2xx response and the page renders an error state  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `page` | string | page slug where error occurred |
| `endpoint` | string | API path (no query params, no IDs) — e.g., `/api/v1/matches` |
| `status_code` | number | HTTP status code |

---

### `error_auth_failed`
**Trigger**: Clerk token fetch fails or returns null inside an API call  
**Instrumented**: No  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `context` | string | which service call triggered the auth failure |

---

### `error_empty_state_shown`
**Trigger**: A page renders its empty-state component (no data, no error)  
**Instrumented**: No  
**Note**: Distinguishes "user has no data" from "load failed". High rate on `draft_advisor` = users haven't drafted; high rate on `match_history` = daemon not working.  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `page` | string | page slug |

---

## 5. Engagement / session proxy events

PostHog captures session duration natively. These supplement it with app-specific context.

### `app_session_started`
**Trigger**: App initializes successfully (after `initializeServices()` resolves)  
**Instrumented**: No — fire once in `main.tsx` after `initializeServices()` resolves  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `services_init_ms` | number | milliseconds for `initializeServices()` to resolve |

---

### `app_user_identified`
**Trigger**: Clerk `isSignedIn` transitions from false to true (first render where user is authenticated)  
**Instrumented**: No — hook alongside `SentryUserSync` in `App.tsx`  
**Note**: This is where PostHog `identify(user.id)` must be called. Do it here and nowhere else.  
**Properties**:

| Property | Type | Description |
|---|---|---|
| `user_id` | string | Clerk user ID |
| `auth_method` | string | `email` \| `google` \| `apple` \| `facebook` (from Clerk user object) |

---

### `app_user_signed_out`
**Trigger**: User explicitly signs out (Clerk sign-out flow completes)  
**Instrumented**: No  
**Properties**: none — fire `posthog.reset()` immediately after to clear identity

---

## 6. Instrumentation status summary

| Event | Category | Status |
|---|---|---|
| `funnel_landing_page_viewed` | Activation | Not instrumented |
| `funnel_sign_up_started` | Activation | Not instrumented |
| `funnel_sign_up_completed` | Activation | Not instrumented |
| `funnel_daemon_download_started` | Activation | Not instrumented |
| `funnel_daemon_connected` | Activation | Not instrumented |
| `funnel_first_data_loaded` | Activation | Not instrumented |
| `funnel_first_feature_used` | Activation | Not instrumented |
| `page_viewed` | Navigation | Not instrumented |
| `feature_match_history_filtered` | Feature | Not instrumented |
| `feature_match_details_opened` | Feature | Not instrumented |
| `feature_draft_advisor_pick_viewed` | Feature | Not instrumented |
| `feature_draft_analytics_viewed` | Feature | Not instrumented |
| `feature_deck_builder_opened` | Feature | Not instrumented |
| `feature_deck_build_around_started` | Feature | Not instrumented |
| `feature_collection_viewed` | Feature | Not instrumented |
| `feature_meta_viewed` | Feature | Not instrumented |
| `feature_ml_suggestions_viewed` | Feature | Not instrumented |
| `feature_chart_interacted` | Feature | Not instrumented |
| `feature_opponent_analysis_viewed` | Feature | Not instrumented |
| `feature_community_comparison_viewed` | Feature | Not instrumented |
| `feature_settings_changed` | Feature | Not instrumented |
| `feature_replay_started` | Feature | Not instrumented |
| `feature_replay_completed` | Feature | Not instrumented |
| `error_daemon_connection_failed` | Error | Not instrumented |
| `error_daemon_never_connected` | Error | Not instrumented |
| `error_data_load_failed` | Error | Not instrumented |
| `error_auth_failed` | Error | Not instrumented |
| `error_empty_state_shown` | Error | Not instrumented |
| `app_session_started` | Engagement | Not instrumented |
| `app_user_identified` | Engagement | Not instrumented |
| `app_user_signed_out` | Engagement | Not instrumented |

**Total events defined**: 31  
**Currently instrumented**: 0  

---

## 7. Implementation notes for the frontend ticket

1. **PostHog initialization** — call `posthog.init(VITE_POSTHOG_KEY, { api_host: 'https://app.posthog.com', capture_pageview: false })` in `main.tsx` before `renderApp()`. Set `capture_pageview: false` because we fire `page_viewed` manually to attach properties.

2. **Identity** — call `posthog.identify(user.id)` inside `app_user_identified` handler only. Never call it in a render path where Clerk state is still loading.

3. **Global super-properties** — call `posthog.register({ app_version, is_signed_in, daemon_status, platform })` in two places: on `app_session_started` and whenever `daemon_status` changes. This keeps every event enriched without repeating properties on each `capture` call.

4. **Preventing duplicate funnel events** — `funnel_first_data_loaded` and `funnel_first_feature_used` must check a `localStorage` flag before firing. Key convention: `vaultmtg_ph_[event_name]_fired`.

5. **Event volume risk** — `feature_draft_advisor_pick_viewed` fires once per pack (not per card). At 45 picks/draft × 3 packs = 45 events per session max, well within PostHog free tier limits at beta scale.

6. **Do not track** — do not fire events for: card hover previews, tooltip opens, CSS animations, filter resets that produce no results. These add noise without signal.

7. **Testing** — verify each event in PostHog's Live Events view before merging the instrumentation PR. The frontend ticket should include a checklist of all 31 events with a "verified in Live Events" checkbox for each.
