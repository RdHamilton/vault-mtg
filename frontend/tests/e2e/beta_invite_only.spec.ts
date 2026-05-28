import { test, expect, Page } from '@playwright/test';

/**
 * beta_invite_only Feature Flag — E2E Tests (#1840)
 *
 * Verifies the AuthBar respects the `beta_invite_only` PostHog feature flag:
 *   - flag OFF  → SignUpButton is NOT rendered; SignInButton IS rendered
 *   - flag ON   → both SignInButton and SignUpButton ARE rendered (default beta UX)
 *
 * Approach:
 *   1. Intercept the PostHog /decide endpoint via page.route() to return a
 *      controlled flag value — the same pattern used by tests/e2e/download.spec.ts
 *      for daemon_download_enabled.
 *   2. Inject signed-out Clerk test state via window.__CLERK_TEST_STATE__ (same
 *      pattern as auth.spec.ts) so the AuthBar signed-out branch renders.
 *
 * When VITE_POSTHOG_KEY is absent (local dev, CI without a key), posthog.init()
 * is a no-op and posthog.__loaded stays false — useFeatureFlag returns true by
 * default, keeping the sign-up button visible. The /decide interception below
 * covers CI runs where the key IS present; the flag-ON test covers both cases.
 *
 * Note: these tests run against the full app (flag-gated by real network routing),
 * NOT against the Clerk mock alone. VITE_CLERK_TEST_MODE=true is set by the
 * playwright.config.ts webServer command, aliasing @clerk/react to clerkMock.tsx.
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
 * Intercept the PostHog /decide endpoint to set feature flag values.
 * PostHog calls /decide?v=3 (or similar) to fetch flag payloads.
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
    // Route PostHog /decide before navigation so flag is available on first load
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
// ---------------------------------------------------------------------------

test.describe('Feature: beta_invite_only flag — PostHog absent (key not configured)', () => {
  test.beforeEach(async ({ page }) => {
    // Abort /decide to simulate PostHog unavailable. useFeatureFlag stays in its
    // null/loading state → AuthBar optimistically shows sign-up (signUpEnabled !== false).
    await abortPostHogDecide(page);
    await setClerkSignedOut(page);
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
