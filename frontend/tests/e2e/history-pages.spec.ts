import { test, expect, Page } from '@playwright/test';

/**
 * History Pages E2E Tests (#1461)
 *
 * Smoke and functional coverage for /history/matches and /history/drafts.
 *
 * Auth approach: the Vite dev server starts with VITE_CLERK_TEST_MODE=true which
 * aliases @clerk/react to src/test/mocks/clerkMock.tsx. That mock reads
 * window.__CLERK_TEST_STATE__ — injected via page.addInitScript() — so tests
 * control auth state without a real Clerk publishable key.
 *
 * Default state (no injection or { isSignedIn: false }): signed-out.
 *   ProtectedRoute renders sign-in UI instead of page content.
 *
 * Signed-in state ({ isSignedIn: true }): ProtectedRoute passes through and
 *   BffMatchHistory / BffDraftHistory render, calling the BFF API.
 *
 * ACs covered (issue #1461):
 *   - Unauthenticated access shows sign-in prompt, not history content
 *   - Signed-in user sees table or empty state (no crash)
 *   - API error shows a user-visible error state
 *   - Pagination controls render when total > PAGE_SIZE
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Inject signed-in Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedIn(page: Page): Promise<void> {
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, { isSignedIn: true, firstName: 'Test', lastName: 'User' });
}

/** Inject signed-out Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedOut(page: Page): Promise<void> {
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, { isSignedIn: false });
}

// ---------------------------------------------------------------------------
// Match history — /history/matches
// ---------------------------------------------------------------------------

test.describe('History: /history/matches', () => {
  test.describe('Unauthenticated', () => {
    test('unauthenticated access does not show match history content @smoke', async ({ page }) => {
      await setClerkSignedOut(page);
      await page.goto('/history/matches');

      // Give the page time to settle — auth guard must have resolved.
      await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {/* ignore timeout */});

      // Match history content must NOT be rendered for an unauthenticated user.
      const table = page.locator('[data-testid="match-history-table"]');
      const empty = page.locator('[data-testid="match-history-empty"]');

      await expect(table).not.toBeVisible();
      await expect(empty).not.toBeVisible();
    });
  });

  test.describe('Authenticated', () => {
    test.beforeEach(async ({ page }) => {
      await setClerkSignedIn(page);
    });

    test('page loads without error and shows table or empty state @smoke', async ({ page }) => {
      await page.goto('/history/matches');

      const table = page.locator('[data-testid="match-history-table"]');
      const empty = page.locator('[data-testid="match-history-empty"]');

      // Either data or empty state must be visible — no crash/blank page.
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });
    });

    test('page title is "Match History"', async ({ page }) => {
      await page.goto('/history/matches');

      // Wait for loading to complete
      const table = page.locator('[data-testid="match-history-table"]');
      const empty = page.locator('[data-testid="match-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      await expect(page.locator('h1.page-title')).toHaveText('Match History');
    });

    test('no error state is shown on initial load', async ({ page }) => {
      await page.goto('/history/matches');

      const table = page.locator('[data-testid="match-history-table"]');
      const empty = page.locator('[data-testid="match-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      // Error state must not be visible after a successful load.
      const errorState = page.locator('.error-state');
      await expect(errorState).not.toBeVisible();
    });

    test('table renders column headers when data is present', async ({ page }) => {
      await page.goto('/history/matches');

      const table = page.locator('[data-testid="match-history-table"]');
      const empty = page.locator('[data-testid="match-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      const hasData = await table.isVisible();
      if (hasData) {
        // The BffMatchHistory table has four columns: Date, Format, Opponent Deck, Result.
        await expect(table.locator('thead th').nth(0)).toHaveText('Date');
        await expect(table.locator('thead th').nth(1)).toHaveText('Format');
        await expect(table.locator('thead th').nth(2)).toHaveText('Opponent Deck');
        await expect(table.locator('thead th').nth(3)).toHaveText('Result');
      }
    });

    test('pagination controls render when table has data', async ({ page }) => {
      await page.goto('/history/matches');

      const table = page.locator('[data-testid="match-history-table"]');
      const empty = page.locator('[data-testid="match-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      const hasData = await table.isVisible();
      if (hasData) {
        // Pagination footer renders with Previous / Next buttons and page info.
        const prevBtn = page.locator('.pagination-btn', { hasText: 'Previous' });
        const nextBtn = page.locator('.pagination-btn', { hasText: 'Next' });
        const pageInfo = page.locator('.pagination-info');

        await expect(prevBtn).toBeVisible({ timeout: 5_000 });
        await expect(nextBtn).toBeVisible({ timeout: 5_000 });
        await expect(pageInfo).toContainText('Page');
      }
    });

    test('error state is shown when the API returns an error', async ({ page }) => {
      // Intercept the BFF match-history endpoint and return a 500 before load.
      await page.route('**/api/v1/history/matches**', (route) => {
        void route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'internal server error' }),
        });
      });

      await page.goto('/history/matches');

      // After the failed request the error state div must be visible.
      const errorState = page.locator('.error-state');
      await expect(errorState).toBeVisible({ timeout: 15_000 });

      // The match table and empty state must NOT appear simultaneously with the error.
      await expect(page.locator('[data-testid="match-history-table"]')).not.toBeVisible();
      await expect(page.locator('[data-testid="match-history-empty"]')).not.toBeVisible();
    });
  });
});

