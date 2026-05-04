import { test, expect } from '@playwright/test';

/**
 * E2E tests for the X-Cache-Degraded degraded-mode notice in the Draft UI.
 *
 * The BFF returns X-Cache-Degraded: true when serving stale/cached ratings.
 * These tests mock the card ratings endpoint to return that header and verify
 * the notice is shown to the user.
 */
test.describe('Cache Degraded Notice', () => {
  test('shows degraded-mode notice when X-Cache-Degraded header is true', async ({ page }) => {
    // Intercept the card ratings endpoint and inject the X-Cache-Degraded header.
    await page.route('**/api/v1/cards/ratings/**', async (route) => {
      const response = await route.fetch();
      await route.fulfill({
        status: response.status(),
        headers: {
          ...Object.fromEntries(response.headers().entries()),
          'x-cache-degraded': 'true',
        },
        body: await response.body(),
      });
    });

    // Navigate to the draft page
    await page.goto('/draft');

    // Wait for the tier list to render (ratings loaded)
    await page.waitForSelector('.tier-list-container, .tier-list-empty, .tier-list-error', {
      timeout: 15000,
    });

    // The degraded notice should now be visible
    const notice = page.getByTestId('cache-degraded-notice');
    await expect(notice).toBeVisible({ timeout: 5000 });
    await expect(notice).toContainText(/ratings data may be stale/i);
  });

  test('does not show degraded-mode notice when X-Cache-Degraded header is absent', async ({ page }) => {
    // Intercept the card ratings endpoint and ensure no degraded header
    await page.route('**/api/v1/cards/ratings/**', async (route) => {
      const response = await route.fetch();
      const headers = Object.fromEntries(response.headers().entries());
      // Explicitly remove x-cache-degraded if the server happened to set it
      delete headers['x-cache-degraded'];
      await route.fulfill({
        status: response.status(),
        headers,
        body: await response.body(),
      });
    });

    await page.goto('/draft');

    await page.waitForSelector('.tier-list-container, .tier-list-empty, .tier-list-error', {
      timeout: 15000,
    });

    const notice = page.getByTestId('cache-degraded-notice');
    await expect(notice).not.toBeVisible();
  });

  test('notice can be dismissed by the user', async ({ page }) => {
    await page.route('**/api/v1/cards/ratings/**', async (route) => {
      const response = await route.fetch();
      await route.fulfill({
        status: response.status(),
        headers: {
          ...Object.fromEntries(response.headers().entries()),
          'x-cache-degraded': 'true',
        },
        body: await response.body(),
      });
    });

    await page.goto('/draft');

    await page.waitForSelector('.tier-list-container, .tier-list-empty, .tier-list-error', {
      timeout: 15000,
    });

    const notice = page.getByTestId('cache-degraded-notice');
    await expect(notice).toBeVisible({ timeout: 5000 });

    // Click the dismiss button
    await page.getByRole('button', { name: /dismiss stale data notice/i }).click();

    // Notice should disappear
    await expect(notice).not.toBeVisible();
  });
});
