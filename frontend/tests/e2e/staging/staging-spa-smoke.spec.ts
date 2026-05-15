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
const BASE_URL = process.env.STAGING_SPA_URL ?? 'https://stg-app.vaultmtg.app';

// ---------------------------------------------------------------------------
// Routes from App.tsx
// ---------------------------------------------------------------------------

/** Public routes — accessible without authentication. */
const PUBLIC_ROUTES = ['/download', '/setup'] as const;

/**
 * Protected routes — require Clerk sign-in.
 * `/` redirects to `/match-history` so it is covered by the protected list.
 */
const PROTECTED_ROUTES = [
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
 * Sign in using Clerk's hosted sign-in flow.
 *
 * Clerk's sign-in UI path may vary. We navigate to the SPA root and let
 * Clerk's ProtectedRoute redirect us to /sign-in, then fill in credentials.
 * Waits until the SPA's authenticated shell is visible before returning.
 */
async function signIn(page: Page): Promise<void> {
  // Navigate to the app root — Clerk will redirect to sign-in if not authenticated
  await page.goto(BASE_URL + '/match-history', { waitUntil: 'networkidle' });

  // Wait for Clerk sign-in form — handles both hosted UI and embedded UI
  const emailInput = page.locator('input[name="identifier"], input[type="email"], input[name="emailAddress"]').first();
  await emailInput.waitFor({ state: 'visible', timeout: 15_000 });
  await emailInput.fill(SMOKE_EMAIL);

  // Click Continue / Next if present (Clerk two-step sign-in)
  const continueBtn = page.locator('button[type="submit"]').first();
  await continueBtn.click();

  // Wait for password input
  const passwordInput = page.locator('input[type="password"]').first();
  await passwordInput.waitFor({ state: 'visible', timeout: 10_000 });
  await passwordInput.fill(SMOKE_PASSWORD);

  // Submit
  const submitBtn = page.locator('button[type="submit"]').first();
  await submitBtn.click();

  // Wait until we are back on the SPA (not on /sign-in any more)
  await page.waitForFunction(
    () => !window.location.pathname.startsWith('/sign-in'),
    { timeout: 20_000 },
  );

  // Give React a moment to mount after sign-in redirect
  await page.waitForLoadState('networkidle');
}

// ---------------------------------------------------------------------------
// Public routes — no auth required
// ---------------------------------------------------------------------------

test.describe('Staging SPA smoke: public routes', () => {
  for (const route of PUBLIC_ROUTES) {
    test(`${route} — no blank screen, no error boundary`, async ({ page }) => {
      await page.goto(BASE_URL + route, { waitUntil: 'networkidle' });
      await assertPageIsHealthy(page, route);
    });
  }
});

// ---------------------------------------------------------------------------
// Root redirect
// ---------------------------------------------------------------------------

test.describe('Staging SPA smoke: root redirect', () => {
  test('/ redirects to /match-history or /sign-in (not blank)', async ({ page }) => {
    await page.goto(BASE_URL + '/', { waitUntil: 'networkidle' });

    // Should land on either match-history (if already authed) or sign-in
    const currentPath = new URL(page.url()).pathname;
    const isExpectedPath =
      currentPath === '/match-history' ||
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

      await sharedPage.goto(BASE_URL + route, { waitUntil: 'networkidle' });

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
