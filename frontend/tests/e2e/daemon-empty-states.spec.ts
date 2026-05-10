import { test, expect } from '@playwright/test';

/**
 * Daemon Empty States E2E Tests (#1697, #1698, #1699)
 *
 * Verifies that Match History, Collection, and Decks pages each render a
 * first-run empty state (with a /setup CTA) when the daemon is not connected.
 *
 * The BFF /api/v1/health/daemon endpoint is mocked to return disconnected for
 * every test in this suite.
 */

test.describe('Daemon Empty States (no daemon connected)', () => {
  /**
   * Route the BFF daemon health endpoint to return disconnected for all tests.
   * Each test re-declares it so the mock is scoped per test.
   */
  async function mockDaemonDisconnected(page: import('@playwright/test').Page) {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });
  }

  test.describe('Match History — daemon not connected', () => {
    test('@smoke shows daemon empty state on Match History when daemon is disconnected', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // Navigate to Match History (default page)
      await expect(page.locator('h1.page-title')).toHaveText('Match History');

      // Should render the daemon empty state
      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });
    });

    test('daemon empty state on Match History has a /setup CTA link', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      const cta = page.locator('[data-testid="daemon-empty-state"] a.empty-state-cta');
      await expect(cta).toBeVisible();
      await expect(cta).toHaveAttribute('href', '/setup');
    });

    test('Match History table is NOT rendered when daemon is disconnected', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      await expect(page.locator('.match-history-table-container')).not.toBeVisible();
    });
  });

  test.describe('Collection — daemon not connected', () => {
    test('@smoke shows daemon empty state on Collection when daemon is disconnected', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // Navigate to Collection
      await page.click('a[href="/collection"]');
      await page.waitForURL('**/collection');

      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });
    });

    test('daemon empty state on Collection has a /setup CTA link', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/collection');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      const cta = page.locator('[data-testid="daemon-empty-state"] a.empty-state-cta');
      await expect(cta).toBeVisible();
      await expect(cta).toHaveAttribute('href', '/setup');
    });

    test('Collection card grid is NOT rendered when daemon is disconnected', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/collection');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      await expect(page.locator('[data-testid="collection-card-grid"]')).not.toBeVisible();
    });
  });

  test.describe('Decks — daemon not connected', () => {
    test('@smoke shows daemon empty state on Decks when daemon is disconnected', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // Navigate to Decks
      await page.click('a[href="/decks"]');
      await page.waitForURL('**/decks');

      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });
    });

    test('daemon empty state on Decks has a /setup CTA link', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/decks');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      const cta = page.locator('[data-testid="daemon-empty-state"] a.empty-state-cta');
      await expect(cta).toBeVisible();
      await expect(cta).toHaveAttribute('href', '/setup');
    });

    test('Decks grid is NOT rendered when daemon is disconnected', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/decks');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      await expect(page.locator('.decks-grid')).not.toBeVisible();
    });
  });

  test.describe('End-to-end: no-daemon empty state flow across pages', () => {
    test('@smoke navigating Match History, Collection, Decks all show daemon empty state when daemon is disconnected', async ({ page }) => {
      await mockDaemonDisconnected(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // Match History (default)
      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      // Navigate to Collection
      await page.click('a[href="/collection"]');
      await page.waitForURL('**/collection');
      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });

      // Navigate to Decks
      await page.click('a[href="/decks"]');
      await page.waitForURL('**/decks');
      await expect(page.locator('[data-testid="daemon-empty-state"]')).toBeVisible({ timeout: 10000 });
    });
  });
});
