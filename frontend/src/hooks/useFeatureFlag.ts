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
 *
 * Test override: Playwright tests may inject window.__POSTHOG_TEST_FLAGS__
 * via page.addInitScript() to force a specific flag value without requiring
 * PostHog to be initialized. The window global takes precedence over all
 * PostHog state when a key is present.
 */
import { useEffect, useState } from 'react';
import posthog from 'posthog-js';

export interface FeatureFlagResult {
  enabled: boolean | null;
}

/**
 * Resolve the current flag value synchronously.
 *
 * Priority order:
 *   1. window.__POSTHOG_TEST_FLAGS__[flagKey] — test override (Playwright only)
 *   2. posthog.isFeatureEnabled(flagKey)      — live PostHog value
 *   3. true                                   — PostHog not loaded (default)
 *
 * Returns null when PostHog is loaded but flags have not arrived yet.
 */
function resolveFlag(flagKey: string): boolean | null {
  // Test override — check before PostHog so Playwright can control flag state
  // without VITE_POSTHOG_KEY being set in the E2E build.
  if (
    typeof window !== 'undefined' &&
    window.__POSTHOG_TEST_FLAGS__ != null &&
    flagKey in window.__POSTHOG_TEST_FLAGS__
  ) {
    return window.__POSTHOG_TEST_FLAGS__[flagKey];
  }

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
    // Re-resolve in case the test override was set after module init.
    setEnabled(resolveFlag(flagKey));

    // If PostHog is not initialized there is nothing to subscribe to.
    if (!posthog.__loaded) {
      return;
    }

    // onFeatureFlags fires once flags have loaded from the PostHog server.
    // If flags are already loaded it fires synchronously on the next tick.
    const unsubscribe = posthog.onFeatureFlags(() => {
      // Test override still takes precedence even after PostHog fires.
      setEnabled(resolveFlag(flagKey));
    });

    return () => {
      if (typeof unsubscribe === 'function') {
        unsubscribe();
      }
    };
  }, [flagKey]);

  return { enabled };
}
