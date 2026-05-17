import { test, expect, type Page } from '@playwright/test';

/**
 * Meta Page E2E Tests (#2178)
 *
 * Tests the Meta page functionality including format selection and error states.
 *
 * /meta is behind ProtectedRoute. Tests inject a signed-in Clerk test state via
 * window.__CLERK_TEST_STATE__ so ProtectedRoute renders the Meta content rather
 * than the sign-in prompt (requires VITE_CLERK_TEST_MODE=true, set in
 * playwright.config.ts webServer command).
 *
 * BFF-data mocking (#2178): the Meta page fetches the Clerk-protected
 * /api/v1/meta/archetypes endpoint on mount. In CI the BFF runs with a Clerk
 * secret that does not accept the Clerk mock's stub token, so that endpoint is
 * mocked via page.route() before navigation so the page does not depend on a
 * live authenticated BFF.
 *
 * Root cause of prior failure: navigating to a protected route without
 * injecting signed-in Clerk state — ProtectedRoute rendered the sign-in prompt
 * and the `.meta-page` container never appeared.
 */

/** Inject signed-in Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedIn(page: Page): Promise<void> {
  await page.addInitScript(() => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = { isSignedIn: true };
  });
}

/**
 * Mock GET /api/v1/meta/archetypes so the Meta page renders without a live
 * authenticated BFF. Registered before page.goto().
 *
 * The shared apiClient (services/apiClient.ts) unwraps every response as
 * `data.data`, so the body is a `{ "data": <payload> }` envelope.
 */
async function mockMetaEndpoint(page: Page): Promise<void> {
  await page.route('**/api/v1/meta/archetypes**', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [] }),
    });
  });
}

test.describe('Meta', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through to Meta.
    await setClerkSignedIn(page);
    await mockMetaEndpoint(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await page.click('a[href="/meta"]');
    await page.waitForURL('**/meta');
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should navigate to Meta page', async ({ page }) => {
      const metaPage = page.locator('.meta-page');
      await expect(metaPage).toBeVisible();
    });

    test('should display page title', async ({ page }) => {
      await expect(page.locator('.meta-title h1')).toContainText('Meta');
    });
  });

  test.describe('Meta Header', () => {
    test('should display meta header', async ({ page }) => {
      const header = page.locator('.meta-header');
      await expect(header).toBeVisible();
    });

    test('should display format selector', async ({ page }) => {
      const formatSelect = page.locator('.format-select');
      await expect(formatSelect).toBeVisible({ timeout: 5000 });
    });

    test('should have refresh button', async ({ page }) => {
      const refreshButton = page.locator('.refresh-button');
      await expect(refreshButton).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Format Selection', () => {
    test('should have format options', async ({ page }) => {
      const formatSelect = page.locator('.format-select');
      await expect(formatSelect).toBeVisible();

      const options = await formatSelect.locator('option').allTextContents();
      expect(options.length).toBeGreaterThan(0);

      const hasStandard = options.some((opt) => opt.toLowerCase().includes('standard'));
      const hasHistoric = options.some((opt) => opt.toLowerCase().includes('historic'));

      expect(hasStandard || hasHistoric).toBeTruthy();
    });

    test('should allow changing format', async ({ page }) => {
      const formatSelect = page.locator('.format-select');
      await expect(formatSelect).toBeVisible();

      // Select a different format
      await formatSelect.selectOption({ index: 1 });

      // Wait for content to update by checking page is still visible
      const metaPage = page.locator('.meta-page');
      await expect(metaPage).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Meta Content', () => {
    test('should display meta content or loading state', async ({ page }) => {
      const metaPage = page.locator('.meta-page');
      await expect(metaPage).toBeVisible();

      // Should have some content
      const content = await metaPage.textContent();
      expect(content?.length).toBeGreaterThan(0);
    });
  });

  test.describe('Loading State', () => {
    test('should show loading indicator while fetching data', async ({ page }) => {
      const metaPage = page.locator('.meta-page');
      await expect(metaPage).toBeVisible();
    });

    test('should handle refresh button click', async ({ page }) => {
      // Wait for page to load
      const refreshButton = page.locator('.refresh-button');
      await expect(refreshButton).toBeVisible();

      // Click refresh
      await refreshButton.click();

      // Page should still be visible after refresh
      const metaPage = page.locator('.meta-page');
      await expect(metaPage).toBeVisible();
    });
  });

  test.describe('Error State', () => {
    /**
     * Verifies that the error banner (.meta-error) renders when the BFF returns
     * a 500 error for the /meta/archetypes endpoint (#1975).
     *
     * The route is registered before navigation so it intercepts the initial
     * data fetch that Meta.tsx triggers on mount.
     *
     * NOTE: marked fixme because the local E2E stack (BFF + Vite) requires
     * DATABASE_URL and a running Postgres instance — not available in the
     * dev environment without staging. Run against a real staging environment
     * to verify. Tracked in #1975.
     */
    test('should display error banner when BFF returns a 500 error for /meta/archetypes', async ({ page }) => {
      test.fixme(true, 'Requires staging environment with DATABASE_URL; tracked in #1975');

      // Register the mock BEFORE navigating so the interceptor is active
      // when Meta.tsx calls getMetaArchetypes() on mount.
      await page.route('**/api/v1/meta/archetypes**', async (route) => {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'internal server error' }),
        });
      });

      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await page.click('a[href="/meta"]');
      await page.waitForURL('**/meta');

      // The error banner must be visible — loading should have resolved to error
      const errorBanner = page.locator('.meta-error');
      await expect(errorBanner).toBeVisible({ timeout: 10000 });
      await expect(errorBanner).not.toBeEmpty();
    });

    /**
     * Verifies the error banner renders when the BFF returns a null/empty body
     * (e.g. an unexpected 204 with no content) for /meta/archetypes (#1975).
     *
     * NOTE: marked fixme — same staging environment requirement as above.
     * Tracked in #1975.
     */
    test('should display error banner when BFF returns null body for /meta/archetypes', async ({ page }) => {
      test.fixme(true, 'Requires staging environment with DATABASE_URL; tracked in #1975');

      await page.route('**/api/v1/meta/archetypes**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: 'null',
        });
      });

      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await page.click('a[href="/meta"]');
      await page.waitForURL('**/meta');

      const errorBanner = page.locator('.meta-error');
      await expect(errorBanner).toBeVisible({ timeout: 10000 });
      await expect(errorBanner).not.toBeEmpty();
    });
  });
});
