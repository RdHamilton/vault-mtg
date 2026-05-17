import { test, expect, Page } from '@playwright/test';

/**
 * Match History E2E Tests (#2000, #2061, #2178)
 *
 * Tests the cloud match-history page at /match-history, served by the
 * BffMatchHistory component. BffMatchHistory fetches the Clerk-protected
 * GET /api/v1/history/matches endpoint and renders a paginated table, an
 * empty state, or an error state.
 *
 * Auth approach: the Vite build is produced with VITE_CLERK_TEST_MODE=true which
 * aliases @clerk/react to src/test/mocks/clerkMock.tsx. That mock reads
 * window.__CLERK_TEST_STATE__ — injected via page.addInitScript() — so tests
 * control auth state without a real Clerk publishable key.
 *
 * Default state (no injection or { isSignedIn: false }): signed-out.
 *   ProtectedRoute renders the sign-in prompt instead of page content.
 *
 * Signed-in state ({ isSignedIn: true }): ProtectedRoute passes through and
 *   BffMatchHistory renders, calling the BFF API.
 *
 * History (#2178): this spec previously tested the legacy MatchHistory component
 * (date-range / format / queue filters, sortable headers, .match-history-table-
 * container). The /match-history route was re-pointed to BffMatchHistory in
 * #1918 — that legacy UI no longer exists — and the spec navigated to '/' which
 * now redirects to /home. The spec was rewritten to exercise the real
 * BffMatchHistory markup, navigate directly to /match-history, and mock the BFF
 * response via page.route() so it does not depend on a live authenticated BFF
 * (the CI BFF rejects the Clerk mock's stub token).
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

type MatchRow = {
  id: number;
  opponent_deck: string;
  result: string;
  format: string;
  played_at: string;
};

/**
 * Mock GET /api/v1/history/matches with fixture rows so BffMatchHistory does not
 * depend on a live authenticated BFF. Must be registered before page.goto().
 */
async function mockMatchHistory(page: Page, rows: MatchRow[], total = rows.length): Promise<void> {
  await page.route('**/api/v1/history/matches**', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ matches: rows, total, limit: 20, offset: 0 }),
    });
  });
}

const MATCH_ROWS: MatchRow[] = Array.from({ length: 20 }, (_, i) => ({
  id: i + 1,
  opponent_deck: `Opponent Deck ${i + 1}`,
  result: i % 2 === 0 ? 'win' : 'loss',
  format: 'Standard',
  played_at: '2026-05-01T12:00:00Z',
}));

// ---------------------------------------------------------------------------
// Tests — signed-in, table rendered
// ---------------------------------------------------------------------------

test.describe('Match History', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in state so ProtectedRoute allows /match-history to render,
    // and mock the BFF response so the table renders deterministically (#2178).
    await setClerkSignedIn(page);
    await mockMatchHistory(page, MATCH_ROWS);
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should display the Match History page', async ({ page }) => {
      await page.goto('/match-history');
      await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 15_000 });
      await expect(page.locator('h1.page-title')).toHaveText('Match History');
    });

    test('should render the match history table container', async ({ page }) => {
      await page.goto('/match-history');
      await expect(page.locator('[data-testid="match-history-page"]')).toBeVisible();
      await expect(page.locator('[data-testid="match-history-table"]')).toBeVisible();
    });
  });

  test.describe('Match Table', () => {
    test('@smoke should display the expected column headers', async ({ page }) => {
      await page.goto('/match-history');

      const table = page.locator('[data-testid="match-history-table"]');
      await expect(table).toBeVisible();

      const headerTexts = await table.locator('thead th').allTextContents();
      expect(headerTexts).toContain('Date');
      expect(headerTexts).toContain('Format');
      expect(headerTexts).toContain('Opponent Deck');
      expect(headerTexts).toContain('Result');
    });

    test('should render a row for every match returned by the BFF', async ({ page }) => {
      await page.goto('/match-history');

      const table = page.locator('[data-testid="match-history-table"]');
      await expect(table).toBeVisible();
      await expect(table.locator('tbody tr')).toHaveCount(MATCH_ROWS.length);
    });
  });

  test.describe('Pagination', () => {
    test('should display pagination controls when total exceeds the page size', async ({ page }) => {
      // 20 rows, total = 41 → more than one page → footer renders.
      await mockMatchHistory(page, MATCH_ROWS, 41);
      await page.goto('/match-history');

      await expect(page.locator('[data-testid="match-history-table"]')).toBeVisible();

      const pageInfo = page.locator('.pagination-info');
      await expect(pageInfo).toBeVisible();
      await expect(pageInfo).toContainText('Page');
    });
  });

  test.describe('Loading State', () => {
    test('should not show error state on initial load', async ({ page }) => {
      await page.goto('/match-history');

      await expect(page.locator('[data-testid="match-history-table"]')).toBeVisible();
      await expect(page.locator('.error-state')).not.toBeVisible();
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
          if (
            text.includes('ERR_CONNECTION_REFUSED') ||
            text.includes('9001')
          ) {
            daemonErrors.push(text);
          }
        }
      });

      await page.goto('/match-history');
      await expect(page.locator('[data-testid="match-history-page"]')).toBeVisible();

      // Wait for the page to finish loading so all API calls have fired.
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {/* ignore timeout */});

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

      await page.goto('/match-history');
      await expect(page.locator('[data-testid="match-history-page"]')).toBeVisible();

      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {/* ignore timeout */});

      expect(
        daemonRequests,
        `Match History page sent requests to the daemon (port 9001): ${daemonRequests.join(', ')}`,
      ).toHaveLength(0);
    });
  });
});

// ---------------------------------------------------------------------------
// Empty state — signed-in, no matches
// ---------------------------------------------------------------------------

test.describe('Match History — empty state', () => {
  test('shows the empty state when the BFF returns no matches', async ({ page }) => {
    await setClerkSignedIn(page);
    await mockMatchHistory(page, [], 0);

    await page.goto('/match-history');

    await expect(page.locator('[data-testid="match-history-empty"]')).toBeVisible();
    await expect(page.locator('[data-testid="match-history-table"]')).not.toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Error state — signed-in, BFF error
// ---------------------------------------------------------------------------

test.describe('Match History — error state', () => {
  test('shows the error state when the BFF returns a 500', async ({ page }) => {
    await setClerkSignedIn(page);
    await page.route('**/api/v1/history/matches**', (route) => {
      void route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'internal server error' }),
      });
    });

    await page.goto('/match-history');

    await expect(page.locator('.error-state')).toBeVisible();
    await expect(page.locator('[data-testid="match-history-table"]')).not.toBeVisible();
    await expect(page.locator('[data-testid="match-history-empty"]')).not.toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Unauthenticated access — protected route shows sign-in prompt (#2000)
//
// When { isSignedIn: false } is injected the Clerk mock reports signed-out.
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
    await expect(page.locator('[data-testid="match-history-table"]')).not.toBeVisible();
    await expect(page.locator('[data-testid="match-history-empty"]')).not.toBeVisible();
  });
});
