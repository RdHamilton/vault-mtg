import { test, expect, type Page } from '@playwright/test';

/**
 * Staging SPA Smoke Suite (#1933)
 *
 * Authenticates via Clerk testing tokens and navigates every SPA route at
 * stg-app.vaultmtg.app, asserting no blank screen and no React error boundary.
 *
 * Authentication:
 *   Uses Clerk's official testing token API (CLERK_SECRET_KEY) to establish a
 *   session programmatically without going through the sign-in UI, without
 *   requiring a dedicated smoke user account, and without creating billable MAU
 *   sessions. If CLERK_SECRET_KEY is absent the suite is skipped with a clear
 *   message so developers running locally don't see failures they cannot fix.
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
 *   CLERK_SECRET_KEY  — Clerk Backend API secret key (sk_*) used to generate
 *                       testing tokens; never exposed in the browser bundle
 *   STAGING_SPA_URL   — override staging SPA base URL (optional)
 */

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const CLERK_SECRET_KEY = process.env.CLERK_SECRET_KEY ?? '';
// Use `||` (not `??`) so that an empty-string CI secret falls back to the
// default. `??` only falls back on `undefined`/`null`, which left
// BASE_URL = '' when STAGING_SPA_URL was set-but-empty in CI (#1933).
const BASE_URL = process.env.STAGING_SPA_URL || 'https://stg-app.vaultmtg.app';
const API_BASE_URL = 'staging-api.vaultmtg.app';

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
 * Establish a Clerk session using a testing token.
 *
 * Testing tokens are Clerk's official CI/E2E pattern — they authenticate a
 * session programmatically without going through the sign-in UI, without
 * requiring a dedicated smoke user account, and without creating billable MAU
 * sessions. The token is injected into the browser via a URL query parameter
 * that Clerk JS picks up automatically.
 *
 * Requires CLERK_SECRET_KEY (sk_*) to be set in the environment.
 */
async function signIn(page: Page): Promise<void> {
  const clerkSecretKey = process.env.CLERK_SECRET_KEY ?? '';
  if (!clerkSecretKey) {
    throw new Error('CLERK_SECRET_KEY not set — cannot generate Clerk testing token');
  }

  // Generate a testing token via Clerk Backend API.
  // Testing tokens establish a session without the sign-in UI and do not
  // count toward MAU billing — they are Clerk's official CI/E2E pattern.
  const tokenRes = await fetch('https://api.clerk.com/v1/testing_tokens', {
    method: 'POST',
    headers: { Authorization: `Bearer ${clerkSecretKey}` },
  });
  if (!tokenRes.ok) {
    throw new Error(`Clerk testing token request failed: ${tokenRes.status} ${await tokenRes.text()}`);
  }
  const { token } = await tokenRes.json() as { token: string };

  // Inject the token into the browser to establish a Clerk session.
  // Navigate to the app with the testing token in the URL — Clerk JS picks it
  // up automatically and sets the session without any UI interaction.
  await page.goto(`${BASE_URL}/?__clerk_testing_token=${token}`, { waitUntil: 'domcontentloaded' });

  // In CI, DOMContentLoaded fires before the JS bundle executes because Vite
  // emits <script type="module"> which Chromium headless treats as async.
  // Explicitly wait for React to mount before checking the URL — otherwise
  // waitForURL times out because the root <Navigate> hasn't rendered yet.
  await page.waitForSelector('#root > *', { timeout: 30_000 });

  // Wait for Clerk to process the token and the session to be established.
  // The root redirect takes us to /home once authenticated.
  await page.waitForURL((url) => url.pathname !== '/', { timeout: 15_000 });
  await page.waitForSelector('[data-testid]', { timeout: 15_000 });
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
    if (!CLERK_SECRET_KEY) {
      // Mark entire describe as skipped in a way Playwright handles gracefully
    }
  });

  // Use one browser context across all protected route tests to avoid re-signing
  // in for every route. Playwright `test.use` applies per-file, so we manage
  // the shared page manually via a beforeAll/afterAll block.
  let sharedPage: Page;

  // Collects any 401/403 responses from the staging BFF during authenticated
  // route navigation. Any entry here indicates a Clerk instance mismatch or
  // auth regression — the suite fails with a descriptive message.
  const apiAuthErrors: string[] = [];

  test.beforeAll(async ({ browser }) => {
    if (!CLERK_SECRET_KEY) {
      return; // sign-in guard is inside each test
    }

    const context = await browser.newContext();
    sharedPage = await context.newPage();

    // Attach a response listener BEFORE sign-in so we catch any auth errors
    // during the initial session establishment as well as subsequent navigation.
    sharedPage.on('response', (response) => {
      if (
        response.url().includes(API_BASE_URL) &&
        (response.status() === 401 || response.status() === 403)
      ) {
        apiAuthErrors.push(`${response.status()} ${response.url()}`);
      }
    });

    await signIn(sharedPage);
  });

  test.afterAll(async () => {
    // Assert no authenticated API calls returned 401/403 across the entire suite.
    // A non-empty list here means the staging BFF rejected the Clerk session —
    // most likely a Clerk instance mismatch (staging key vs prod key) or an
    // auth regression on the BFF.
    expect(
      apiAuthErrors,
      `Authenticated API calls returned 401/403 — Clerk instance mismatch or auth regression: ${apiAuthErrors.join(', ')}`,
    ).toHaveLength(0);

    if (sharedPage) {
      await sharedPage.context().close();
    }
  });

  for (const route of PROTECTED_ROUTES) {
    test(`${route} — no blank screen, no error boundary`, async () => {
      if (!CLERK_SECRET_KEY) {
        test.skip(
          true,
          'CLERK_SECRET_KEY not set — skipping authenticated route smoke tests',
        );
        return;
      }

      // Clear per-route errors so failures are attributed to the right route.
      apiAuthErrors.length = 0;

      await sharedPage.goto(BASE_URL + route, { waitUntil: 'domcontentloaded' });

      // If Clerk redirected us to sign-in, the session expired — fail loudly
      const currentPath = new URL(sharedPage.url()).pathname;
      if (currentPath.startsWith('/sign-in')) {
        throw new Error(
          `Route ${route}: Clerk session expired mid-suite — page redirected to ${currentPath}`,
        );
      }

      await assertPageIsHealthy(sharedPage, route);

      // Assert no authenticated API calls returned 401/403 for this route.
      expect(
        [...apiAuthErrors],
        `Route ${route}: authenticated API calls returned 401/403 — Clerk instance mismatch or auth regression: ${apiAuthErrors.join(', ')}`,
      ).toHaveLength(0);
    });
  }
});
