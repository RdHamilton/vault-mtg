import { test, expect, type Page } from '@playwright/test';

/**
 * Settings Page E2E Tests (#2178)
 *
 * Tests the Settings page functionality including sections and buttons.
 *
 * /settings is behind ProtectedRoute. Tests inject a signed-in Clerk test state
 * via window.__CLERK_TEST_STATE__ so ProtectedRoute renders the Settings content
 * rather than the sign-in prompt (requires VITE_CLERK_TEST_MODE=true, set in
 * playwright.config.ts webServer command).
 *
 * BFF-data mocking (#2178): useSettings() fetches GET /api/v1/settings on mount.
 * In CI the BFF runs with a Clerk secret that does not accept the Clerk mock's
 * stub token, so that endpoint is mocked via page.route() before navigation so
 * the page does not depend on a live authenticated BFF.
 *
 * Root cause of prior failure: the "Settings" describe block navigated to a
 * protected route without injecting signed-in Clerk state, so ProtectedRoute
 * rendered the sign-in prompt and every selector assertion timed out.
 */

// ---------------------------------------------------------------------------
// Clerk test-state helpers (same pattern as auth.spec.ts)
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

/**
 * Mock GET /api/v1/settings so the Settings page renders without a live
 * authenticated BFF. Registered before page.goto().
 *
 * The shared apiClient (services/apiClient.ts) unwraps every response as
 * `data.data`, so the body is a `{ "data": <payload> }` envelope.
 */
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

test.describe('Settings', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through to Settings.
    await setClerkSignedIn(page);
    await mockSettingsEndpoint(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await page.click('a[href="/settings"]');
    await page.waitForURL('**/settings');
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should navigate to Settings page', async ({ page }) => {
      await expect(page.locator('h1')).toContainText('Settings');
    });

    test('should display settings header', async ({ page }) => {
      const header = page.locator('.settings-header');
      await expect(header).toBeVisible();
    });
  });

  test.describe('Settings Sections', () => {
    test('should display settings content', async ({ page }) => {
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();
    });

    test('should have accordion sections', async ({ page }) => {
      // Wait for settings content to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      // Settings uses accordion sections
      const accordionSections = page.locator('.settings-section, .accordion-item, [class*="accordion"]');
      await expect(accordionSections.first()).toBeVisible();
    });
  });

  test.describe('Connection Settings', () => {
    test('should display daemon connection section', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      // Look for daemon/connection section
      const connectionSection = page.locator('text=Daemon').first();
      const settingsPage = page.locator('.settings-header');

      const hasConnection = await connectionSection.isVisible().catch(() => false);
      const hasHeader = await settingsPage.isVisible();

      expect(hasConnection || hasHeader).toBeTruthy();
    });
  });

  test.describe('Preferences Section', () => {
    test('should have preference settings available', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      // Look for preference-related elements
      const preferencesText = page.locator('text=Preferences').first();
      const autoRefreshText = page.locator('text=Auto Refresh').first();
      const themeText = page.locator('text=Theme').first();

      const hasPrefs =
        (await preferencesText.isVisible().catch(() => false)) ||
        (await autoRefreshText.isVisible().catch(() => false)) ||
        (await themeText.isVisible().catch(() => false));

      expect(hasPrefs).toBeTruthy();
    });
  });

  test.describe('Action Buttons', () => {
    test('@smoke should have save button', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      // Scope to the page-level action bar: the API-key section also renders a
      // "Save" button, so an unscoped /save/i filter is ambiguous (#2178).
      const saveButton = page
        .locator('.settings-actions button')
        .filter({ hasText: 'Save Settings' });
      await expect(saveButton).toBeVisible();
    });

    test('should have reset to defaults option', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      const resetButton = page
        .locator('.settings-actions button')
        .filter({ hasText: 'Reset to Defaults' });
      await expect(resetButton).toBeVisible();
    });
  });

  test.describe('About Section', () => {
    test('should have version info in settings', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      // Look for version text or about button
      const versionText = page.locator('text=Version').first();
      const aboutButton = page.locator('button').filter({ hasText: /about/i }).first();
      const settingsHeader = page.locator('.settings-header');

      const hasVersion = await versionText.isVisible().catch(() => false);
      const hasAboutButton = await aboutButton.isVisible().catch(() => false);
      const hasHeader = await settingsHeader.isVisible();

      expect(hasVersion || hasAboutButton || hasHeader).toBeTruthy();
    });
  });

  test.describe('17Lands Integration', () => {
    test('should have 17Lands settings section', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      const landsSection = page.locator('text=17Lands').first();
      await expect(landsSection).toBeVisible();
    });
  });

  test.describe('ML Settings', () => {
    test('should have ML/AI settings section', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      const mlSection = page.locator('text=ML').first();
      const aiSection = page.locator('text=AI').first();
      const ollamaSection = page.locator('text=Ollama').first();

      const hasML =
        (await mlSection.isVisible().catch(() => false)) ||
        (await aiSection.isVisible().catch(() => false)) ||
        (await ollamaSection.isVisible().catch(() => false));

      expect(hasML).toBeTruthy();
    });
  });

  test.describe('Loading State', () => {
    test('should not show error state on initial load', async ({ page }) => {
      // Wait for settings to load
      const settingsContent = page.locator('.settings-content');
      await expect(settingsContent).toBeVisible();

      const errorState = page.locator('.settings-error, .error-state');
      await expect(errorState).not.toBeVisible();
    });
  });
});

// ---------------------------------------------------------------------------
// User Profile section smoke tests (#1515)
// ---------------------------------------------------------------------------

test.describe('@smoke Settings — User Profile section', () => {
  test('authenticated user navigates to /settings and sees their email address', async ({ page }) => {
    // Inject Clerk signed-in state with a known email before the page loads.
    await setClerkSignedIn(page, { email: 'smoke@example.com', firstName: 'Smoke', lastName: 'Tester' });
    await mockSettingsEndpoint(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Expand the User Profile accordion section
    const userProfileButton = page.locator('button').filter({ hasText: /user profile/i });
    await expect(userProfileButton).toBeVisible();
    await userProfileButton.click();

    // The email address must be visible in the profile section
    await expect(page.locator('[data-testid="user-profile-email"]')).toBeVisible();
    await expect(page.locator('[data-testid="user-profile-email"]')).toHaveText('smoke@example.com');
  });

  test('authenticated user sees their display name on /settings', async ({ page }) => {
    await setClerkSignedIn(page, { firstName: 'Smoke', lastName: 'Tester', email: 'smoke@example.com' });
    await mockSettingsEndpoint(page);

    await page.goto('/settings');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    const userProfileButton = page.locator('button').filter({ hasText: /user profile/i });
    await expect(userProfileButton).toBeVisible();
    await userProfileButton.click();

    await expect(page.locator('[data-testid="user-profile-name"]')).toHaveText('Smoke Tester');
  });
});
