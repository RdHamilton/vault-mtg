import { test, expect, type Page } from '@playwright/test';

/**
 * Collection Page E2E Tests (#1459, #2178)
 *
 * Tests the Collection page functionality including navigation and filters.
 *
 * /collection is behind ProtectedRoute. Tests inject a signed-in Clerk test
 * state via window.__CLERK_TEST_STATE__ so ProtectedRoute renders the
 * Collection content rather than the sign-in prompt. This requires Playwright
 * to be started with VITE_CLERK_TEST_MODE=true (set in playwright.config.ts
 * webServer command).
 *
 * BFF-data mocking (#2178): in CI the BFF runs with a Clerk secret that does
 * not accept the Clerk mock's stub token, so the real Clerk-protected
 * /api/v1/collection/* and /api/v1/cards/* endpoints reject every request and
 * the page renders its error state instead of the collection UI. To keep these
 * tests independent of a live authenticated BFF, the collection and card
 * endpoints are mocked via page.route() before navigation.
 *
 * Root cause of prior failure: the Clerk mock's stub token was rejected by the
 * real Clerk-backed BFF, so getCollectionWithMetadata() / getAllSetInfo() threw
 * and the Collection page never rendered its content.
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Inject signed-in Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedIn(page: Page): Promise<void> {
  await page.addInitScript(() => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = { isSignedIn: true };
  });
}

/**
 * Mock the Clerk-protected collection + card endpoints so the Collection page
 * renders without a live authenticated BFF. Registered before page.goto().
 *
 * Empty-but-valid envelopes are returned so the page reaches its rendered
 * (empty) state rather than an error state.
 *
 * Response envelope: the shared apiClient (services/apiClient.ts) unwraps every
 * response as `data.data`, so every body below is a `{ "data": <payload> }`
 * envelope — a bare object/array would be dropped.
 */
async function mockCollectionEndpoints(page: Page): Promise<void> {
  // POST /api/v1/collection — the main collection query.
  await page.route('**/api/v1/collection', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          cards: [],
          totalCount: 0,
          filterCount: 0,
          unknownCardsRemaining: 0,
          unknownCardsFetched: 0,
        },
      }),
    });
  });

  // GET /api/v1/collection/value — collection value summary.
  await page.route('**/api/v1/collection/value', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          totalValueUsd: 0,
          totalValueEur: 0,
          uniqueCardsWithPrice: 0,
          cardCount: 0,
          valueByRarity: {},
          topCards: [],
        },
      }),
    });
  });

  // GET /api/v1/collection/stats and /collection/sets — defensive coverage.
  await page.route('**/api/v1/collection/stats', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: {} }),
    });
  });
  await page.route('**/api/v1/collection/sets', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [] }),
    });
  });

  // GET /api/v1/cards/sets — set metadata used by the set filter.
  await page.route('**/api/v1/cards/sets', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [] }),
    });
  });
}

test.describe('Collection', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through to Collection content.
    await setClerkSignedIn(page);
    // Mock the BFF data endpoints so the page does not depend on a live authenticated BFF.
    await mockCollectionEndpoints(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await page.click('a[href="/collection"]');
    await page.waitForURL('**/collection');
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should navigate to Collection page', async ({ page }) => {
      // Verify URL is collection
      await expect(page).toHaveURL(/.*\/collection/);

      // Wait for page content
      await expect(page.locator('h1')).toContainText('Collection', { timeout: 20000 });
    });

    test('should display page title', async ({ page }) => {
      await expect(page.locator('h1')).toContainText('Collection');
    });
  });

  test.describe('Collection Header', () => {
    test('should display collection header', async ({ page }) => {
      const header = page.locator('[data-testid="collection-header"]');
      await expect(header).toBeVisible();
    });

    test('should display collection stats summary', async ({ page }) => {
      const stats = page.locator('[data-testid="collection-stats"]');
      await expect(stats).toBeVisible();
    });
  });

  test.describe('Filter Controls', () => {
    test('should have search input', async ({ page }) => {
      const searchInput = page.locator('[data-testid="collection-search-input"]');
      await expect(searchInput).toBeVisible({ timeout: 5000 });
    });

    test('should have set filter dropdown', async ({ page }) => {
      const setFilter = page.locator('[data-testid="collection-set-filter"]');
      await expect(setFilter).toBeVisible({ timeout: 5000 });
    });

    test('should have rarity filter', async ({ page }) => {
      const rarityFilter = page.locator('[data-testid="collection-rarity-filter"]');
      await expect(rarityFilter).toBeVisible({ timeout: 5000 });
    });

    test('should have sort dropdown', async ({ page }) => {
      const sortSelect = page.locator('[data-testid="collection-sort-select"]');
      await expect(sortSelect).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Collection Content', () => {
    test('should display collection cards or empty state', async ({ page }) => {
      const collectionPage = page.locator('[data-testid="collection-page"]');

      // Wait for the page container — loading state resolves before this renders
      await expect(collectionPage).toBeVisible();

      const cardGrid = page.locator('[data-testid="collection-card-grid"]');
      const emptyState = page.locator('[data-testid="collection-empty"]');

      const hasCards = await cardGrid.isVisible().catch(() => false);
      const hasEmptyState = await emptyState.isVisible().catch(() => false);

      expect(hasCards || hasEmptyState).toBeTruthy();
    });
  });

  test.describe('Set Completion Toggle', () => {
    test('should show Set Completion button when a set is selected', async ({ page }) => {
      // Wait for page to load
      await expect(page.locator('[data-testid="collection-page"]')).toBeVisible();

      // Select a set — button only appears when setCode filter is active
      const setFilter = page.locator('[data-testid="collection-set-filter"]');
      await expect(setFilter).toBeVisible({ timeout: 5000 });

      const options = await setFilter.locator('option').allInnerTexts();
      // Skip if only the "All Sets" placeholder exists (no set data loaded)
      test.skip(options.length <= 1, 'No set data available in this environment');

      await setFilter.selectOption({ index: 1 });

      const setCompletionButton = page.locator('[data-testid="collection-toggle-set-completion"]');
      await expect(setCompletionButton).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Loading State', () => {
    test('should not show error state on initial load', async ({ page }) => {
      // Wait for page container
      await expect(page.locator('[data-testid="collection-page"]')).toBeVisible();

      const errorState = page.locator('[data-testid="collection-error"]');
      await expect(errorState).not.toBeVisible();
    });
  });
});
