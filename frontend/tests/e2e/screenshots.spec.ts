import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

/**
 * Documentation Screenshots
 *
 * This test file generates screenshots for documentation purposes.
 * Screenshots are saved to docs/images/ for use in README and documentation.
 *
 * Run with: npm run screenshots
 *
 * Note: Screenshots are generated using test fixtures for realistic data.
 * Dark theme is used by default to match the application's default appearance.
 */

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const SCREENSHOT_DIR = path.join(__dirname, '../../../../docs/images');

// Ensure the screenshots directory exists
test.beforeAll(async () => {
  if (!fs.existsSync(SCREENSHOT_DIR)) {
    fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
  }
});

test.describe('Documentation Screenshots', () => {
  test.beforeEach(async ({ page }) => {
    // Set viewport for consistent screenshots
    await page.setViewportSize({ width: 1280, height: 800 });
  });

  test('capture Match History page', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for primary content to be visible
    await page.waitForSelector('.match-history-table-container, .empty-state', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'match-history.png'),
      fullPage: false,
    });
  });

  test('capture Draft History page', async ({ page }) => {
    await page.goto('/draft');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for draft content to be visible
    await page.waitForSelector('.draft-container, .draft-empty, .empty-state', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'draft-history.png'),
      fullPage: false,
    });
  });

  test('capture Decks page', async ({ page }) => {
    await page.goto('/decks');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for decks page content to be visible
    await page.waitForSelector('.decks-page', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'decks.png'),
      fullPage: false,
    });
  });

  test('capture Quests page', async ({ page }) => {
    await page.goto('/quests');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for quests content to be visible
    await page.waitForSelector('.quests-section, .quests-header, .empty-state', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'quests.png'),
      fullPage: false,
    });
  });

  test('capture Collection page', async ({ page }) => {
    await page.goto('/collection');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for collection content to be visible
    await page.waitForSelector('.collection-container, .collection-page, .empty-state', { timeout: 15000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'collection.png'),
      fullPage: false,
    });
  });

  test('capture Meta Dashboard', async ({ page }) => {
    await page.goto('/meta');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for meta page content to be visible
    await page.waitForSelector('.meta-page', { timeout: 15000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'meta-dashboard.png'),
      fullPage: false,
    });
  });

  test('capture Charts - Deck Performance', async ({ page }) => {
    await page.goto('/charts/deck-performance');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for chart page container to be visible
    await page.waitForSelector('.page-container', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'charts-deck-performance.png'),
      fullPage: false,
    });
  });

  test('capture Charts - Format Distribution', async ({ page }) => {
    await page.goto('/charts/format-distribution');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for chart page container to be visible
    await page.waitForSelector('.page-container', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'charts-format-distribution.png'),
      fullPage: false,
    });
  });

  test('capture Charts - Result Breakdown', async ({ page }) => {
    await page.goto('/charts/result-breakdown');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for chart page container to be visible
    await page.waitForSelector('.page-container', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'charts-result-breakdown.png'),
      fullPage: false,
    });
  });

  test('capture Settings page', async ({ page }) => {
    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for settings page content to be visible
    await page.waitForSelector('.settings-page, .settings-container, [class*="settings"]', { timeout: 10000 }).catch(() => {});

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'settings.png'),
      fullPage: false,
    });
  });
});
