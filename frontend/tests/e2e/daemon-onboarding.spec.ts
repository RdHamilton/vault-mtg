import { test, expect } from '@playwright/test';

/**
 * Daemon Onboarding Flow E2E tests (#1398)
 *
 * Verifies that the OnboardingModal appears for a new user whose daemon
 * is not connected, and that the 3-step flow works correctly.
 *
 * The BFF's /api/v1/health/daemon endpoint must return disconnected for
 * these tests. In the test environment, the BFF starts in daemon=false
 * mode, so the daemon health endpoint returns disconnected by default.
 *
 * Note: Onboarding modal visibility is gated on:
 * 1. User is signed in (Clerk test mode provides mock auth)
 * 2. Daemon is disconnected (BFF health check returns disconnected)
 * 3. User has not previously dismissed/completed onboarding (localStorage is clean)
 */

test.describe('Daemon Onboarding Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Clear localStorage so onboarding state is fresh for each test
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    // Wait for the app to load (timeout governed by global expect.timeout: 30_000)
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('@smoke onboarding modal appears for new user with no daemon', async ({ page }) => {
    // Mock the daemon health endpoint to return disconnected
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    // Navigate and clear localStorage
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Onboarding modal should appear once the daemon health check returns disconnected
    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();
  });

  test('@smoke step 1 shows download link to vaultmtg.app/download', async ({ page }) => {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    const downloadLink = page.locator('[data-testid="onboarding-download-link"]');
    await expect(downloadLink).toBeVisible();
    await expect(downloadLink).toHaveAttribute('href', 'https://vaultmtg.app/download');
  });

  test('step 1 to step 2 navigation works', async ({ page }) => {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    await page.locator('[data-testid="onboarding-step-1-next"]').click();
    await expect(page.locator('[data-testid="onboarding-step-2"]')).toBeVisible();
  });

  test('step 2 shows Mac and Windows install instructions', async ({ page }) => {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    await page.locator('[data-testid="onboarding-step-1-next"]').click();
    await expect(page.locator('[data-testid="onboarding-platform-mac"]')).toBeVisible();
    await expect(page.locator('[data-testid="onboarding-platform-windows"]')).toBeVisible();
  });

  test('step 2 to step 3 navigation works', async ({ page }) => {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    await page.locator('[data-testid="onboarding-step-1-next"]').click();
    await page.locator('[data-testid="onboarding-step-2-next"]').click();
    await expect(page.locator('[data-testid="onboarding-step-3"]')).toBeVisible();
  });

  test('step 3 shows spinner while waiting for daemon', async ({ page }) => {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    await page.locator('[data-testid="onboarding-step-1-next"]').click();
    await page.locator('[data-testid="onboarding-step-2-next"]').click();
    await expect(page.locator('[data-testid="onboarding-spinner"]')).toBeVisible();
  });

  test('step 3 shows success state when daemon connects', async ({ page }) => {
    // First return disconnected to trigger modal, then return connected
    let callCount = 0;
    await page.route('**/api/v1/health/daemon', async (route) => {
      callCount++;
      if (callCount <= 1) {
        // Initial nav check — disconnected so modal appears
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ status: 'disconnected' }),
        });
      } else {
        // Step 3 poll — daemon connected
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ status: 'connected' }),
        });
      }
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    await page.locator('[data-testid="onboarding-step-1-next"]').click();
    await page.locator('[data-testid="onboarding-step-2-next"]').click();

    // Wait for the step 3 poll to succeed and show the success state
    await expect(page.locator('[data-testid="onboarding-success-heading"]')).toBeVisible();
  });

  test('dismiss button closes modal and does not re-show', async ({ page }) => {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    await page.locator('[data-testid="onboarding-modal-close"]').click();
    await expect(page.locator('[data-testid="onboarding-modal"]')).not.toBeVisible();

    // Verify localStorage was updated
    const dismissed = await page.evaluate(() =>
      localStorage.getItem('vaultmtg_onboarding_dismissed')
    );
    expect(dismissed).toBe('true');
  });

  test('clicking the disconnected daemon indicator re-opens onboarding', async ({ page }) => {
    await page.route('**/api/v1/health/daemon', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'disconnected' }),
      });
    });

    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('vaultmtg_onboarding_dismissed');
      localStorage.removeItem('vaultmtg_onboarding_completed');
    });

    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();

    // Dismiss
    await page.locator('[data-testid="onboarding-modal-close"]').click();
    await expect(page.locator('[data-testid="onboarding-modal"]')).not.toBeVisible();

    // Click the daemon health indicator to re-open
    await page.locator('[data-testid="daemon-health-indicator"]').click();
    await expect(page.locator('[data-testid="onboarding-modal"]')).toBeVisible();
  });
});