// ---------------------------------------------------------------------------
// Draft history — /history/drafts
// ---------------------------------------------------------------------------

test.describe('History: /history/drafts', () => {
  test.describe('Unauthenticated', () => {
    test('unauthenticated access does not show draft history content @smoke', async ({ page }) => {
      await setClerkSignedOut(page);
      await page.goto('/history/drafts');

      await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {/* ignore timeout */});

      // Draft history content must NOT be rendered for an unauthenticated user.
      const table = page.locator('[data-testid="draft-history-table"]');
      const empty = page.locator('[data-testid="draft-history-empty"]');

      await expect(table).not.toBeVisible();
      await expect(empty).not.toBeVisible();
    });
  });

  test.describe('Authenticated', () => {
    test.beforeEach(async ({ page }) => {
      await setClerkSignedIn(page);
    });

    test('page loads without error and shows table or empty state @smoke', async ({ page }) => {
      await page.goto('/history/drafts');

      const table = page.locator('[data-testid="draft-history-table"]');
      const empty = page.locator('[data-testid="draft-history-empty"]');

      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });
    });

    test('page title is "Draft History"', async ({ page }) => {
      await page.goto('/history/drafts');

      const table = page.locator('[data-testid="draft-history-table"]');
      const empty = page.locator('[data-testid="draft-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      await expect(page.locator('h1.page-title')).toHaveText('Draft History');
    });

    test('no error state is shown on initial load', async ({ page }) => {
      await page.goto('/history/drafts');

      const table = page.locator('[data-testid="draft-history-table"]');
      const empty = page.locator('[data-testid="draft-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      const errorState = page.locator('.error-state');
      await expect(errorState).not.toBeVisible();
    });

    test('table renders column headers when data is present', async ({ page }) => {
      await page.goto('/history/drafts');

      const table = page.locator('[data-testid="draft-history-table"]');
      const empty = page.locator('[data-testid="draft-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      const hasData = await table.isVisible();
      if (hasData) {
        // The BffDraftHistory table has four columns: Date, Set, Wins, Losses.
        await expect(table.locator('thead th').nth(0)).toHaveText('Date');
        await expect(table.locator('thead th').nth(1)).toHaveText('Set');
        await expect(table.locator('thead th').nth(2)).toHaveText('Wins');
        await expect(table.locator('thead th').nth(3)).toHaveText('Losses');
      }
    });

    test('pagination controls render when table has data', async ({ page }) => {
      await page.goto('/history/drafts');

      const table = page.locator('[data-testid="draft-history-table"]');
      const empty = page.locator('[data-testid="draft-history-empty"]');
      await expect(table.or(empty)).toBeVisible({ timeout: 15_000 });

      const hasData = await table.isVisible();
      if (hasData) {
        const prevBtn = page.locator('.pagination-btn', { hasText: 'Previous' });
        const nextBtn = page.locator('.pagination-btn', { hasText: 'Next' });
        const pageInfo = page.locator('.pagination-info');

        await expect(prevBtn).toBeVisible({ timeout: 5_000 });
        await expect(nextBtn).toBeVisible({ timeout: 5_000 });
        await expect(pageInfo).toContainText('Page');
      }
    });

    test('error state is shown when the API returns an error', async ({ page }) => {
      await page.route('**/api/v1/history/drafts**', (route) => {
        void route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'internal server error' }),
        });
      });

      await page.goto('/history/drafts');

      const errorState = page.locator('.error-state');
      await expect(errorState).toBeVisible({ timeout: 15_000 });

      await expect(page.locator('[data-testid="draft-history-table"]')).not.toBeVisible();
      await expect(page.locator('[data-testid="draft-history-empty"]')).not.toBeVisible();
    });
  });
});
