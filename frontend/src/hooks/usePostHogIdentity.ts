/**
 * Identifies the signed-in Clerk user with PostHog and fires
 * `funnel_sign_up_completed` once per browser session.
 *
 * Rules:
 * - Only fires when Clerk `isLoaded && isSignedIn && user.id` is truthy.
 * - `funnel_sign_up_completed` is guarded by a sessionStorage key so it fires
 *   at most once per browser session (not once per page load).
 * - Identity is reset on sign-out via `resetIdentity()`.
 */
import { useEffect, useRef } from 'react';
import { useUser } from '@clerk/clerk-react';
import {
  Events,
  captureEvent,
  identifyUser,
  resetIdentity,
} from '../services/analytics';

const SESSION_KEY = 'vaultmtg_ph_funnel_sign_up_completed_fired';

export function usePostHogIdentity(): void {
  const { isLoaded, isSignedIn, user } = useUser();
  const identifiedRef = useRef(false);

  useEffect(() => {
    if (!isLoaded) return;

    if (isSignedIn && user?.id) {
      if (!identifiedRef.current) {
        identifyUser(user.id);
        identifiedRef.current = true;

        // Fire funnel_sign_up_completed once per session.
        if (!sessionStorage.getItem(SESSION_KEY)) {
          captureEvent(Events.FUNNEL_SIGN_UP_COMPLETED, {
            user_id: user.id,
          });
          sessionStorage.setItem(SESSION_KEY, '1');
        }
      }
    } else if (isLoaded && !isSignedIn) {
      // User signed out — reset PostHog identity.
      if (identifiedRef.current) {
        resetIdentity();
        identifiedRef.current = false;
      }
    }
  }, [isLoaded, isSignedIn, user?.id]);
}
