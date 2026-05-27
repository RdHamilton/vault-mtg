/**
 * Connected Devices E2E Tests (#2632)
 *
 * Covers the critical flow: navigate to Settings → expand Connected Devices →
 * list renders → revoke a device → row is removed.
 *
 * Auth: injects signed-in Clerk test state via window.__CLERK_TEST_STATE__ (same
 * pattern as settings.spec.ts). The VITE_CLERK_TEST_MODE=true build aliases
 * @clerk/react to the test mock that reads this state.
 *
 * BFF mocking: all BFF endpoints (GET /api/v1/daemons, DELETE /api/v1/daemons/:id,
 * GET /api/v1/settings) are mocked via page.route() so the test does not depend
 * on a live authenticated BFF.
 *
 * Out-of-scope (per Ray Q5 verdict): the 401-heartbeat assertion is NOT included.
 * That flow is delegated to Bob's BFF tests + Tim's staging-verify post-merge.
 */

import { test, expect, type Page } from '@playwright/test';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type ClerkTestState = {
  isSignedIn: boolean;
  firstName?: string;
  lastName?: string;
  email?: string;
};

async function setClerkSignedIn(page: Page, user?: Partial<ClerkTestState>): Promise<void> {
  const state: ClerkTestState = {
    isSignedIn: true,
    firstName: user?.firstName ?? 'Test',
    lastName: user?.lastName ?? 'User',
    email: user?.email ?? 'test@example.com',
  };
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, state);
}

/** Mock GET /api/v1/settings (required so Settings page renders). */
async function mockSettingsEndpoint(page: Page): Promise<void> {
  await page.route('**/api/v1/settings', (route) => {
    if (route.request().method() === 'GET') {
      void route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: {} }),
      });
      return;
    }
    void route.continue();
  });
}

type DeviceFixture = {
  device_id: string;
  platform: string;
  daemon_ver: string;
  paired_at: string;
  last_used_at: string | null;
};

/** Mock GET /api/v1/daemons with the given device list. */
async function mockDaemonsList(page: Page, devices: DeviceFixture[]): Promise<void> {
  await page.route('**/api/v1/daemons', (route) => {
    if (route.request().method() === 'GET') {
      void route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ devices }),
      });
      return;
    }
    void route.continue();
  });
}

