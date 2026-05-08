import { test, expect, Page } from '@playwright/test';

/**
 * API Keys Page E2E tests (#1314)
 *
 * Verifies that authenticated users can navigate to the /api-keys page
 * and interact with the Clerk API Keys component.
 *
 * The Clerk mock (src/test/mocks/clerkMock.tsx) provides a stub APIKeys
 * component that renders data-testid elements we can assert against.
 * Auth state is injected via window.__CLERK_TEST_STATE__ before navigation.
 */

// ---------------------------------------------------------------------------
// Helpers
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

async function setClerkSignedOut(page: Page): Promise<void> {
  const state: ClerkTestState = { isSignedIn: false };
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, state);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Feature: API Keys page', () => {
  test(
    'signed-in user navigates to /api-keys and sees the Clerk API Keys component @smoke',
    async ({ page }) => {
      await setClerkSignedIn(page);
      await page.goto('/api-keys');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

      // Page container must render
      await expect(page.locator('[data-testid="api-keys-page"]')).toBeVisible({ timeout: 10_000 });

      // Page title
      await expect(page.locator('h1')).toContainText('API Keys');

      // Clerk APIKeys stub component must be present
      await expect(page.locator('[data-testid="clerk-api-keys-component"]')).toBeVisible();
    }
  );

  test('signed-in user sees create API key button', async ({ page }) => {
    await setClerkSignedIn(page);
    await page.goto('/api-keys');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    await expect(page.locator('[data-testid="clerk-create-api-key-btn"]')).toBeVisible({ timeout: 10_000 });
  });

  test('signed-in user sees API key list area', async ({ page }) => {
    await setClerkSignedIn(page);
    await page.goto('/api-keys');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    await expect(page.locator('[data-testid="clerk-api-key-list"]')).toBeVisible({ timeout: 10_000 });
  });

  test('signed-in user sees description mentioning one-time key visibility', async ({ page }) => {
    await setClerkSignedIn(page);
    await page.goto('/api-keys');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    await expect(
      page.locator('.api-keys-description')
    ).toContainText('full key is only shown once', { timeout: 10_000 });
  });

  test('unauthenticated user visiting /api-keys sees sign-in prompt, not the keys page', async ({ page }) => {
    await setClerkSignedOut(page);
    await page.goto('/api-keys');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    // ProtectedRoute must intercept
    await expect(page.locator('[data-testid="protected-route-prompt"]')).toBeVisible({ timeout: 10_000 });

    // API keys page content must NOT render
    await expect(page.locator('[data-testid="api-keys-page"]')).not.toBeVisible();
  });

  test('api-keys-content container wraps the Clerk component', async ({ page }) => {
    await setClerkSignedIn(page);
    await page.goto('/api-keys');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });

    const content = page.locator('[data-testid="api-keys-content"]');
    await expect(content).toBeVisible({ timeout: 10_000 });

    // Clerk stub is a child of the content container
    await expect(content.locator('[data-testid="clerk-api-keys-component"]')).toBeVisible();
  });
});
