/**
 * useFeatureFlag — wraps posthog.isFeatureEnabled with a loading state.
 *
 * Returns:
 *   { enabled: true }   — flag is on
 *   { enabled: false }  — flag is off
 *   { enabled: null }   — flag value not yet loaded (show skeleton)
 *
 * Relies on posthog-js being initialized by initAnalytics() in analytics.ts.
 * When PostHog is not initialized (key absent, test env) the hook resolves
 * immediately to { enabled: true } so the default experience is preserved in
 * environments without PostHog configured.
 */
import { useEffect, useState } from 'react';
import posthog from 'posthog-js';

export interface FeatureFlagResult {
  enabled: boolean | null;
}

/**
 * Resolve the current flag value synchronously.
 * Returns true when PostHog is not loaded (dev / test envs default to enabled).
 * Returns null when PostHog is loaded but flags have not arrived yet.
 */
function resolveFlag(flagKey: string): boolean | null {
  if (!posthog.__loaded) {
    return true;
  }
  // isFeatureEnabled returns boolean | undefined — undefined means not yet loaded
  const value = posthog.isFeatureEnabled(flagKey);
  if (value === undefined) {
    return null;
  }
  return value;
}

export function useFeatureFlag(flagKey: string): FeatureFlagResult {
  const [enabled, setEnabled] = useState<boolean | null>(() => resolveFlag(flagKey));

  useEffect(() => {
    // If PostHog is not initialized there is nothing to subscribe to.
    if (!posthog.__loaded) {
      return;
    }

    // onFeatureFlags fires once flags have loaded from the PostHog server.
    // If flags are already loaded it fires synchronously on the next tick.
    const unsubscribe = posthog.onFeatureFlags(() => {
      const value = posthog.isFeatureEnabled(flagKey);
      setEnabled(value ?? false);
    });

    return () => {
      if (typeof unsubscribe === 'function') {
        unsubscribe();
      }
    };
  }, [flagKey]);

  return { enabled };
}
