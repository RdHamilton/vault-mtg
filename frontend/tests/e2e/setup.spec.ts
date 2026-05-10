/**
 * Setup Page E2E Tests
 *
 * Covers:
 * - #1644: Gatekeeper + SmartScreen install warning sections
 * - #1645: PKCE pairing status (waiting → success → redirect)
 * - #1646: Download page link
 *
 * The daemon health endpoint is mocked at the network level so no real daemon
 * is required.
 */

import { test, expect } from '@playwright/test';

test.describe('Setup Page — structure', () => {
  test.beforeEach(async ({ page }) => {
    // Default: daemon health unreachable (keeps pairing in "waiting" state)
    await page.route('http://localhost:9001/health', async (route) => {
      await route.abort('connectionrefused');
    });
    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('@smoke renders the setup heading', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Install the VaultMTG Daemon');
  });

  test('@smoke renders the setup container', async ({ page }) => {
    await expect(page.locator('[data-testid="setup-container"]')).toBeVisible();
  });

  test('renders Step 1 download section', async ({ page }) => {
    await expect(page.locator('[data-testid="setup-download-section"]')).toBeVisible();
  });

  test('renders Step 2 warnings section', async ({ page }) => {
    await expect(page.locator('[data-testid="setup-warnings-section"]')).toBeVisible();
  });

  test('renders Step 3 pairing section', async ({ page }) => {
    await expect(page.locator('[data-testid="setup-pairing-section"]')).toBeVisible();
  });

  test('download page link points to /download', async ({ page }) => {
    const link = page.locator('[data-testid="download-page-link"]');
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute('href', '/download');
  });
});

test.describe('Setup Page — install warnings', () => {
  test('@smoke both Gatekeeper and SmartScreen sections are accessible', async ({ page }) => {
    await page.route('http://localhost:9001/health', async (route) => {
      await route.abort('connectionrefused');
    });
    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Both warning testids must exist somewhere on the page (one may be inside <details>)
    const gatekeeperCount = await page
      .locator('[data-testid="gatekeeper-warning"]')
      .count();
    const smartscreenCount = await page
      .locator('[data-testid="smartscreen-warning"]')
      .count();

    expect(gatekeeperCount).toBeGreaterThan(0);
    expect(smartscreenCount).toBeGreaterThan(0);
  });

  test('Gatekeeper section contains bypass instructions', async ({ page }) => {
    await page.route('http://localhost:9001/health', async (route) => {
      await route.abort('connectionrefused');
    });
    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Open any collapsed details that may contain the warning
    const detailsElements = page.locator('details');
    const count = await detailsElements.count();
    for (let i = 0; i < count; i++) {
      await detailsElements.nth(i).evaluate((el: HTMLDetailsElement) => {
        el.open = true;
      });
    }

    // Find gatekeeper warning text anywhere on page
    const gatekeeperText = await page
      .locator('[data-testid="gatekeeper-warning"]')
      .first()
      .textContent();
    expect(gatekeeperText).toMatch(/open anyway/i);
    expect(gatekeeperText).toMatch(/gatekeeper/i);
  });

  test('SmartScreen section contains bypass instructions', async ({ page }) => {
    await page.route('http://localhost:9001/health', async (route) => {
      await route.abort('connectionrefused');
    });
    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Open any collapsed details
    const detailsElements = page.locator('details');
    const count = await detailsElements.count();
    for (let i = 0; i < count; i++) {
      await detailsElements.nth(i).evaluate((el: HTMLDetailsElement) => {
        el.open = true;
      });
    }

    const smartscreenText = await page
      .locator('[data-testid="smartscreen-warning"]')
      .first()
      .textContent();
    expect(smartscreenText).toMatch(/run anyway/i);
    expect(smartscreenText).toMatch(/more info/i);
  });

  test('copy is empathetic — explains unsigned beta is normal', async ({ page }) => {
    await page.route('http://localhost:9001/health', async (route) => {
      await route.abort('connectionrefused');
    });
    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
    const body = await page.locator('body').textContent();
    expect(body).toMatch(/indie beta software/i);
  });
});

test.describe('Setup Page — PKCE pairing flow', () => {
  test('@smoke shows "Waiting for auth" in initial state', async ({ page }) => {
    await page.route('http://localhost:9001/health', async (route) => {
      await route.abort('connectionrefused');
    });
    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
    await expect(page.locator('[data-testid="pairing-waiting"]')).toBeVisible();
    await expect(page.locator('[data-testid="pairing-waiting"]')).toContainText(
      /waiting for auth/i
    );
  });

  test('transitions to "Auth complete" when daemon returns configured: true', async ({
    page,
  }) => {
    // Daemon returns configured: true immediately
    await page.route('http://localhost:9001/health', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ configured: true }),
      });
    });

    await page.goto('/setup');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Wait for success state (polling interval is 3s in production; tests may see
    // it quickly since the mock resolves immediately on every poll)
    await expect(page.locator('[data-testid="pairing-success"]')).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.locator('[data-testid="pairing-success"]')).toContainText(
      /auth complete/i
    );
  });
});

test.describe('Setup Page — navigation', () => {
  test('setup page is accessible at /setup (HTTP 200)', async ({ page }) => {
    await page.route('http://localhost:9001/health', async (route) => {
      await route.abort('connectionrefused');
    });
    const response = await page.goto('/setup');
    expect(response?.status()).toBe(200);
  });
});
