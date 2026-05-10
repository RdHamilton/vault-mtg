import { test, expect, Page } from '@playwright/test';

/**
 * Clerk Auth UI E2E Tests (#1258)
 *
 * Verifies the Clerk auth flow: signed-out UI, protected route redirect, and signed-in UI.
 *
 * Approach: The Vite dev server is started with VITE_CLERK_TEST_MODE=true, which aliases
 * @clerk/react to src/test/mocks/clerkMock.tsx. That mock reads auth state from
 * window.__CLERK_TEST_STATE__ — an object we inject via page.addInitScript() before
 * each navigation. This avoids needing a real Clerk publishable key and eliminates
 * flaky CDN intercepts.
 *
 * window.__CLERK_TEST_STATE__ shape:
 *   { isSignedIn: boolean; firstName?: string; lastName?: string }
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type ClerkTestState = {
  isSignedIn: boolean;
  firstName?: string;
  lastName?: string;
};

/** Inject signed-out state before page load. Must be called before page.goto(). */
async function setClerkSignedOut(page: Page): Promise<void> {
  const state: ClerkTestState = { isSignedIn: false };
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, state);
}

/** Inject signed-in state before page load. Must be called before page.goto(). */
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Feature: Clerk Auth UI', () => {
  test('signed-out state: AuthBar shows SignInButton and SignUpButton; UserButton is NOT visible @smoke', async ({ page }) => {
    await setClerkSignedOut(page);
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Auth bar container must be present
    const authBar = page.locator('[data-testid="auth-bar"]');
    await expect(authBar).toBeVisible();

    // Signed-out section is visible with both action buttons
    await expect(page.locator('[data-testid="auth-signed-out"]')).toBeVisible();
    await expect(page.locator('[data-testid="sign-in-btn"]')).toBeVisible();
    await expect(page.locator('[data-testid="sign-up-btn"]')).toBeVisible();

    // Signed-in section (UserButton) must NOT be present
    await expect(page.locator('[data-testid="auth-signed-in"]')).not.toBeVisible();
  });

  test('protected route redirect: navigating to /draft while unauthenticated shows sign-in prompt, not Draft content @smoke', async ({ page }) => {
    await setClerkSignedOut(page);
    await page.goto('/draft');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // ProtectedRoute sign-in prompt must be shown
    await expect(page.locator('[data-testid="protected-route-prompt"]')).toBeVisible();

    // The prompt should contain the sign-in button
    await expect(page.locator('[data-testid="protected-route-sign-in-btn"]')).toBeVisible();

    // Draft-specific content must NOT be rendered
    await expect(page.locator('.draft-container')).not.toBeVisible();
    await expect(page.locator('[data-testid="protected-route-loading"]')).not.toBeVisible();
  });

  test('protected route redirect: sign-in prompt title mentions Draft access', async ({ page }) => {
    await setClerkSignedOut(page);
    await page.goto('/draft');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await expect(page.locator('[data-testid="protected-route-prompt"]')).toBeVisible();
    await expect(page.locator('text=Sign in to access Draft')).toBeVisible();
  });

  test('signed-in state: UserButton is visible; SignInButton and SignUpButton are NOT visible @smoke', async ({ page }) => {
    await setClerkSignedIn(page);
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Auth bar must be present
    await expect(page.locator('[data-testid="auth-bar"]')).toBeVisible();

    // Signed-in section must be visible
    await expect(page.locator('[data-testid="auth-signed-in"]')).toBeVisible();

    // Signed-out buttons must NOT be visible
    await expect(page.locator('[data-testid="auth-signed-out"]')).not.toBeVisible();
    await expect(page.locator('[data-testid="sign-in-btn"]')).not.toBeVisible();
    await expect(page.locator('[data-testid="sign-up-btn"]')).not.toBeVisible();
  });

  test('signed-in state: navigating to /draft renders Draft content, not the sign-in prompt', async ({ page }) => {
    await setClerkSignedIn(page);
    await page.goto('/draft');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Sign-in prompt must NOT appear
    await expect(page.locator('[data-testid="protected-route-prompt"]')).not.toBeVisible();

    // Draft content (container) must render — .draft-container wraps .draft-empty
    await expect(page.locator('.draft-container').first()).toBeVisible();
  });
});
