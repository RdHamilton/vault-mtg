import { test, expect, Page } from '@playwright/test';

/**
 * Match History E2E Tests
 *
 * Tests the main Match History page which is the default landing page.
 * Uses REST API backend for testing.
 *
 * Auth approach: the Vite dev server starts with VITE_CLERK_TEST_MODE=true which
 * aliases @clerk/react to src/test/mocks/clerkMock.tsx. That mock reads
 * window.__CLERK_TEST_STATE__ — injected via page.addInitScript() — so tests
 * control auth state without a real Clerk publishable key.
 *
 * Default state (no injection or { isSignedIn: false }): signed-out.
 *   ProtectedRoute renders sign-in prompt instead of page content.
 *
 * Signed-in state ({ isSignedIn: true }): ProtectedRoute passes through and
 *   BffMatchHistory renders, calling the BFF API.
 *
 * Fix (#2000): Added setClerkSignedIn() injection in beforeEach so that the
 * /match-history protected route receives an authenticated Clerk context in CI.
 * Without this injection the Clerk mock defaults to isSignedIn: false, causing
 * ProtectedRoute to show the sign-in prompt and every selector assertion to time
 * out with "TimeoutError waiting for sign-in email input".
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type ClerkTestState = {
  isSignedIn: boolean;
  firstName?: string;
  lastName?: string;
};

/** Inject signed-in Clerk state before page load. Must be called before page.goto(). */
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

