import { test, expect } from '@playwright/test';

/**
 * Collection Page E2E Tests
 *
 * Tests the Collection page functionality including navigation and filters.
 * Uses REST API backend for testing.
 *
 * /collection is behind ProtectedRoute. Tests inject a signed-in Clerk test
 * state via window.__CLERK_TEST_STATE__ so ProtectedRoute renders the
 * Collection content rather than the sign-in prompt. This requires Playwright
 * to be started with VITE_CLERK_TEST_MODE=true (set in playwright.config.ts
 * webServer command).
 *
 * Fixes: https://github.com/RdHamilton/MTGA-Companion/issues/1459
 * Root cause of prior skip: missing Clerk test state injection meant
 * ProtectedRoute redirected to sign-in and the page never loaded.
 */
test.describe('Collection', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through to Collection content.
    await page.addInitScript(() => {
      (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = { isSignedIn: true };
    });

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15000 });

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
      await expect(header).toBeVisible({ timeout: 10000 });
    });

    test('should display collection stats summary', async ({ page }) => {
      const stats = page.locator('[data-testid="collection-stats"]');
      await expect(stats).toBeVisible({ timeout: 10000 });
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
      await expect(collectionPage).toBeVisible({ timeout: 15000 });

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
      await expect(page.locator('[data-testid="collection-page"]')).toBeVisible({ timeout: 15000 });

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
      await expect(page.locator('[data-testid="collection-page"]')).toBeVisible({ timeout: 15000 });

      const errorState = page.locator('[data-testid="collection-error"]');
      await expect(errorState).not.toBeVisible();
    });
  });
});
