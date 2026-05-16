import { test, expect, Page } from '@playwright/test';

/**
 * Home Page E2E Tests (#2005)
 *
 * Verifies the /home route — the default authenticated landing page.
 *
 * Approach: VITE_CLERK_TEST_MODE=true aliases @clerk/react to clerkMock.tsx.
 * Auth state is injected via window.__CLERK_TEST_STATE__ before each navigation
 * using page.addInitScript(), matching the pattern in auth.spec.ts and sse-auth.spec.ts.
 *
 * The /home route is inside ProtectedRoute; without signed-in state the mock
 * renders the sign-in prompt instead of Home content.
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

test.describe('Feature: Home page (#2005)', () => {
  // AC1 / AC4 — root redirect and authenticated landing
  test.describe('Navigation and page load', () => {
    test('AC1 @smoke: navigating to / redirects authenticated users to /home', async ({ page }) => {
      await setClerkSignedIn(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // URL must end at /home
      await page.waitForURL('**/home');
      await expect(page).toHaveURL(/\/home$/);
    });

    test('AC4 @smoke: authenticated users land on the Home page at /home', async ({ page }) => {
      await setClerkSignedIn(page);
      await page.goto('/home');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // The Home page root element must be rendered
      await expect(page.locator('[data-testid="home-page"]')).toBeVisible();
    });

    test('AC4: Home page shows a personalised welcome heading', async ({ page }) => {
      await setClerkSignedIn(page, { firstName: 'Test' });
      await page.goto('/home');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      const title = page.locator('[data-testid="home-title"]');
      await expect(title).toBeVisible();
      await expect(title).toContainText('Welcome back');
    });

    test('unauthenticated visit to /home shows sign-in prompt, not Home content', async ({ page }) => {
      await setClerkSignedOut(page);
      await page.goto('/home');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      // ProtectedRoute must show the sign-in prompt
      await expect(page.locator('[data-testid="protected-route-prompt"]')).toBeVisible();

      // Home content must NOT be rendered
      await expect(page.locator('[data-testid="home-page"]')).not.toBeVisible();
    });
  });

  // AC2 — 4 feature entry points are visible
  test.describe('AC2: Feature entry points', () => {
    test.beforeEach(async ({ page }) => {
      await setClerkSignedIn(page);
      await page.goto('/home');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
      await expect(page.locator('[data-testid="home-page"]')).toBeVisible();
    });

    test('@smoke all 4 feature cards are visible', async ({ page }) => {
      const features = page.locator('[data-testid="home-features"]');
      await expect(features).toBeVisible();

      await expect(page.locator('[data-testid="home-feature-match-history"]')).toBeVisible();
      await expect(page.locator('[data-testid="home-feature-draft"]')).toBeVisible();
      await expect(page.locator('[data-testid="home-feature-decks"]')).toBeVisible();
      await expect(page.locator('[data-testid="home-feature-collection"]')).toBeVisible();
    });

    test('Match History feature card is visible with correct label', async ({ page }) => {
      const card = page.locator('[data-testid="home-feature-match-history"]');
      await expect(card).toBeVisible();
      await expect(card).toContainText('Match History');
    });

    test('Draft feature card is visible with correct label', async ({ page }) => {
      const card = page.locator('[data-testid="home-feature-draft"]');
      await expect(card).toBeVisible();
      await expect(card).toContainText('Draft');
    });

    test('Decks feature card is visible with correct label', async ({ page }) => {
      const card = page.locator('[data-testid="home-feature-decks"]');
      await expect(card).toBeVisible();
      await expect(card).toContainText('Decks');
    });

    test('Collection feature card is visible with correct label', async ({ page }) => {
      const card = page.locator('[data-testid="home-feature-collection"]');
      await expect(card).toBeVisible();
      await expect(card).toContainText('Collection');
    });
  });

  // AC2 — clicking a feature card navigates to the correct route
  test.describe('AC2: Feature card navigation', () => {
    test.beforeEach(async ({ page }) => {
      await setClerkSignedIn(page);
      await page.goto('/home');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
      await expect(page.locator('[data-testid="home-page"]')).toBeVisible();
    });

    test('@smoke clicking Match History card navigates to /match-history', async ({ page }) => {
      await page.locator('[data-testid="home-feature-match-history"]').click();
      await page.waitForURL('**/match-history');
      await expect(page).toHaveURL(/\/match-history$/);
    });

    test('@smoke clicking Draft card navigates to /draft', async ({ page }) => {
      await page.locator('[data-testid="home-feature-draft"]').click();
      await page.waitForURL('**/draft');
      await expect(page).toHaveURL(/\/draft$/);
    });

    test('clicking Decks card navigates to /decks', async ({ page }) => {
      await page.locator('[data-testid="home-feature-decks"]').click();
      await page.waitForURL('**/decks');
      await expect(page).toHaveURL(/\/decks$/);
    });

    test('clicking Collection card navigates to /collection', async ({ page }) => {
      await page.locator('[data-testid="home-feature-collection"]').click();
      await page.waitForURL('**/collection');
      await expect(page).toHaveURL(/\/collection$/);
    });
  });

  // AC3 — root no longer goes to /match-history
  test.describe('AC3: Root redirect changed', () => {
    test('@smoke / redirects to /home, not /match-history', async ({ page }) => {
      await setClerkSignedIn(page);
      await page.goto('/');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

      await page.waitForURL('**/home');
      await expect(page).toHaveURL(/\/home$/);
      await expect(page).not.toHaveURL(/\/match-history/);
    });
  });
});
