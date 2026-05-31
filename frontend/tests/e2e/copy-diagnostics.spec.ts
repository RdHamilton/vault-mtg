/**
 * Copy Diagnostics E2E Tests (vault-mtg-tickets#90)
 *
 * Covers the success path: open Settings → expand Copy Diagnostics accordion
 * → click "Copy Diagnostics" → assert clipboard contents include daemon version.
 *
 * Auth: injects signed-in Clerk test state via window.__CLERK_TEST_STATE__
 * (same pattern as connected-devices.spec.ts).
 *
 * The local daemon endpoint (127.0.0.1:9001) is mocked via page.route() so
 * the test does not depend on a live daemon process.
 *
 * Clipboard: Playwright exposes navigator.clipboard in the test context.
 * We grant 'clipboard-read' permission and read the value after the click.
 *
 * AC6: success path — click Copy Diagnostics → clipboard includes daemon_version.
 */

import { test, expect, type Page, type BrowserContext } from '@playwright/test';

// ---------------------------------------------------------------------------
// Fixtures / helpers
// ---------------------------------------------------------------------------

type ClerkTestState = {
  isSignedIn: boolean;
  firstName?: string;
  lastName?: string;
};

async function setClerkSignedIn(page: Page, user?: Partial<ClerkTestState>): Promise<void> {
  const state: ClerkTestState = {
    isSignedIn: true,
    firstName: user?.firstName ?? 'Test',
    lastName: user?.lastName ?? 'User',
  };
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, state);
}

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

const mockDiagnosticsPayload = {
  daemon_version: '0.3.6-e2e',
  os: 'darwin',
  arch: 'arm64',
  uptime_seconds: 1800,
  started_at: '2026-05-31T10:00:00Z',
  cloud_api_url: 'https://api.vaultmtg.app',
  session_id: 'sess_e2e_test',
  log_path: '/Users/test/Library/Logs/vaultmtg-daemon.log',
  log_tail: ['2026-05-31T10:00:00Z INFO daemon started'],
};

async function mockDiagnosticsEndpoint(page: Page): Promise<void> {
  await page.route('http://127.0.0.1:9001/api/v1/system/diagnostics', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockDiagnosticsPayload),
    });
  });
}

async function grantClipboardPermissions(context: BrowserContext): Promise<void> {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Feature: Copy Diagnostics panel', () => {
  test(
    'AC6 — success path: open Settings, click Copy Diagnostics, clipboard contains daemon_version @smoke',
    async ({ page, context }) => {
      await grantClipboardPermissions(context);
      await setClerkSignedIn(page);
      await mockSettingsEndpoint(page);
      await mockDiagnosticsEndpoint(page);

      await page.goto('/settings');

      // Wait for the Settings page to render
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // Find and expand the Copy Diagnostics accordion item.
      // The SettingsAccordion renders items as buttons/panels — look for the
      // accordion trigger containing "Copy Diagnostics".
      const accordionTrigger = page.getByRole('button', { name: /copy diagnostics/i }).first();
      await accordionTrigger.waitFor({ state: 'visible' });
      await accordionTrigger.click();

      // The section panel should now be visible.
      await expect(page.getByTestId('copy-diagnostics-section')).toBeVisible();

      // Click the Copy Diagnostics button inside the section.
      const copyBtn = page.getByTestId('copy-diagnostics-section').getByRole('button', {
        name: /copy diagnostics/i,
      });
      await copyBtn.click();

      // Wait for the async fetch + clipboard write to complete.
      // The button should return to its non-loading label.
      await expect(copyBtn).toHaveText(/copy diagnostics/i, { timeout: 5000 });

      // No error banner should be visible.
      await expect(page.getByTestId('copy-diagnostics-error')).not.toBeVisible();

      // Read clipboard and assert it includes the daemon version from the mock.
      const clipboardText: string = await page.evaluate(() =>
        navigator.clipboard.readText(),
      );
      expect(clipboardText).toContain('0.3.6-e2e');
    },
  );

  test(
    'daemon-down error path: Copy Diagnostics shows error banner when daemon is offline',
    async ({ page }) => {
      await setClerkSignedIn(page);
      await mockSettingsEndpoint(page);

      // Do NOT mock the daemon endpoint — the fetch will fail (net::ERR_CONNECTION_REFUSED).
      // But since Playwright intercepts all routes to localhost in test mode, we mock
      // it to return a connection-refused style failure instead.
      await page.route('http://127.0.0.1:9001/**', (route) => {
        void route.abort('connectionrefused');
      });

      await page.goto('/settings');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      const accordionTrigger = page.getByRole('button', { name: /copy diagnostics/i }).first();
      await accordionTrigger.waitFor({ state: 'visible' });
      await accordionTrigger.click();

      await expect(page.getByTestId('copy-diagnostics-section')).toBeVisible();

      const copyBtn = page.getByTestId('copy-diagnostics-section').getByRole('button', {
        name: /copy diagnostics/i,
      });
      await copyBtn.click();

      // Error banner should appear.
      await expect(page.getByTestId('copy-diagnostics-error')).toBeVisible({ timeout: 5000 });
    },
  );
});
