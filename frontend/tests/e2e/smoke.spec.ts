import { test, expect } from '@playwright/test';

/**
 * Vercel BFF Connectivity Smoke Tests (#1243)
 *
 * Verifies that the SPA loads correctly and that the BFF API is reachable
 * from the deployed Vercel frontend without CORS errors.
 *
 * Run against a Vercel preview URL:
 *   PLAYWRIGHT_BASE_URL=https://<preview>.vercel.app npx playwright test tests/e2e/smoke.spec.ts
 *
 * Note: The baseURL in playwright.config.ts defaults to localhost:3000.
 * Override with PLAYWRIGHT_BASE_URL env var when targeting Vercel previews.
 *
 * Coverage gap: Automated CORS header inspection requires a live preview URL
 * with a distinct origin. Run this suite against the Vercel preview URL and
 * verify no console errors manually after the CI preview build completes.
 */

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000';
const BFF_URL = process.env.VITE_API_BASE_URL ?? 'http://localhost:8080';

test.describe('@smoke Vercel BFF Connectivity', () => {
  test('SPA loads — root element visible', async ({ page }) => {
    await page.goto(BASE_URL);

    // The app container must be present (timeout governed by global expect.timeout: 30_000).
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('SPA loads — page has a non-empty title', async ({ page }) => {
    await page.goto(BASE_URL);

    const title = await page.title();
    expect(title.length).toBeGreaterThan(0);
  });

  test('BFF health endpoint reachable — no network error', async ({ page }) => {
    // Collect console errors to detect CORS failures.
    const consoleErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });

    // Navigate to the SPA first so fetch runs from the app origin.
    await page.goto(BASE_URL);

    // Call the health endpoint from inside the browser context.
    const result = await page.evaluate(async (bffUrl: string) => {
      try {
        const res = await fetch(`${bffUrl}/api/v1/health`, {
          method: 'GET',
          credentials: 'include',
        });
        return { ok: true, status: res.status };
      } catch (err) {
        return { ok: false, status: 0, error: String(err) };
      }
    }, BFF_URL);

    // If the endpoint returns any HTTP response (even 401/403 for auth-gated
    // routes), a CORS preflight succeeded and the BFF is reachable.
    expect(result.ok, `BFF fetch threw a network error: ${(result as { error?: string }).error ?? 'unknown'}`).toBe(true);

    // Filter out unrelated extension/noise errors; flag genuine CORS errors.
    const corsErrors = consoleErrors.filter((e) =>
      e.toLowerCase().includes('cors') || e.toLowerCase().includes('cross-origin')
    );
    expect(corsErrors, `CORS errors detected: ${corsErrors.join(', ')}`).toHaveLength(0);
  });

  test('BFF decks endpoint reachable — no network error', async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });

    await page.goto(BASE_URL);

    const result = await page.evaluate(async (bffUrl: string) => {
      try {
        const res = await fetch(`${bffUrl}/api/v1/decks`, {
          method: 'GET',
          credentials: 'include',
        });
        // Any HTTP status (including 401/403) means CORS passed and BFF responded.
        return { ok: true, status: res.status };
      } catch (err) {
        return { ok: false, status: 0, error: String(err) };
      }
    }, BFF_URL);

    expect(result.ok, `BFF /api/v1/decks threw a network error: ${(result as { error?: string }).error ?? 'unknown'}`).toBe(true);

    const corsErrors = consoleErrors.filter((e) =>
      e.toLowerCase().includes('cors') || e.toLowerCase().includes('cross-origin')
    );
    expect(corsErrors, `CORS errors on /api/v1/decks: ${corsErrors.join(', ')}`).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// EnvBadge smoke assertions (#1495)
//
// The EnvBadge component is gated by import.meta.env.MODE !== 'production'.
// In development / preview / staging the badge renders with data-testid="env-badge".
// In production builds it returns null — the element must NOT be present.
//
// The dev server (playwright.config.ts webServer) starts Vite in development
// mode, so MODE is always 'development' during local Playwright runs and in CI
// preview deployments — the badge WILL be visible.
//
// Against the production CloudFront URL (PLAYWRIGHT_BASE_URL=https://app.vaultmtg.app)
// the badge must be absent. The test is skipped when no production URL is provided
// rather than failing, so CI stays green on preview runs.
// ---------------------------------------------------------------------------

const IS_PRODUCTION_URL =
  BASE_URL.includes('app.vaultmtg.app') ||
  process.env.PLAYWRIGHT_ENV === 'production';

test.describe('@smoke EnvBadge visibility', () => {
  test('EnvBadge is visible in development / preview build', async ({ page }) => {
    test.skip(IS_PRODUCTION_URL, 'Skipped: targeting a production URL where EnvBadge is hidden');

    await page.goto(BASE_URL);
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await expect(page.locator('[data-testid="env-badge"]')).toBeVisible();
  });

  test('EnvBadge is NOT present in production build', async ({ page }) => {
    test.skip(!IS_PRODUCTION_URL, 'Skipped: not targeting a production URL');

    await page.goto(BASE_URL);
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await expect(page.locator('[data-testid="env-badge"]')).not.toBeAttached();
  });
});
