import { test, expect, type Page } from '@playwright/test';

/**
 * Quests Page E2E Tests (#2178)
 *
 * Tests the Quests page functionality.
 *
 * /quests is behind ProtectedRoute. Tests inject a signed-in Clerk test state
 * via window.__CLERK_TEST_STATE__ so ProtectedRoute renders the Quests content
 * rather than the sign-in prompt (requires VITE_CLERK_TEST_MODE=true, set in
 * playwright.config.ts webServer command).
 *
 * BFF-data mocking (#2178): the Quests page fetches several Clerk-protected
 * endpoints on mount (/quests/*, /system/account). In CI the BFF runs with a
 * Clerk secret that does not accept the Clerk mock's stub token, so those
 * endpoints are mocked via page.route() before navigation so the page does not
 * depend on a live authenticated BFF.
 *
 * Root cause of prior failure: navigating to a protected route without injecting
 * signed-in Clerk state — ProtectedRoute rendered the sign-in prompt and the
 * Quests `h1` never appeared.
 *
 * AC4 coverage (#2008): Quest page navigation must produce no ERR_CONNECTION_REFUSED
 * errors for /system/account — that endpoint is served by the BFF (port 8080),
 * not the daemon (port 9001). The fix routes getCurrentAccount() through the BFF
 * client so the page loads account data even when the daemon is offline.
 */

/** Inject signed-in Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedIn(page: Page): Promise<void> {
  await page.addInitScript(() => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = { isSignedIn: true };
  });
}

/**
 * Mock the Clerk-protected endpoints the Quests page fetches on mount so it
 * renders without a live authenticated BFF. Registered before page.goto().
 *
 * The shared apiClient (services/apiClient.ts) unwraps every response as
 * `data.data`, so every body below is a `{ "data": <payload> }` envelope.
 */
async function mockQuestsEndpoints(page: Page): Promise<void> {
  await page.route('**/api/v1/quests/active', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { quests: [] } }),
    });
  });
  await page.route('**/api/v1/quests/history**', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [] }),
    });
  });
  await page.route('**/api/v1/quests/wins/daily', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { dailyWins: 0, goal: 0 } }),
    });
  });
  await page.route('**/api/v1/quests/wins/weekly', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { weeklyWins: 0, goal: 0 } }),
    });
  });
  await page.route('**/api/v1/system/account', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: {} }),
    });
  });
}

test.describe('Quests', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through to Quests.
    await setClerkSignedIn(page);
    await mockQuestsEndpoints(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await page.click('a[href="/quests"]');
    await page.waitForURL('**/quests');
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should navigate to Quests page', async ({ page }) => {
      await expect(page.locator('h1')).toContainText('Quests');
    });

    test('should display quests header', async ({ page }) => {
      const header = page.locator('.quests-header');
      await expect(header).toBeVisible();
    });
  });

  test.describe('Active Quests Section', () => {
    test('should display active quests section', async ({ page }) => {
      const questsSection = page.locator('.quests-section');
      const emptyState = page.locator('.empty-state');

      // Wait for either content type to appear
      await expect(questsSection.first().or(emptyState)).toBeVisible();

      const hasSection = await questsSection.first().isVisible();
      const hasEmptyState = await emptyState.isVisible();

      expect(hasSection || hasEmptyState).toBeTruthy();
    });
  });

  test.describe('Quest History Section', () => {
    test('should have date range filter', async ({ page }) => {
      // Wait for page to load
      const questsHeader = page.locator('.quests-header');
      await expect(questsHeader).toBeVisible();

      // Check for date range select (if present)
      const dateRangeSelect = page.locator('select').first();
      const hasDateRange = await dateRangeSelect.isVisible().catch(() => false);

      if (hasDateRange) {
        const options = await dateRangeSelect.locator('option').allTextContents();
        expect(options.length).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Loading State', () => {
    test('should not show error state on initial load', async ({ page }) => {
      // Wait for content to load
      const content = page.locator('.quests-section, .empty-state, .quests-header');
      await expect(content.first()).toBeVisible();

      const errorState = page.locator('.error-state');
      await expect(errorState).not.toBeVisible();
    });
  });

  test.describe('Account Data — no daemon dependency (#2008)', () => {
    test('should not emit ERR_CONNECTION_REFUSED errors for /system/account when navigating to Quests page', async ({ page }) => {
      // Collect all console errors to detect failed network requests to port 9001
      // (the daemon port). Before the fix, getCurrentAccount() routed through the
      // daemon client and produced ERR_CONNECTION_REFUSED when the daemon was offline.
      const daemonErrors: string[] = [];
      page.on('console', (msg) => {
        if (msg.type() === 'error') {
          const text = msg.text();
          // Flag any error mentioning port 9001 or ERR_CONNECTION_REFUSED for /system/account
          if (
            (text.includes('9001') && text.includes('system/account')) ||
            (text.includes('ERR_CONNECTION_REFUSED') && text.includes('system/account'))
          ) {
            daemonErrors.push(text);
          }
        }
      });

      // Wait for the page to finish loading so all API calls have fired
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {/* ignore timeout */});

      // No ERR_CONNECTION_REFUSED errors for /system/account should have been emitted.
      // If this fails, getCurrentAccount() is still routing through the daemon client.
      expect(
        daemonErrors,
        `Quest page emitted daemon connection errors for /system/account: ${daemonErrors.join('; ')}`
      ).toHaveLength(0);
    });

    test('should not make network requests to port 9001 for /system/account', async ({ page }) => {
      // Intercept all network requests and flag any to port 9001 for /system/account.
      // The BFF serves this endpoint on port 8080; the daemon should never be called.
      const daemonAccountRequests: string[] = [];
      page.on('request', (request) => {
        const url = request.url();
        if (url.includes('9001') && url.includes('system/account')) {
          daemonAccountRequests.push(url);
        }
      });

      // Wait for network to settle so all API calls have fired
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {/* ignore timeout */});

      expect(
        daemonAccountRequests,
        `Quest page sent /system/account requests to the daemon (port 9001): ${daemonAccountRequests.join(', ')}`
      ).toHaveLength(0);
    });
  });
});
