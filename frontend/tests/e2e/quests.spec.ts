import { test, expect } from '@playwright/test';

/**
 * Quests Page E2E Tests
 *
 * Tests the Quests page functionality.
 * Uses REST API backend for testing.
 *
 * AC4 coverage (#2008): Quest page navigation must produce no ERR_CONNECTION_REFUSED
 * errors for /system/account — that endpoint is served by the BFF (port 8080),
 * not the daemon (port 9001). The fix routes getCurrentAccount() through the BFF
 * client so the page loads account data even when the daemon is offline.
 */
test.describe('Quests', () => {
  test.beforeEach(async ({ page }) => {
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
