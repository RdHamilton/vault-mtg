import { test, expect, type Page } from '@playwright/test';

/**
 * Daemon Uninstall E2E (#1890 / Phase 2 PR #18)
 *
 * Covers the full Settings → Data Recovery → "Danger Zone — Uninstall
 * Daemon" flow:
 *   - Button visibility + disabled state when daemon is disconnected
 *   - Two-step confirmation panel
 *   - Cancel returns to initial state
 *   - Confirm with purge=false renders the backend's residual-action
 *     message verbatim
 *   - Confirm with purge=true sends the query param + renders the
 *     purge-variant message
 *   - Failed POST renders the error text from the response payload
 *
 * The daemon's local API (port 9001) is mocked via page.route so the
 * tests don't depend on a running daemon. Selectors target the exact
 * DataRecoverySection labels:
 *   - "Uninstall VaultMTG Daemon" — the entry-point button
 *   - "Confirm Uninstall" — the destructive action
 *   - "Also wipe my local config" — the purge checkbox
 */

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

// Mock the daemon's /api/v1/system/status to look "connected" so the
// uninstall button is enabled. Tests that want it disabled call
// mockDaemonDisconnected instead.
async function mockDaemonConnected(page: Page): Promise<void> {
  await page.route('**/localhost:9001/api/v1/system/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          status: 'connected',
          connected: true,
          mode: 'live',
          url: 'http://localhost:8080',
          port: 9001,
        },
      }),
    });
  });
}

async function mockDaemonDisconnected(page: Page): Promise<void> {
  // Disconnected state: either the daemon returns degraded status OR
  // the request fails outright. Both result in isConnected=false on
  // the SPA side. We use the failure path because it matches a daemon-
  // not-running scenario most realistically.
  await page.route('**/localhost:9001/api/v1/system/status', async (route) => {
    await route.abort('failed');
  });
}

// Mock the uninstall endpoint with a chosen response shape. Lets each
// test configure success vs failure independently.
async function mockUninstall(
  page: Page,
  opts: {
    status: number;
    body: { status: string; message: string };
    onMatch?: (url: URL) => void;
  },
): Promise<void> {
  await page.route('**/localhost:9001/api/v1/system/uninstall*', async (route) => {
    if (opts.onMatch) {
      opts.onMatch(new URL(route.request().url()));
    }
    await route.fulfill({
      status: opts.status,
      contentType: 'application/json',
      body: JSON.stringify({ data: opts.body }),
    });
  });
}

async function openDataRecovery(page: Page): Promise<void> {
  await page.goto('/settings');
  await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  await page.waitForURL('**/settings');
  const header = page.locator('button').filter({ hasText: /data recovery/i });
  await expect(header).toBeVisible();
  await header.click();
}

test.describe('Daemon Uninstall — DataRecoverySection', () => {
  test.beforeEach(async ({ page }) => {
    await setClerkSignedIn(page);
  });

  test('disables the uninstall button when the daemon is disconnected', async ({ page }) => {
    await mockDaemonDisconnected(page);
    await openDataRecovery(page);

    const uninstallBtn = page.getByRole('button', { name: /Uninstall VaultMTG Daemon/i });
    await expect(uninstallBtn).toBeVisible();
    await expect(uninstallBtn).toBeDisabled();
    await expect(page.getByText(/Daemon must be running to trigger uninstall/i)).toBeVisible();
  });

  test('cancel returns to the initial state', async ({ page }) => {
    await mockDaemonConnected(page);
    await openDataRecovery(page);

    const uninstallBtn = page.getByRole('button', { name: /Uninstall VaultMTG Daemon/i });
    await expect(uninstallBtn).toBeEnabled();
    await uninstallBtn.click();

    await expect(page.getByRole('button', { name: /Confirm Uninstall/i })).toBeVisible();
    await expect(page.getByRole('checkbox', { name: /Also wipe my local config/i })).toBeVisible();

    await page.getByRole('button', { name: /Cancel/i }).click();
    await expect(page.getByRole('button', { name: /Uninstall VaultMTG Daemon/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Confirm Uninstall/i })).toHaveCount(0);
  });

  test('confirm uninstall (purge=false) renders the backend message', async ({ page }) => {
    await mockDaemonConnected(page);
    let capturedURL: URL | null = null;
    await mockUninstall(page, {
      status: 200,
      body: {
        status: 'scheduled',
        message:
          'Daemon stopped and removed from launchd. Drag VaultMTG to the Trash to remove the app bundle.',
      },
      onMatch: (url) => {
        capturedURL = url;
      },
    });
    await openDataRecovery(page);

    await page.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }).click();
    await page.getByRole('button', { name: /Confirm Uninstall/i }).click();

    await expect(
      page.getByText(/Drag VaultMTG to the Trash to remove the app bundle/i),
    ).toBeVisible();

    // purge=false → no query string on the POST.
    expect(capturedURL).not.toBeNull();
    expect(capturedURL!.searchParams.get('purge')).toBeNull();
  });

  test('confirm uninstall (purge=true) sends purge=true and renders the purge-variant message', async ({
    page,
  }) => {
    await mockDaemonConnected(page);
    let capturedURL: URL | null = null;
    await mockUninstall(page, {
      status: 200,
      body: {
        status: 'scheduled',
        message:
          'Daemon stopped, removed from launchd, and config wiped. Drag VaultMTG to the Trash to remove the app bundle.',
      },
      onMatch: (url) => {
        capturedURL = url;
      },
    });
    await openDataRecovery(page);

    await page.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }).click();
    await page.getByRole('checkbox', { name: /Also wipe my local config/i }).check();
    await page.getByRole('button', { name: /Confirm Uninstall/i }).click();

    await expect(page.getByText(/config wiped/i)).toBeVisible();

    expect(capturedURL).not.toBeNull();
    expect(capturedURL!.searchParams.get('purge')).toBe('true');
  });

  test('renders the backend error message when the uninstall request fails', async ({ page }) => {
    await mockDaemonConnected(page);
    await mockUninstall(page, {
      status: 500,
      body: { status: 'error', message: 'boom' },
    });
    await openDataRecovery(page);

    await page.getByRole('button', { name: /Uninstall VaultMTG Daemon/i }).click();
    await page.getByRole('button', { name: /Confirm Uninstall/i }).click();

    // The daemonClient surfaces the response payload's "message" as
    // the thrown error's .message, which DataRecoverySection renders
    // in the error panel.
    await expect(page.getByText(/boom/i)).toBeVisible();
  });
});
