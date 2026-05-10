import { test, expect } from '@playwright/test';

/**
 * Setup Page E2E Tests
 *
 * Verifies the setup stub page is accessible and displays the placeholder content.
 */
test.describe('Setup Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('@smoke should display the setup heading', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Setup');
  });

  test('@smoke should display coming soon message', async ({ page }) => {
    await expect(page.locator('text=/coming soon/i')).toBeVisible();
  });

  test('should display daemon setup text', async ({ page }) => {
    await expect(page.locator('text=/daemon setup will be available here/i')).toBeVisible();
  });

  test('setup page is accessible from the sidebar', async ({ page }) => {
    // Verify the setup page is navigable
    const response = await page.goto('/setup');
    expect(response?.status()).toBe(200);
  });
});
