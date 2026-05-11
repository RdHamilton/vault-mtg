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
 * Download Page E2E Tests
 *
 * Verifies the daemon download section is visible, download links have correct
 * hrefs pointing to GitHub Releases, and the getting-started steps are displayed.
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
      'Get Started with MTGA Companion'
    );
  });

  test('should show the Download nav tab', async ({ page }) => {
    await expect(page.locator('[data-testid="nav-tab-download"]')).toBeVisible();
  });

  test.describe('Download Links', () => {
    test('@smoke Windows download link has correct href', async ({ page }) => {
      const link = page.locator('[data-testid="download-link-windows-amd64"]');
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/mtga-companion-daemon-windows-amd64.exe`
      );
    });

    test('@smoke macOS Apple Silicon download link has correct href', async ({ page }) => {
      const link = page.locator('[data-testid="download-link-darwin-arm64"]');
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/mtga-companion-daemon-darwin-arm64.dmg`
      );
    });

    test('@smoke macOS Intel download link has correct href', async ({ page }) => {
      const link = page.locator('[data-testid="download-link-darwin-amd64"]');
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/mtga-companion-daemon-darwin-amd64.dmg`
      );
    });

    test('exactly 3 download links are rendered', async ({ page }) => {
      const buttons = page.locator('[data-testid="daemon-download-buttons"] a');
      await expect(buttons).toHaveCount(3);
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
      await expect(page.getByText('macOS 12+ on M1/M2/M3')).toBeVisible();
      await expect(page.getByText('macOS 12+ on Intel')).toBeVisible();
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
    // Start from match history
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Click the Download tab
    await page.locator('[data-testid="nav-tab-download"]').click();

    // Verify download section is visible
    await expect(page.locator('[data-testid="daemon-download-section"]')).toBeVisible();
  });
});

test.describe('Download Page — feature flag OFF (coming soon)', () => {
  test.beforeEach(async ({ page }) => {
    await mockPostHogFlag(page, 'daemon_download_enabled', false);
    await page.goto('/download');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('@smoke should show coming-soon CTA when daemon_download_enabled flag is off', async ({ page }) => {
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
