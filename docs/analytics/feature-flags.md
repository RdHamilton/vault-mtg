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
Frontend and BFF integration of this flag is deferred to a separate ticket. The flag exists in PostHog and is ready to be used once that ticket is implemented.

### Related Documentation
- Event taxonomy: `docs/analytics/event-taxonomy.md`
