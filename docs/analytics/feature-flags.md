# Feature Flags

## beta_invite_only

**Flag ID**: 669797  
**Key**: `beta_invite_only`  
**Status**: Active  
**Created**: 2026-05-08

### Purpose
Gates the beta invite flow. Controls which users see and can access the beta signup/invite experience. Used to roll out beta access to selected users as invitations are sent.

### Rollout Strategy
- **Default rollout**: 0% — no users have access unless explicitly enabled
- **Activation model**: Per-user opt-in via PostHog dashboard or API
- Users matching this flag will be shown beta features and allowed to enter beta signup flows

### Configuration
- **Filters**: Single group with 0% rollout_percentage
- **Aggregation key**: `distinct_id` (per-user bucketing)
- **Active**: Yes

### Code Integration
- **Frontend**: `frontend/src/components/AuthBar.tsx` — `useFeatureFlag('beta_invite_only')` controls `<SignUpButton>` visibility.
  - flag ON → SignUpButton visible (default beta UX)
  - flag OFF → SignUpButton hidden; SignInButton always visible
  - flag loading (null) → optimistic show (avoids flash)
- **PostHog auto-emission**: `$feature_flag_called` emitted automatically via posthog-js v1.372.9 `isFeatureEnabled` call path on every new `(key, variant)` pair per session.

### Related Documentation
- Event taxonomy: `docs/analytics/event-taxonomy.md`
- Implementation: PR #2681 (feat/1840-posthog-flag-eval)

---

## waitlist_signup_enabled

**Flag ID**: 671181  
**Key**: `waitlist_signup_enabled`  
**Status**: Active  
**Created**: 2026-05-27

### Purpose
Controls whether the waitlist/signup UI is shown globally. Distinct from `beta_invite_only` which gates per-user access — this flag controls global signup UI visibility (e.g., show a "Join Waitlist" CTA before beta launch, hide during closed beta).

### Rollout Strategy
- **Default rollout**: 0% — not shown until beta launch
- **Activation model**: Global rollout flip on 2026-08-18 beta launch
- **Target cohort**: All users; flip to 100% on beta launch

### Configuration
- **Aggregation key**: `distinct_id`
- **Active**: Yes

### Code Integration
- **Frontend**: `frontend/src/components/AuthBar.tsx` — future ticket; wiring deferred. The flag exists in PostHog and is ready to be used. See backlog for the companion ticket.

### Related Documentation
- Implementation ticket: Backlog (separate from #1840)

---

## session_replay_enabled

**Flag ID**: 671182  
**Key**: `session_replay_enabled`  
**Status**: Active  
**Created**: 2026-05-27

### Purpose
Gates the PostHog session replay recording (`startSessionReplay()`) for signed-in users. Allows gradual rollout of session recording to avoid performance impact and control costs.

### Rollout Strategy
- **Default rollout**: 100% — session replay is enabled for all signed-in users
- **Activation model**: Percentage rollout
- **Target cohort**: Signed-in users

### Configuration
- **Aggregation key**: `distinct_id`
- **Active**: Yes

### Code Integration
- **Frontend**: `frontend/src/hooks/usePostHogIdentity.ts` — gates `startSessionReplay()` call on sign-in. Wiring deferred to a separate ticket; currently `startSessionReplay()` is called unconditionally on identity resolution.

### Related Documentation
- Implementation ticket: Backlog (separate from #1840; scope deferred per plan)

---

## live_draft_advisor_enabled

**Flag ID**: 671183  
**Key**: `live_draft_advisor_enabled`  
**Status**: Active  
**Created**: 2026-05-27

### Purpose
Gates the live draft advisor feature. Controls access to the real-time AI-assisted draft recommendation experience.

### Rollout Strategy
- **Default rollout**: 0% — beta cohort only
- **Activation model**: Per-user opt-in, beta-invite cohort
- **Target cohort**: Beta-invite users

### Configuration
- **Aggregation key**: `distinct_id`
- **Active**: Yes

### Code Integration
- **Frontend**: `frontend/src/pages/DraftLive.tsx` (line ~91, `DraftLive` component) — gates the live draft page entry point or advisor panel. The `/draft/live` route is registered in `frontend/src/App.tsx` line 232. Exact gate insertion point (route guard vs. in-component feature panel) to be determined in the wiring ticket.

### Related Documentation
- Implementation ticket: Backlog (separate from #1840; wiring deferred)

---

## match_replay_enabled

**Flag ID**: 671184  
**Key**: `match_replay_enabled`  
**Status**: Active  
**Created**: 2026-05-27

### Purpose
Gates the match replay feature. Controls whether users can replay previously played matches.

### Rollout Strategy
- **Default rollout**: 0% — beta cohort only
- **Activation model**: Per-user opt-in, beta-invite cohort
- **Target cohort**: Beta-invite users

### Configuration
- **Aggregation key**: `distinct_id`
- **Active**: Yes

### Code Integration
- **Frontend**: `frontend/src/pages/BffMatchHistory.tsx` — match replay entry point lives in the match history page. The `/match-history` route is registered in `frontend/src/App.tsx` line 216. Exact gate insertion point (replay button or replay route) to be determined in the wiring ticket.

### Related Documentation
- Implementation ticket: Backlog (separate from #1840; wiring deferred)
- Related event: `FEATURE_REPLAY_STARTED`, `FEATURE_REPLAY_COMPLETED` in `frontend/src/services/analytics.ts`

---

## community_comparison_enabled

**Flag ID**: 671185  
**Key**: `community_comparison_enabled`  
**Status**: Active  
**Created**: 2026-05-27

### Purpose
Gates the community comparison feature. Controls visibility of the community performance comparison panel in draft analytics.

### Rollout Strategy
- **Default rollout**: 0% — beta cohort only
- **Activation model**: Per-user opt-in, beta-invite cohort
- **Target cohort**: Beta-invite users

### Configuration
- **Aggregation key**: `distinct_id`
- **Active**: Yes

### Code Integration
- **Frontend**: `frontend/src/pages/DraftAnalytics.tsx` line 112, `<CommunityComparison />` component render. The `CommunityComparison` component is imported from `frontend/src/components/CommunityComparison.tsx`. The wiring ticket should guard the `<CommunityComparison>` render with `useFeatureFlag('community_comparison_enabled')`.

### Related Documentation
- Implementation ticket: Backlog (separate from #1840; wiring deferred)
- Related event: `FEATURE_COMMUNITY_COMPARISON_VIEWED` in `frontend/src/services/analytics.ts` (line 96)

---

## daemon_download_enabled

**Flag ID**: (see `docs/analytics/daemon-download-flag-setup.md`)  
**Key**: `daemon_download_enabled`  
**Status**: Active  

### Purpose
Gates the daemon download UI on the `/download` route. When disabled, shows a "coming soon" placeholder instead of the download links.

### Code Integration
- **Frontend**: `frontend/src/components/DaemonDownload.tsx` — see `docs/analytics/daemon-download-flag-setup.md` for full setup instructions.

### Related Documentation
- Setup guide: `docs/analytics/daemon-download-flag-setup.md`
