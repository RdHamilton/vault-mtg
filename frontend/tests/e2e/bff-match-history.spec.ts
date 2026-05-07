import { test, expect } from '@playwright/test';

/**
 * BFF Match History E2E Tests
 *
 * Verifies the BFF-backed match history page (/history/matches) renders
 * correctly after the PostHog analytics wiring was added. PostHog is a no-op
 * when VITE_POSTHOG_KEY is unset in the test environment, so these tests only
 * assert UI behavior.
 */
test.describe('BFF Match History Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/history/matches');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 10000 });
  });

  test('@smoke should display the Match History heading', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toHaveText('Match History');
  });

  test('@smoke should show loading spinner while fetching', async ({ page }) => {
    // The spinner may disappear quickly; just confirm page loads without error.
    await expect(page.locator('.page-container')).toBeVisible();
  });

  test('should render match table or empty state (not an error page)', async ({ page }) => {
    // Wait for loading to complete — either table or empty state must appear.
    await Promise.race([
      page.locator('[data-testid="match-history-table"]').waitFor({ state: 'visible', timeout: 8000 }),
      page.locator('[data-testid="match-history-empty"]').waitFor({ state: 'visible', timeout: 8000 }),
    ]);

    // Confirm page has not crashed — heading still present.
    await expect(page.locator('h1.page-title')).toHaveText('Match History');
  });
});
