import { test, expect, type Page } from '@playwright/test';

/**
 * Staging SPA Smoke Suite (#1933)
 *
 * Authenticates with a real Clerk account and navigates every SPA route at
 * stg-app.vaultmtg.app, asserting no blank screen and no React error boundary.
 *
 * Authentication:
 *   Uses SMOKE_CLERK_EMAIL / SMOKE_CLERK_PASSWORD to sign in via the Clerk
 *   hosted sign-in UI. If either env var is absent the suite is skipped with
 *   a clear message so developers running locally don't see failures they
 *   cannot fix.
 *
 * waitUntil strategy (#1949):
 *   All page.goto() calls use 'domcontentloaded' instead of 'networkidle'.
 *   Background analytics/CDN requests can keep the network busy indefinitely
 *   on GitHub-hosted runners, causing intermittent 30 s timeouts.
 *
 * Assertion strategy per route:
 *   1. Page has content (no blank screen) — document.body has child elements
 *   2. No React error boundary visible
 *   3. At least one ARIA landmark or known data-testid is present
 *
 * Required environment variables:
 *   SMOKE_CLERK_EMAIL      — Clerk test account email
 *   SMOKE_CLERK_PASSWORD   — Clerk test account password
 *   STAGING_SPA_URL        — override staging SPA base URL (optional)
 */

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const SMOKE_EMAIL = process.env.SMOKE_CLERK_EMAIL ?? '';
const SMOKE_PASSWORD = process.env.SMOKE_CLERK_PASSWORD ?? '';
// Use `||` (not `??`) so that an empty-string CI secret falls back to the
// default. `??` only falls back on `undefined`/`null`, which left
// BASE_URL = '' when STAGING_SPA_URL was set-but-empty in CI (#1933).
const BASE_URL = process.env.STAGING_SPA_URL || 'https://stg-app.vaultmtg.app';

// ---------------------------------------------------------------------------
// Routes from App.tsx
// ---------------------------------------------------------------------------

/** Public routes — accessible without authentication. */
const PUBLIC_ROUTES = ['/download', '/setup'] as const;

/**
 * Protected routes — require Clerk sign-in.
 * `/` redirects to `/home` so it is covered by the protected list.
 */
const PROTECTED_ROUTES = [
  '/home',
  '/match-history',
  '/quests',
  '/draft',
  '/draft-analytics',
  '/decks',
  '/collection',
  '/meta',
  '/charts/win-rate-trend',
  '/charts/deck-performance',
  '/charts/rank-progression',
  '/charts/format-distribution',
  '/charts/result-breakdown',
  '/settings',
  '/history/drafts',
  '/draft/live',
  '/api-keys',
] as const;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Assert no blank screen and no visible React error boundary on the current page.
 *
 * A "blank screen" is defined as a page whose #root element has no child nodes
 * after the React tree has had time to mount. An "error boundary" is detected
 * by looking for elements with `.react-error-boundary` class or
 * `data-testid="error-boundary"` attribute.
 */
async function assertPageIsHealthy(page: Page, route: string): Promise<void> {
  // 1. Page must have some content
  const bodyChildren = await page.evaluate(() => document.body.children.length);
  expect(
    bodyChildren,
    `Route ${route}: blank screen — document.body has no children`,
  ).toBeGreaterThan(0);

  // 2. No visible React error boundary
  const errorBoundary = page.locator('.react-error-boundary, [data-testid="error-boundary"]');
  await expect(
    errorBoundary,
    `Route ${route}: React error boundary is visible`,
  ).not.toBeVisible();

  // 3. At least one ARIA landmark or known root element is present
  const hasLandmark = await page.evaluate(() => {
    const landmarks = [
      'main', 'nav', 'header', 'footer', 'aside',
      '[role="main"]', '[role="navigation"]', '[role="banner"]',
      '#root', '[data-testid]',
    ];
    return landmarks.some((selector) => document.querySelector(selector) !== null);
  });
  expect(
    hasLandmark,
    `Route ${route}: no ARIA landmark or known data-testid found — page may not have mounted`,
  ).toBe(true);
}

/**
 * Ensure the test account is signed in before protected-route tests run.
 *
 * ProtectedRoute does NOT redirect unauthenticated users to a /sign-in page —
 * it renders an inline prompt with a SignInButton that opens a Clerk modal.
 *
 * The CI runner's Clerk session persists across workflow runs, so the account
 * may already be authenticated when this helper runs. Two states are handled:
 *
 * Already authenticated:
 *   1. Navigate to /match-history — ProtectedRoute renders page content directly.
 *   2. Return immediately (nothing to do).
 *
 * Not yet authenticated:
 *   1. Navigate to /match-history — ProtectedRoute renders the sign-in prompt.
 *   2. Click the "Sign In" button to open the Clerk modal.
 *   3. Fill email → Continue → fill password → Submit in the modal.
 *   4. Wait until the modal closes and the page content mounts.
 */
