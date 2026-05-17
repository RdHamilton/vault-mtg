import { test, expect } from '@playwright/test';

/**
 * Vercel BFF Connectivity tests (#1243)
 *
 * Verifies that the SPA loads correctly and that the BFF API is reachable
 * from a *deployed* frontend without CORS errors.
 *
 * Run against a Vercel preview URL:
 *   PLAYWRIGHT_BASE_URL=https://<preview>.vercel.app npx playwright test tests/e2e/smoke.spec.ts
 *
 * These tests are intentionally NOT tagged @smoke (#2178):
 * the smoke project's CI webServer builds a production bundle and runs the BFF
 * on localhost. The cross-origin CORS path these tests exercise only exists on
 * a real Vercel preview deployment with a distinct origin — against localhost
 * the SPA and BFF share an origin so there is no CORS preflight to verify, and
 * the BFF connectivity probes are redundant with the page-level smoke checks in
 * the page-specific specs. Keep them in the `full` project, run against a live
 * preview URL via PLAYWRIGHT_BASE_URL.
 *
 * Coverage gap: Automated CORS header inspection requires a live preview URL
 * with a distinct origin. Run this suite against the Vercel preview URL and
 * verify no console errors manually after the CI preview build completes.
 */

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000';
const BFF_URL = process.env.VITE_API_BASE_URL ?? 'http://localhost:8080';

test.describe('Vercel BFF Connectivity', () => {
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
// EnvBadge visibility (#1495, #2178)
//
// The EnvBadge component is gated by import.meta.env.MODE !== 'production'.
// In development / preview / staging the badge renders with data-testid="env-badge".
// In production builds it returns null AND is dead-code-eliminated by the bundler.
//
// The smoke project's CI webServer runs `vite build` (a *production*-mode build)
// then `vite preview`, so MODE === 'production' there — the badge is absent and
// the element is tree-shaken out. The dev-build badge assertion therefore cannot
// be validated by the smoke webServer and must NOT be @smoke-tagged (#2178).
//
// - "EnvBadge is visible in development build" runs only against an explicitly
//   dev-mode target (local `vite dev`, or PLAYWRIGHT_ENV=development) and is
//   skipped otherwise. Not @smoke.
// - "EnvBadge is NOT present in production build" verifies the production
//   guarantee. It runs against the production bundle the smoke webServer builds,
//   or against the production CloudFront URL, and IS @smoke-tagged.
// ---------------------------------------------------------------------------

const IS_PRODUCTION_URL =
  BASE_URL.includes('app.vaultmtg.app') ||
  process.env.PLAYWRIGHT_ENV === 'production';

// In CI the smoke webServer builds a production bundle, so the badge is absent.
// Treat the smoke run as a production-mode target unless explicitly told the
// target was built in development mode.
const IS_DEV_BUILD = process.env.PLAYWRIGHT_ENV === 'development';

test.describe('EnvBadge visibility', () => {
  test('EnvBadge is visible in a development build', async ({ page }) => {
    test.skip(
      !IS_DEV_BUILD,
      'Skipped: the smoke webServer builds a production bundle (EnvBadge is tree-shaken). ' +
        'Run with PLAYWRIGHT_ENV=development against a dev build to validate.',
    );

    await page.goto(BASE_URL);
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await expect(page.locator('[data-testid="env-badge"]')).toBeVisible();
  });

  test('@smoke EnvBadge is NOT present in a production build', async ({ page }) => {
    test.skip(
      IS_DEV_BUILD,
      'Skipped: target was built in development mode where EnvBadge is intentionally visible.',
    );

    await page.goto(BASE_URL);
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // In a production build the badge returns null and is dead-code-eliminated.
    await expect(page.locator('[data-testid="env-badge"]')).not.toBeAttached();
  });

  test('production CloudFront URL hides EnvBadge', async ({ page }) => {
    test.skip(!IS_PRODUCTION_URL, 'Skipped: not targeting the production CloudFront URL');

    await page.goto(BASE_URL);
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await expect(page.locator('[data-testid="env-badge"]')).not.toBeAttached();
  });
});
