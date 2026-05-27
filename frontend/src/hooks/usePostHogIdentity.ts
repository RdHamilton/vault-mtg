/**
 * Identifies the signed-in Clerk user with PostHog, fires session lifecycle
 * events, and registers super-properties on every identity change.
 *
 * Rules:
 * - Only fires when Clerk `isLoaded && isSignedIn && user.id` is truthy.
 * - `funnel_sign_up_completed` is guarded by a sessionStorage key so it fires
 *   at most once per browser session (not once per page load).
 * - `app_user_identified` fires on every successful identify call.
 * - `app_user_signed_out` fires BEFORE `posthog.reset()` on sign-out.
 * - Identity is reset on sign-out via `resetIdentity()`.
 * - Super-properties registered: `app_version`, `is_signed_in`, `platform`.
 *   `daemon_status` is intentionally excluded — follow-up ticket pending.
 */
import { useEffect, useRef } from 'react';
import { useUser } from '@clerk/react';
import {
  trackEvent,
  identifyUser,
  resetIdentity,
  startSessionReplay,
  stopSessionReplay,
  registerSuperProperties,
} from '../services/analytics';

const SESSION_KEY = 'vaultmtg_ph_funnel_sign_up_completed_fired';

// app_version comes from Vite build-time injection; fallback to 'unknown' per
// Ray adj. #5 — do NOT fail-build if the env var is absent.
const APP_VERSION: string =
  (import.meta.env.VITE_APP_VERSION as string | undefined) ?? 'unknown';

// platform is always 'desktop' for the SPA served at app.vaultmtg.app.
const PLATFORM = 'desktop' as const;

export function usePostHogIdentity(): void {
  const { isLoaded, isSignedIn, user } = useUser();
  const identifiedRef = useRef(false);

  useEffect(() => {
    if (!isLoaded) return;

    if (isSignedIn && user?.id) {
      if (!identifiedRef.current) {
        identifyUser(user.id);
        // Enable session replay now that we have a confirmed signed-in user.
        // Recording is disabled at init time and only starts here.
        startSessionReplay();
        identifiedRef.current = true;

        // Register super-properties for every subsequent event in this session.
        // Narrowed set per Ray adj. #3: daemon_status excluded (follow-up ticket).
        registerSuperProperties({
          app_version: APP_VERSION,
          is_signed_in: true,
          platform: PLATFORM,
        });

        // Fire app_user_identified on every successful identify.
        // NOTE: user_id is intentionally omitted — frontend hashing pending.
        trackEvent({
          name: 'app_user_identified',
          properties: { auth_method: 'email' },
        });

        // Fire funnel_sign_up_completed once per session.
        if (!sessionStorage.getItem(SESSION_KEY)) {
          trackEvent({
            name: 'funnel_sign_up_completed',
            properties: {
              auth_method: 'email',
              user_id: user.id,
            },
          });
          sessionStorage.setItem(SESSION_KEY, '1');
        }
      }
    } else if (isLoaded && !isSignedIn) {
      // User signed out — fire lifecycle event, update super-properties,
      // then reset PostHog identity and stop recording.
      if (identifiedRef.current) {
        // app_user_signed_out fires BEFORE posthog.reset() (ordering is
        // critical — reset() clears the distinct_id so the event would be
        // attributed to an anonymous user if called after reset).
        trackEvent({ name: 'app_user_signed_out', properties: {} });

        // Update is_signed_in super-property for any post-signout events.
        registerSuperProperties({
          app_version: APP_VERSION,
          is_signed_in: false,
          platform: PLATFORM,
        });

        stopSessionReplay();
        resetIdentity();
        identifiedRef.current = false;
      }
    }
  }, [isLoaded, isSignedIn, user?.id]);
}