/** Mock DELETE /api/v1/daemons/:id to return 204. */
async function mockRevokeSuccess(page: Page): Promise<void> {
  await page.route('**/api/v1/daemons/**', (route) => {
    if (route.request().method() === 'DELETE') {
      void route.fulfill({ status: 204 });
      return;
    }
    void route.continue();
  });
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const DEVICE_A: DeviceFixture = {
  device_id: 'aaaaaaaa-1111-2222-3333-444444444444',
  platform: 'windows',
  daemon_ver: 'v0.3.3',
  paired_at: '2026-05-01T10:00:00Z',
  last_used_at: null,
};

const DEVICE_B: DeviceFixture = {
  device_id: 'bbbbbbbb-5555-6666-7777-888888888888',
  platform: 'darwin',
  daemon_ver: 'v0.3.2',
  paired_at: '2026-04-15T14:30:00Z',
  last_used_at: null,
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Connected Devices — Settings page', () => {
  test.beforeEach(async ({ page }) => {
    await setClerkSignedIn(page);
    await mockSettingsEndpoint(page);
  });

  test('@smoke navigate to Settings and expand Connected Devices accordion', async ({ page }) => {
    await mockDaemonsList(page, [DEVICE_A, DEVICE_B]);
    await mockRevokeSuccess(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('h1')).toContainText('Settings');

    // Expand the Connected Devices accordion section
    const connectedDevicesButton = page.locator('button').filter({ hasText: /connected devices/i });
    await expect(connectedDevicesButton).toBeVisible();
    await connectedDevicesButton.click();

    // Section content must be visible
    const section = page.locator('.settings-section').filter({ hasText: 'Connected Devices' });
    await expect(section).toBeVisible();
  });

  test('list renders — shows device rows for each paired device', async ({ page }) => {
    await mockDaemonsList(page, [DEVICE_A, DEVICE_B]);
    await mockRevokeSuccess(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    // Expand the accordion
    const connectedDevicesButton = page.locator('button').filter({ hasText: /connected devices/i });
    await connectedDevicesButton.click();

    // Wait for device rows to appear (BFF mock responds synchronously)
    await expect(page.locator('[data-testid="device-row"]').first()).toBeVisible({ timeout: 5_000 });

    // Both devices should be rendered
    const rows = page.locator('[data-testid="device-row"]');
    await expect(rows).toHaveCount(2);
  });

  test('device row shows truncated device_id and platform', async ({ page }) => {
    await mockDaemonsList(page, [DEVICE_A]);
    await mockRevokeSuccess(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    const connectedDevicesButton = page.locator('button').filter({ hasText: /connected devices/i });
    await connectedDevicesButton.click();

    await expect(page.locator('[data-testid="device-row"]')).toBeVisible({ timeout: 5_000 });

    // Truncated device_id (first 8 chars + ellipsis)
    await expect(page.locator('.device-id')).toHaveText('aaaaaaaa…');
    // Platform
    await expect(page.locator('.device-platform')).toHaveText('windows');
    // Full UUID must NOT appear
    await expect(page.locator('body')).not.toContainText(DEVICE_A.device_id);
  });

  test('empty state — shows "No devices connected." when no devices paired', async ({ page }) => {
    await mockDaemonsList(page, []);
    await mockRevokeSuccess(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    const connectedDevicesButton = page.locator('button').filter({ hasText: /connected devices/i });
    await connectedDevicesButton.click();

    await expect(page.locator('[data-testid="connected-devices-empty"]')).toBeVisible({ timeout: 5_000 });
    await expect(page.locator('[data-testid="connected-devices-empty"]')).toHaveText('No devices connected.');
  });

  test('@smoke revoke — clicking Revoke removes the device row', async ({ page }) => {
    await mockDaemonsList(page, [DEVICE_A, DEVICE_B]);
    await mockRevokeSuccess(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    // Expand Connected Devices
    const connectedDevicesButton = page.locator('button').filter({ hasText: /connected devices/i });
    await connectedDevicesButton.click();

    // Wait for 2 rows
    await expect(page.locator('[data-testid="device-row"]').first()).toBeVisible({ timeout: 5_000 });
    await expect(page.locator('[data-testid="device-row"]')).toHaveCount(2);

    // Click the first Revoke button
    const revokeButtons = page.locator('[data-testid="revoke-button"]');
    await revokeButtons.first().click();

    // Row count should drop to 1 after optimistic removal
    await expect(page.locator('[data-testid="device-row"]')).toHaveCount(1, { timeout: 5_000 });
  });

  test('revoke — DEVICE_A row is removed after revoke, DEVICE_B stays', async ({ page }) => {
    await mockDaemonsList(page, [DEVICE_A, DEVICE_B]);
    await mockRevokeSuccess(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    const connectedDevicesButton = page.locator('button').filter({ hasText: /connected devices/i });
    await connectedDevicesButton.click();

    await expect(page.locator('[data-testid="device-row"]').first()).toBeVisible({ timeout: 5_000 });
    await expect(page.locator('[data-testid="device-row"]')).toHaveCount(2);

    // Click Revoke for DEVICE_A (first row)
    await page.locator('[data-testid="revoke-button"]').first().click();

    // DEVICE_A row gone
    await expect(
      page.locator(`[data-testid="device-row-${DEVICE_A.device_id}"]`)
    ).not.toBeVisible({ timeout: 5_000 });

    // DEVICE_B row still present
    await expect(
      page.locator(`[data-testid="device-row-${DEVICE_B.device_id}"]`)
    ).toBeVisible();
  });
});