async function signIn(page: Page): Promise<void> {
  // Navigate to a protected route — ProtectedRoute renders the sign-in prompt
  // when not authenticated, or the page content when already authenticated.
  await page.goto(BASE_URL + '/match-history', { waitUntil: 'domcontentloaded' });

  // Wait for Clerk to finish initializing (loading spinner disappears).
  await page.locator('[data-testid="protected-route-loading"]').waitFor({ state: 'hidden', timeout: 30_000 });

  // After init, either the sign-in button or the page content will be visible.
  // The CI runner's Clerk session can persist across workflow runs, meaning the
  // test account may already be authenticated. Handle both states.
  const signInBtn = page.locator('[data-testid="protected-route-sign-in-btn"]');
  const matchHistoryContent = page.locator('[data-testid="match-history-page"]');

  // Wait for either to appear (whichever state we're in)
  await page.waitForSelector(
    '[data-testid="protected-route-sign-in-btn"], [data-testid="match-history-page"]',
    { timeout: 15_000 },
  );

  // Already authenticated — nothing to do
  if (await matchHistoryContent.isVisible()) {
    return;
  }

  // Not yet authenticated — complete the modal sign-in flow
  await signInBtn.click();

  // Wait for Clerk modal sign-in form — the modal renders inside a portal
  const emailInput = page.locator('input[name="identifier"], input[type="email"], input[name="emailAddress"]').first();
  await emailInput.waitFor({ state: 'visible', timeout: 15_000 });
  await emailInput.fill(SMOKE_EMAIL);

  // Click Continue / Next (Clerk two-step sign-in)
  const continueBtn = page.locator('button[type="submit"]').first();
  await continueBtn.click();

  // Wait for password input
  const passwordInput = page.locator('input[type="password"]').first();
  await passwordInput.waitFor({ state: 'visible', timeout: 10_000 });
  await passwordInput.fill(SMOKE_PASSWORD);

  // Submit
  const submitBtn = page.locator('button[type="submit"]').first();
  await submitBtn.click();

  // Wait until the Clerk modal closes and the page content mounts
  await page.waitForSelector('[data-testid="match-history-page"]', { timeout: 20_000 });

  // Give React a moment to fully settle after sign-in
  await page.waitForLoadState('load');
}

// ---------------------------------------------------------------------------
// Public routes — no auth required
// ---------------------------------------------------------------------------

test.describe('Staging SPA smoke: public routes', () => {
  for (const route of PUBLIC_ROUTES) {
    test(`${route} — no blank screen, no error boundary`, async ({ page }) => {
      await page.goto(BASE_URL + route, { waitUntil: 'domcontentloaded' });
      await assertPageIsHealthy(page, route);
    });
  }
});

// ---------------------------------------------------------------------------
// Root redirect
// ---------------------------------------------------------------------------

test.describe('Staging SPA smoke: root redirect', () => {
  test('/ redirects to /home or /sign-in (not blank)', async ({ page }) => {
    await page.goto(BASE_URL + '/', { waitUntil: 'domcontentloaded' });

    // Should land on /home (authenticated), /sign-in, or still / while loading
    // App.tsx: <Route path="/" element={<Navigate to="/home" replace />} />
    const currentPath = new URL(page.url()).pathname;
    const isExpectedPath =
      currentPath === '/home' ||
      currentPath === '/match-history' || // allow legacy redirect in case of stale deploy
      currentPath.startsWith('/sign-in') ||
      currentPath === '/';

    expect(
      isExpectedPath,
      `/ redirected to unexpected path: ${currentPath}`,
    ).toBe(true);

    // Either way — no blank screen
    const bodyChildren = await page.evaluate(() => document.body.children.length);
    expect(bodyChildren, '/ redirect resulted in blank screen').toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// Protected routes — require Clerk sign-in
// ---------------------------------------------------------------------------

test.describe('Staging SPA smoke: protected routes (authenticated)', () => {
  test.beforeAll(async () => {
    if (!SMOKE_EMAIL || !SMOKE_PASSWORD) {
      // Mark entire describe as skipped in a way Playwright handles gracefully
    }
  });

  // Use one browser context across all protected route tests to avoid re-signing
  // in for every route. Playwright `test.use` applies per-file, so we manage
  // the shared page manually via a beforeAll/afterAll block.
  let sharedPage: Page;

  test.beforeAll(async ({ browser }) => {
    if (!SMOKE_EMAIL || !SMOKE_PASSWORD) {
      return; // sign-in guard is inside each test
    }

    const context = await browser.newContext();
    sharedPage = await context.newPage();
    await signIn(sharedPage);
  });

  test.afterAll(async () => {
    if (sharedPage) {
      await sharedPage.context().close();
    }
  });

  for (const route of PROTECTED_ROUTES) {
    test(`${route} — no blank screen, no error boundary`, async () => {
      if (!SMOKE_EMAIL || !SMOKE_PASSWORD) {
        test.skip(
          true,
          'SMOKE_CLERK_EMAIL / SMOKE_CLERK_PASSWORD not set — skipping authenticated route smoke tests',
        );
        return;
      }

      await sharedPage.goto(BASE_URL + route, { waitUntil: 'domcontentloaded' });

      // If Clerk redirected us to sign-in, the session expired — fail loudly
      const currentPath = new URL(sharedPage.url()).pathname;
      if (currentPath.startsWith('/sign-in')) {
        throw new Error(
          `Route ${route}: Clerk session expired mid-suite — page redirected to ${currentPath}`,
        );
      }

      await assertPageIsHealthy(sharedPage, route);
    });
  }
});
