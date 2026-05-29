import { test, expect, Page } from '@playwright/test';

/**
 * beta_invite_only Feature Flag — E2E Tests (#1840)
 *
 * Verifies the AuthBar respects the `beta_invite_only` PostHog feature flag:
 *   - flag OFF  → SignUpButton is NOT rendered; SignInButton IS rendered
 *   - flag ON   → both SignInButton and SignUpButton ARE rendered (default beta UX)
 *
 * Approach:
 *   1. Inject flag state via window.__POSTHOG_TEST_FLAGS__ (addInitScript) so
 *      useFeatureFlag picks up the override without PostHog being initialized.
 *      This is deterministic on CI regardless of VITE_POSTHOG_KEY presence —
 *      no /decide network call is needed. The PostHog /decide intercept is kept
 *      as defence-in-depth for environments where VITE_POSTHOG_KEY IS set, but
 *      the window override is the primary mechanism.
 *   2. Inject signed-out Clerk test state via window.__CLERK_TEST_STATE__ (same
 *      pattern as auth.spec.ts) so the AuthBar signed-out branch renders.
 *
 * Root cause of prior CI failure (#26614819125):
 *   VITE_POSTHOG_KEY is absent in the e2e-smoke.yml build. Without the key,
 *   posthog.init() is a no-op, posthog.__loaded stays false, and useFeatureFlag
 *   always returns { enabled: true } (the "not loaded" default). The /decide
 *   route intercept was never called, so the flag-OFF test saw sign-up always
 *   visible and failed. Fix: useFeatureFlag now checks window.__POSTHOG_TEST_FLAGS__
 *   before consulting PostHog, and tests inject the flag there instead of relying
 *   on the network intercept alone.
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type ClerkTestState = {
  isSignedIn: boolean;
  firstName?: string;
  lastName?: string;
};

/** Inject signed-out state before page load. Must be called before page.goto(). */
async function setClerkSignedOut(page: Page): Promise<void> {
  const state: ClerkTestState = { isSignedIn: false };
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, state);
}

/**
 * Inject a PostHog feature flag value via window.__POSTHOG_TEST_FLAGS__.
 * useFeatureFlag reads this override before consulting posthog.isFeatureEnabled(),
 * making the flag state deterministic regardless of whether PostHog is initialized.
 * Must be called before page.goto().
 */
async function setPostHogFlag(
  page: Page,
  flagKey: string,
  enabled: boolean
): Promise<void> {
  await page.addInitScript(
    ({ key, value }: { key: string; value: boolean }) => {
      if (!(window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__) {
        (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__ = {};
      }
      ((window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__ as Record<string, boolean>)[key] = value;
    },
    { key: flagKey, value: enabled }
  );
}

/**
 * Intercept the PostHog /decide endpoint to set feature flag values.
 * Kept as defence-in-depth for environments where VITE_POSTHOG_KEY IS set.
 * The window.__POSTHOG_TEST_FLAGS__ override (above) is the primary mechanism.
 * Must be called before page.goto().
 */
async function mockPostHogFlag(
  page: Page,
  flagKey: string,
  enabled: boolean
): Promise<void> {
  await page.route('**/decide**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        featureFlags: {
          [flagKey]: enabled,
        },
        featureFlagPayloads: {},
      }),
    });
  });
}

/**
 * Simulate PostHog /decide being unreachable (network failure or key absent).
 * Aborting the request prevents posthog.onFeatureFlags() from firing, leaving
 * useFeatureFlag in its initial null/loading state. AuthBar treats null as
 * "show sign-up" (optimistic default — signUpEnabled !== false).
 * Must be called before page.goto().
 */
async function abortPostHogDecide(page: Page): Promise<void> {
  await page.route('**/decide**', async (route) => {
    await route.abort();
  });
}

// ---------------------------------------------------------------------------
// Tests: flag OFF path (primary E2E coverage per ticket requirement)
// ---------------------------------------------------------------------------

test.describe('Feature: beta_invite_only flag — flag OFF (SignUp hidden)', () => {
  test.beforeEach(async ({ page }) => {
    // Primary: inject flag via window global — works regardless of VITE_POSTHOG_KEY
    await setPostHogFlag(page, 'beta_invite_only', false);
    // Defence-in-depth: also intercept /decide for envs with VITE_POSTHOG_KEY set
    await mockPostHogFlag(page, 'beta_invite_only', false);
    await setClerkSignedOut(page);
  });

  test('@smoke beta_invite_only OFF — sign-up button is NOT rendered; sign-in IS rendered', async ({
    page,
  }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Auth bar must be present
    await expect(page.locator('[data-testid="auth-bar"]')).toBeVisible();

    // Signed-out section is visible
    await expect(page.locator('[data-testid="auth-signed-out"]')).toBeVisible();

    // Sign-in must be present regardless of flag
    await expect(page.locator('[data-testid="sign-in-btn"]')).toBeVisible();

    // Sign-up must NOT be present when flag is off
    await expect(page.locator('[data-testid="sign-up-btn"]')).not.toBeAttached();
  });

  test('beta_invite_only OFF — sign-up button absent on /download route', async ({ page }) => {
    await page.goto('/download');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Sign-in is still available
    await expect(page.locator('[data-testid="sign-in-btn"]')).toBeVisible();

    // Sign-up absent
    await expect(page.locator('[data-testid="sign-up-btn"]')).not.toBeAttached();
  });
});

// ---------------------------------------------------------------------------
// Tests: flag ON path (regression guard — existing UX must not break)
// ---------------------------------------------------------------------------

test.describe('Feature: beta_invite_only flag — flag ON (both buttons visible)', () => {
  test.beforeEach(async ({ page }) => {
    await setPostHogFlag(page, 'beta_invite_only', true);
    await mockPostHogFlag(page, 'beta_invite_only', true);
    await setClerkSignedOut(page);
  });

  test('@smoke beta_invite_only ON — both sign-in and sign-up buttons ARE rendered', async ({
    page,
  }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await expect(page.locator('[data-testid="auth-signed-out"]')).toBeVisible();
    await expect(page.locator('[data-testid="sign-in-btn"]')).toBeVisible();
    await expect(page.locator('[data-testid="sign-up-btn"]')).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Tests: PostHog absent (VITE_POSTHOG_KEY not set — dev / CI without key)
// When PostHog is not initialized, posthog.__loaded is false and useFeatureFlag
// falls back to true → sign-up visible (safe default).
// No __POSTHOG_TEST_FLAGS__ injection here — this test validates the fallback
// path when neither PostHog nor the test override is present.
// ---------------------------------------------------------------------------

test.describe('Feature: beta_invoke_only flag — PostHog absent (key not configured)', () => {
  test.beforeEach(async ({ page }) => {
    // Abort /decide to simulate PostHog unavailable. useFeatureFlag stays in its
    // null/loading state → AuthBar optimistically shows sign-up (signUpEnabled !== false).
    await abortPostHogDecide(page);
    await setClerkSignedOut(page);
    // Intentionally no setPostHogFlag() — validate the no-PostHog-no-override path
  });

  test('sign-up button is visible when PostHog is not initialized (optimistic default)', async ({
    page,
  }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // When PostHog is absent, useFeatureFlag returns true → sign-up visible
    await expect(page.locator('[data-testid="sign-in-btn"]')).toBeVisible();
    await expect(page.locator('[data-testid="sign-up-btn"]')).toBeVisible();
  });
});
