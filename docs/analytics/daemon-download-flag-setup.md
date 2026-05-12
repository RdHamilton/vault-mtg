# PostHog Feature Flag Setup: daemon_download_enabled

**Timeline**: Complete by 2026-08-18 (closed beta launch)  
**Owner**: Ray Hamilton  
**Effort**: 2 minutes (dashboard UI only, no code)

---

## Summary

Create a PostHog feature flag `daemon_download_enabled` to gate the daemon installer download buttons on `/download`. This flag will be:
- **ON** for staging (internal team only)
- **OFF** for production (until beta launch on August 18, 2026)

---

## Prerequisites

- PostHog account with access to both **staging** and **production** projects
- Clerk user ID(s) or email address(es) to whitelist for staging access

---

## Step 1: Set up Staging Flag (Immediate)

1. Go to https://app.posthog.com
2. **Select staging project** (top-left dropdown)
3. Navigate to **Experimentation** → **Feature Flags** (left sidebar)
4. Click **Create new feature flag** (top-right)
5. Fill in:
   - **Name**: `daemon_download_enabled` (or "Daemon Download Enabled")
   - **Key**: `daemon_download_enabled` (must match exactly)
   - **Description**: "Gates daemon installer download buttons on /download page. When OFF, users see waitlist CTA instead."
6. Click **Proceed to rollout**
7. In the **Rollout** section:
   - **Condition type**: Users matching a property
   - **Property**: Select "email"
   - **Condition**: "contains" @stablekernel.com
   - **Percentage**: 100% (give all matching users access)
   - **Default**: OFF (for non-matching users)
8. Click **Save and release**

---

## Step 2: Set up Production Flag (Complete Today)

1. **Switch to production project** (top-left dropdown)
2. Repeat steps 3–8 from above, but in the **Rollout** section:
   - **Condition type**: Choose "Release conditions" (or leave empty for global setting)
   - **Percentage**: 0% (disabled globally)
   - **Default**: OFF
   - **Save and release**

---

## Step 3: Schedule August 18 Activation (Optional but Recommended)

After production flag is created:

1. **Edit production flag**: Click the flag name or pencil icon
2. Scroll to **Rollout percentage** and look for **"Schedule a release"** (if available in your PostHog version)
3. Set **date**: 2026-08-18
4. Set **percentage**: 100%
5. Save

If PostHog doesn't have schedule UI, you'll manually flip it to 100% on August 18 — set a calendar reminder.

---

## Step 4: Verify Integration

### In staging:
1. **Test as internal user** (sign in with @stablekernel.com email or use local dev)
2. Visit `/download` page
3. **Expected behavior**: Download buttons are visible
4. Open browser DevTools Console:
   ```javascript
   // This should return true
   window.posthog.isFeatureEnabled('daemon_download_enabled')
   ```

### In production:
1. **Test as any user** (or use anonymous/different account)
2. Visit `https://vaultmtg.app/download`
3. **Expected behavior**: "Coming soon / join waitlist" CTA is visible
4. **Expected console result**: `false`

---

## Flag Keys Reference

For frontend code integration, use this key exactly:
```
daemon_download_enabled
```

Example component integration (TBD in separate ticket):
```typescript
import { useFeatureFlag } from '@/hooks/useFeatureFlag';

export function DownloadPage() {
  const { isFeatureEnabled } = useFeatureFlag('daemon_download_enabled');
  
  return (
    <div>
      {isFeatureEnabled ? (
        <DownloadButtons />
      ) : (
        <WaitlistCTA />
      )}
    </div>
  );
}
```

---

## Troubleshooting

| Issue | Fix |
|---|---|
| Flag doesn't appear in frontend | Verify `VITE_POSTHOG_KEY` is set in Vercel project settings for the environment |
| Flag returns `false` even when it should be `true` | Verify user email/ID matches staging filter; check that the user is identified to PostHog via `identifyUser(userId)` |
| Cannot find Rollout UI | Your PostHog version may use a different UI — look for "Rollout conditions" or "Targeting" section |

---

## Rollback Plan

If something goes wrong:

1. **Staging**: Edit flag → set percentage to 0% → save
2. **Production**: Ensure percentage stays 0% until August 18
3. **Frontend**: Feature gracefully degrades (users see waitlist CTA if flag is OFF, which is the safe default)

---

## Related Documentation

- Feature flags registry: `docs/analytics/feature-flags.md`
- Frontend hooks: `frontend/src/hooks/useFeatureFlag.ts`
- Analytics init: `frontend/src/services/analytics.ts`
- Download page: `frontend/src/pages/DownloadPage.tsx` (TBD)