/** Inject signed-out Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedOut(page: Page): Promise<void> {
  await page.addInitScript((s) => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = s;
  }, { isSignedIn: false });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Match History', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in state so ProtectedRoute allows /match-history to render.
    // Without this, the Clerk mock defaults to isSignedIn: false and the
    // protected route displays a sign-in prompt instead of Match History content,
    // causing all selector assertions to time out in CI (#2000).
    await setClerkSignedIn(page);
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]'), {
      message: 'App container must be visible after navigation to /',
    }).toBeVisible({ timeout: 15_000 });
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should display Match History as the default page', async ({ page }) => {
      await expect(page.locator('h1.page-title')).toHaveText('Match History');
    });

    test('should display page header with title', async ({ page }) => {
      const header = page.locator('.match-history-header');
      await expect(header).toBeVisible();
      await expect(header.locator('h1')).toHaveText('Match History');
    });
  });

  test.describe('Filter Controls', () => {
    test('@smoke should display date range filter', async ({ page }) => {
      const filterRow = page.locator('.filter-row');
      await expect(filterRow).toBeVisible();

      const dateRangeSelect = filterRow.locator('select').first();
      await expect(dateRangeSelect).toBeVisible();

      const options = await dateRangeSelect.locator('option').allTextContents();
      expect(options).toContain('Last 7 Days');
      expect(options).toContain('Last 30 Days');
      expect(options).toContain('All Time');
      expect(options).toContain('Custom Range');
    });

    test('should display card format filter', async ({ page }) => {
      const cardFormatLabel = page.locator('.filter-label').filter({ hasText: 'Card Format' });
      await expect(cardFormatLabel).toBeVisible();

      const filterGroup = cardFormatLabel.locator('..');
      const select = filterGroup.locator('select');
      await expect(select).toBeVisible();

      const options = await select.locator('option').allTextContents();
      expect(options).toContain('All Card Formats');
      expect(options).toContain('Standard');
      expect(options).toContain('Historic');
    });

    test('should display queue type filter', async ({ page }) => {
      const queueTypeLabel = page.locator('.filter-label').filter({ hasText: 'Queue Type' });
      await expect(queueTypeLabel).toBeVisible();

      const filterGroup = queueTypeLabel.locator('..');
      const select = filterGroup.locator('select');
      await expect(select).toBeVisible();

      const options = await select.locator('option').allTextContents();
      expect(options).toContain('All Queues');
      expect(options).toContain('Ranked');
      expect(options).toContain('Play Queue');
    });

    test('should display result filter', async ({ page }) => {
      const resultLabel = page.locator('.filter-label').filter({ hasText: 'Result' });
      await expect(resultLabel).toBeVisible();

      const filterGroup = resultLabel.locator('..');
      const select = filterGroup.locator('select');
      await expect(select).toBeVisible();

      const options = await select.locator('option').allTextContents();
      expect(options).toContain('All Results');
      expect(options).toContain('Wins Only');
      expect(options).toContain('Losses Only');
    });

    test('should show custom date pickers when Custom Range is selected', async ({ page }) => {
      const dateRangeSelect = page.locator('.filter-group').first().locator('select');
      await dateRangeSelect.selectOption('custom');

      const startDateInput = page.locator('input[type="date"]').first();
      const endDateInput = page.locator('input[type="date"]').last();

      await expect(startDateInput).toBeVisible();
      await expect(endDateInput).toBeVisible();
    });
  });

  test.describe('Content State', () => {
    test('should display either matches table or empty state after loading', async ({ page }) => {
      const table = page.locator('.match-history-table-container');
      const emptyState = page.locator('.empty-state');

      // Wait for either content type to appear
      await expect(table.or(emptyState)).toBeVisible();

      const hasTable = await table.isVisible();
      const hasEmptyState = await emptyState.isVisible();

      expect(hasTable || hasEmptyState).toBeTruthy();

      if (hasEmptyState) {
        await expect(emptyState.locator('.empty-state-title')).toBeVisible();
        await expect(emptyState.locator('.empty-state-message')).toBeVisible();
      }

      if (hasTable) {
        await expect(table.locator('table')).toBeVisible();
      }
    });
  });

  test.describe('Match Table', () => {
    test('should display table headers when matches exist', async ({ page }) => {
      const table = page.locator('.match-history-table-container table');
      const hasTable = await table.isVisible().catch(() => false);

      if (hasTable) {
        const headers = table.locator('thead th');
        const headerTexts = await headers.allTextContents();

        expect(headerTexts.some((h) => h.includes('Time'))).toBeTruthy();
        expect(headerTexts.some((h) => h.includes('Result'))).toBeTruthy();
        expect(headerTexts.some((h) => h.includes('Format'))).toBeTruthy();
        expect(headerTexts.some((h) => h.includes('Event'))).toBeTruthy();
        expect(headerTexts.some((h) => h.includes('Score'))).toBeTruthy();
        expect(headerTexts.some((h) => h.includes('Opponent'))).toBeTruthy();
      }
    });

    test('should have sortable column headers', async ({ page }) => {
      const table = page.locator('.match-history-table-container table');
      const hasTable = await table.isVisible().catch(() => false);

      if (hasTable) {
        const timeHeader = table.locator('thead th').first();
        await expect(timeHeader).toHaveCSS('cursor', 'pointer');
      }
    });
  });

  test.describe('Loading State', () => {
    test('should not show error state on initial load', async ({ page }) => {
      // Wait for content to load
      const content = page.locator('.match-history-table-container, .empty-state');
      await expect(content.first()).toBeVisible();

      const errorState = page.locator('.error-state');
      await expect(errorState).not.toBeVisible();
    });
  });

  test.describe('Match Count', () => {
    test('should display match count when matches exist', async ({ page }) => {
      const table = page.locator('.match-history-table-container');
      const hasTable = await table.isVisible().catch(() => false);

      if (hasTable) {
        const matchCount = page.locator('.match-count');
        await expect(matchCount).toBeVisible();
        await expect(matchCount).toContainText('Showing');
      }
    });
  });

  test.describe('Daemon independence — no ERR_CONNECTION_REFUSED (#2061)', () => {
    test('should not emit ERR_CONNECTION_REFUSED errors when navigating to Match History page', async ({ page }) => {
      // Collect all console errors to detect failed network requests to port 9001
      // (the daemon port). Before the fix in PR #2058, getHealth() was called without
      // an isDesktopApp() guard and produced ERR_CONNECTION_REFUSED when the daemon
      // was offline. This test ensures the guard is in place.
      const daemonErrors: string[] = [];
      page.on('console', (msg) => {
        if (msg.type() === 'error') {
          const text = msg.text();
          // Flag any error mentioning port 9001 or ERR_CONNECTION_REFUSED
          if (
            text.includes('ERR_CONNECTION_REFUSED') ||
            text.includes('9001')
          ) {
            daemonErrors.push(text);
          }
        }
      });

      // Wait for the page to finish loading so all API calls have fired
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {/* ignore timeout */});

      // No ERR_CONNECTION_REFUSED errors should have been emitted.
      // If this fails, getHealth() is being called without the isDesktopApp() guard.
      expect(
        daemonErrors,
        `Match History page emitted daemon connection errors: ${daemonErrors.join('; ')}`,
      ).toHaveLength(0);
    });

    test('should not make network requests to port 9001 on initial load', async ({ page }) => {
      // Intercept all network requests and flag any directed to port 9001 (the daemon).
      // The match history page should only contact the BFF on port 8080.
      const daemonRequests: string[] = [];
      page.on('request', (request) => {
        const url = request.url();
        if (url.includes('9001')) {
          daemonRequests.push(url);
        }
      });

      // Wait for network to settle so all API calls have fired
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {/* ignore timeout */});

      expect(
        daemonRequests,
        `Match History page sent requests to the daemon (port 9001): ${daemonRequests.join(', ')}`,
      ).toHaveLength(0);
    });
  });
});

// ---------------------------------------------------------------------------
// Unauthenticated access — protected route shows sign-in prompt (#2000)
//
// When no Clerk state is injected the mock defaults to isSignedIn: false.
// ProtectedRoute must show the sign-in prompt, NOT match-history content.
// ---------------------------------------------------------------------------

test.describe('Match History — unauthenticated access', () => {
  test('@smoke unauthenticated visit to /match-history shows sign-in prompt, not match content', async ({ page }) => {
    await setClerkSignedOut(page);
    await page.goto('/match-history');

    // Give the page time to resolve the auth guard.
    await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {/* ignore timeout */});

    // ProtectedRoute must render the sign-in prompt for unauthenticated users.
    await expect(page.locator('[data-testid="protected-route-prompt"]'), {
      message: 'ProtectedRoute must show the sign-in prompt for unauthenticated users on /match-history',
    }).toBeVisible({ timeout: 10_000 });

    // The prompt must contain the sign-in action button.
    await expect(page.locator('[data-testid="protected-route-sign-in-btn"]')).toBeVisible();

    // Match History content must NOT be rendered without authentication.
    await expect(page.locator('.match-history-table-container')).not.toBeVisible();
    await expect(page.locator('.filter-row')).not.toBeVisible();
  });
});
