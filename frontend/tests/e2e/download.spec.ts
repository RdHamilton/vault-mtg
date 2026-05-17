import { test, expect } from '@playwright/test';

const RELEASES_BASE =
  'https://github.com/RdHamilton/MTGA-Companion/releases/latest/download';

/**
 * Intercept the PostHog /decide endpoint to set feature flag values.
 * PostHog calls /decide?v=3 (or similar) to fetch flag payloads.
 */
async function mockPostHogFlag(
  page: import('@playwright/test').Page,
  flagKey: string,
  enabled: boolean
) {
  await page.route('**/decide**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        featureFlags: {
          [flagKey]: enabled,
        },
        featureFlagPayloads: {},
      }),
    });
  });
}

/**
 * Download Page E2E Tests (#2178)
 *
 * Verifies the daemon download section is visible, download links have correct
 * hrefs pointing to GitHub Releases, and the getting-started steps are displayed.
 *
 * Updated for the VaultMTG rebrand (#2178): the daemon download UI now reads
 * "Get Started with VaultMTG", ships a single macOS Universal binary
 * (vaultmtg-daemon-darwin-universal) plus a Windows binary
 * (vaultmtg-daemon-windows-amd64) — the legacy "MTGA Companion" copy and the
 * separate Apple-Silicon / Intel artifacts no longer exist. Assertions track
 * src/components/DaemonDownload.tsx.
 */
test.describe('Download Page', () => {
  test.beforeEach(async ({ page }) => {
    await mockPostHogFlag(page, 'daemon_download_enabled', true);
    await page.goto('/download');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('@smoke should display the daemon download section', async ({ page }) => {
    await expect(page.locator('[data-testid="daemon-download-section"]')).toBeVisible();
  });

  test('@smoke should display the page title', async ({ page }) => {
    await expect(page.locator('[data-testid="daemon-download-title"]')).toHaveText(
      'Get Started with VaultMTG'
    );
  });

  test('should show the Download nav tab', async ({ page }) => {
    await expect(page.locator('[data-testid="nav-tab-download"]')).toBeVisible();
  });

  test.describe('Download Links', () => {
    test('@smoke Windows download link has correct href', async ({ page }) => {
      const link = page.locator('[data-testid="download-link-vaultmtg-daemon-windows-amd64"]');
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/vaultmtg-daemon-windows-amd64.exe`
      );
    });

    test('@smoke macOS Universal download link has correct href', async ({ page }) => {
      const link = page.locator('[data-testid="download-link-vaultmtg-daemon-darwin-universal"]');
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/vaultmtg-daemon-darwin-universal.dmg`
      );
    });

    test('exactly 2 download links are rendered', async ({ page }) => {
      const buttons = page.locator('[data-testid="daemon-download-buttons"] a');
      await expect(buttons).toHaveCount(2);
    });

    test('each download link has the download attribute', async ({ page }) => {
      const buttons = page.locator('[data-testid="daemon-download-buttons"] a');
      const count = await buttons.count();
      for (let i = 0; i < count; i++) {
        await expect(buttons.nth(i)).toHaveAttribute('download', '');
      }
    });

    test('platform descriptions are visible', async ({ page }) => {
      await expect(page.getByText('Windows 10/11 64-bit')).toBeVisible();
      await expect(page.getByText('macOS 12+ — Apple Silicon and Intel')).toBeVisible();
    });
  });

  test.describe('Getting Started Steps', () => {
    test('@smoke should display all 4 getting started steps', async ({ page }) => {
      await expect(page.locator('[data-testid="daemon-getting-started"]')).toBeVisible();
      for (let i = 1; i <= 4; i++) {
        await expect(
          page.locator(`[data-testid="getting-started-step-${i}"]`)
        ).toBeVisible();
      }
    });

    test('steps contain correct titles', async ({ page }) => {
      await expect(page.locator('[data-testid="getting-started-step-1"]')).toContainText('Download');
      await expect(page.locator('[data-testid="getting-started-step-2"]')).toContainText('Run the installer');
      await expect(page.locator('[data-testid="getting-started-step-3"]')).toContainText('Launch MTGA Arena');
      await expect(page.locator('[data-testid="getting-started-step-4"]')).toContainText('Open the companion app');
    });
  });

  test('navigating from nav tab reaches download page', async ({ page }) => {
    // Start from the default landing page.
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Click the Download tab
    await page.locator('[data-testid="nav-tab-download"]').click();

    // Verify download section is visible
    await expect(page.locator('[data-testid="daemon-download-section"]')).toBeVisible();
  });
});

/**
 * Feature-flag-OFF coverage.
 *
 * NOT @smoke-tagged (#2178): the coming-soon CTA only renders when the
 * `daemon_download_enabled` PostHog flag resolves to false. useFeatureFlag
 * (src/hooks/useFeatureFlag.ts) defaults to `true` whenever PostHog is not
 * initialized — and the CI smoke harness does not set VITE_POSTHOG_KEY, so
 * posthog.init() never runs, PostHog never requests /decide, and the
 * mockPostHogFlag() route is never hit. The flag is therefore always ON in the
 * smoke project, so the flag-OFF assertions cannot pass there. These tests stay
 * in the `full` project, to be run against an environment with PostHog
 * configured. The flag-ON @smoke tests above match the CI default and remain.
 */
test.describe('Download Page — feature flag OFF (coming soon)', () => {
  test.beforeEach(async ({ page }) => {
    await mockPostHogFlag(page, 'daemon_download_enabled', false);
    await page.goto('/download');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('should show coming-soon CTA when daemon_download_enabled flag is off', async ({ page }) => {
    await expect(page.locator('[data-testid="daemon-download-coming-soon"]')).toBeVisible();
  });

  test('should display beta launch message in CTA', async ({ page }) => {
    await expect(
      page.getByText(/The daemon installer will be available at beta launch/i)
    ).toBeVisible();
  });

  test('should display the waitlist link', async ({ page }) => {
    const link = page.locator('[data-testid="daemon-download-waitlist-link"]');
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute('href', 'https://vaultmtg.app/#waitlist');
  });

  test('should NOT show the download buttons when flag is off', async ({ page }) => {
    await expect(page.locator('[data-testid="daemon-download-buttons"]')).not.toBeVisible();
  });

  test('should still show the download section header and getting-started steps', async ({ page }) => {
    await expect(page.locator('[data-testid="daemon-download-title"]')).toBeVisible();
    await expect(page.locator('[data-testid="daemon-getting-started"]')).toBeVisible();
  });
});
