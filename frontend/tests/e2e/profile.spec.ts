import { test, expect, type Page } from '@playwright/test';

/**
 * Profile Page E2E Tests (#2025)
 *
 * Covers the dedicated /profile route: content rendering (name, avatar
 * placeholder, email) and the display-name edit flow.
 *
 * Auth approach: same pattern as auth.spec.ts — inject
 * window.__CLERK_TEST_STATE__ via page.addInitScript() so the Clerk mock
 * (src/test/mocks/clerkMock.tsx) returns a controlled signed-in user without
 * needing a real Clerk session or network call.
 *
 * The mock's useUser() returns:
 *   { isLoaded: true, isSignedIn: true, user: { firstName, lastName, fullName,
 *     primaryEmailAddress: { emailAddress }, imageUrl: '', id, update, setProfileImage } }
 *
 * Note: Profile.tsx accepts a `useUserHook` prop for DI, but in E2E the real
 * component is mounted (no prop injection). Auth state flows through the Clerk
 * mock, which is identical to the pattern used by settings.spec.ts and
 * auth.spec.ts.
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type ClerkTestState = {
  isSignedIn: boolean;
  firstName?: string;
  lastName?: string;
  email?: string;
};

/** Inject signed-in state before page load. Must be called before page.goto(). */
async function setClerkSignedIn(page: Page, user?: Partial<ClerkTestState>): Promise<void> {
  const state: ClerkTestState = {
    isSignedIn: true,
    firstName: user?.firstName ?? 'Planeswalker',
    lastName: user?.lastName ?? 'Mock',
    email: user?.email ?? 'planeswalker@vaultmtg.test',
  };
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, state);
}

/** Inject signed-out state before page load. Must be called before page.goto(). */
async function setClerkSignedOut(page: Page): Promise<void> {
  const state: ClerkTestState = { isSignedIn: false };
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, state);
}

// ---------------------------------------------------------------------------
// Navigation and content rendering
// ---------------------------------------------------------------------------

test.describe('@smoke Profile page — navigation and content', () => {
  test('authenticated user navigating to /profile sees the profile page @smoke', async ({ page }) => {
    await setClerkSignedIn(page, { firstName: 'Planeswalker', lastName: 'Mock', email: 'planeswalker@vaultmtg.test' });

    await page.goto('/profile');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // The profile page container must render
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    // Page title
    await expect(page.locator('[data-testid="profile-title"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-title"]')).toContainText('User Profile');
  });

  test('profile page renders display name @smoke', async ({ page }) => {
    await setClerkSignedIn(page, { firstName: 'Planeswalker', lastName: 'Mock', email: 'planeswalker@vaultmtg.test' });

    await page.goto('/profile');
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    // Display name section and value
    await expect(page.locator('[data-testid="profile-name-section"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-name-value"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-name-value"]')).toContainText('Planeswalker Mock');
  });

  test('profile page renders email address @smoke', async ({ page }) => {
    await setClerkSignedIn(page, { firstName: 'Planeswalker', lastName: 'Mock', email: 'planeswalker@vaultmtg.test' });

    await page.goto('/profile');
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    // Email section and value
    await expect(page.locator('[data-testid="profile-email-section"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-email-value"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-email-value"]')).toHaveText('planeswalker@vaultmtg.test');
  });

  test('profile page renders avatar placeholder when no imageUrl is set @smoke', async ({ page }) => {
    await setClerkSignedIn(page, { firstName: 'Planeswalker', lastName: 'Mock' });

    await page.goto('/profile');
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    // Avatar section is present
    await expect(page.locator('[data-testid="profile-avatar-section"]')).toBeVisible();

    // No imageUrl → placeholder with the initial letter
    await expect(page.locator('[data-testid="profile-avatar-placeholder"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-avatar-placeholder"]')).toContainText('P');
  });
});

// ---------------------------------------------------------------------------
// Unauthenticated access
// ---------------------------------------------------------------------------

test.describe('Profile page — unauthenticated', () => {
  test('unauthenticated user visiting /profile sees the ProtectedRoute sign-in prompt', async ({ page }) => {
    await setClerkSignedOut(page);

    await page.goto('/profile');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // ProtectedRoute must intercept and show the sign-in prompt
    await expect(page.locator('[data-testid="protected-route-prompt"]')).toBeVisible();

    // Profile page content must NOT render
    await expect(page.locator('[data-testid="profile-page"]')).not.toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Name-edit flow
// ---------------------------------------------------------------------------

test.describe('@smoke Profile page — name-edit flow', () => {
  test('Edit button opens the name-edit form with pre-filled first and last name inputs @smoke', async ({ page }) => {
    await setClerkSignedIn(page, { firstName: 'Planeswalker', lastName: 'Mock', email: 'planeswalker@vaultmtg.test' });

    await page.goto('/profile');
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    // Click Edit in the Display Name section
    await page.click('[data-testid="profile-edit-name-button"]');

    // The name form must appear
    await expect(page.locator('[data-testid="profile-name-form"]')).toBeVisible();

    // Inputs are pre-filled with the current name
    await expect(page.locator('[data-testid="profile-first-name-input"]')).toHaveValue('Planeswalker');
    await expect(page.locator('[data-testid="profile-last-name-input"]')).toHaveValue('Mock');
  });

  test('Cancel button dismisses the name-edit form without changes @smoke', async ({ page }) => {
    await setClerkSignedIn(page, { firstName: 'Planeswalker', lastName: 'Mock', email: 'planeswalker@vaultmtg.test' });

    await page.goto('/profile');
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    await page.click('[data-testid="profile-edit-name-button"]');
    await expect(page.locator('[data-testid="profile-name-form"]')).toBeVisible();

    // Cancel — form must disappear and display reverts
    await page.click('[data-testid="profile-cancel-name-button"]');
    await expect(page.locator('[data-testid="profile-name-form"]')).not.toBeVisible();
    await expect(page.locator('[data-testid="profile-name-display"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-name-value"]')).toContainText('Planeswalker Mock');
  });

  test('Save button submits the updated name and shows success feedback @smoke', async ({ page }) => {
    // The Clerk mock's useUser() returns an update() function that resolves immediately,
    // so the save flow runs through its happy path without a real API call.
    await setClerkSignedIn(page, { firstName: 'Planeswalker', lastName: 'Mock', email: 'planeswalker@vaultmtg.test' });

    await page.goto('/profile');
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    // Open the edit form
    await page.click('[data-testid="profile-edit-name-button"]');
    await expect(page.locator('[data-testid="profile-name-form"]')).toBeVisible();

    // Clear and type a new first name
    await page.fill('[data-testid="profile-first-name-input"]', 'Teferi');
    await page.fill('[data-testid="profile-last-name-input"]', 'Hero');

    // Save
    await page.click('[data-testid="profile-save-name-button"]');

    // After save the form closes and the success message appears
    await expect(page.locator('[data-testid="profile-name-form"]')).not.toBeVisible();
    await expect(page.locator('[data-testid="profile-name-success"]')).toBeVisible();
    await expect(page.locator('[data-testid="profile-name-success"]')).toContainText('Display name updated successfully');
  });
});

// ---------------------------------------------------------------------------
// Back button
// ---------------------------------------------------------------------------

test.describe('Profile page — back button', () => {
  test('Back button is visible on the profile page', async ({ page }) => {
    await setClerkSignedIn(page);

    await page.goto('/profile');
    await expect(page.locator('[data-testid="profile-page"]')).toBeVisible();

    await expect(page.locator('[data-testid="profile-back-button"]')).toBeVisible();
  });
});
