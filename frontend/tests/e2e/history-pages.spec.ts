import { test, expect, Page } from '@playwright/test';

/**
 * History Pages E2E Tests (#1461, #2178)
 *
 * Smoke and functional coverage for the cloud match-history page (/match-history,
 * served by BffMatchHistory) and the draft-history page (/history/drafts, served
 * by BffDraftHistory).
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
 *   BffMatchHistory / BffDraftHistory render and call the BFF API.
 *
 * BFF-data mocking (#2178): in CI the BFF runs with a Clerk secret that does not
 * accept the Clerk mock's stub token, so the real Clerk-protected
 * /api/v1/history/* endpoints reject every request. To keep these tests
 * independent of a live authenticated BFF, every authenticated test installs a
 * page.route() interceptor that fulfils /api/v1/history/matches and
 * /api/v1/history/drafts with deterministic fixture data before navigation.
 *
 * Route note (#2178): there is no /history/matches route — the cloud match
 * history page lives at /match-history (App.tsx). These tests target
 * /match-history directly.
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

type DraftRow = {
  id: number;
  set_code: string;
  wins: number;
  losses: number;
  drafted_at: string;
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

/**
 * Mock GET /api/v1/history/drafts with fixture rows so BffDraftHistory does not
 * depend on a live authenticated BFF. Must be registered before page.goto().
 */
async function mockDraftHistory(page: Page, rows: DraftRow[], total = rows.length): Promise<void> {
  await page.route('**/api/v1/history/drafts**', (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ drafts: rows, total, limit: 20, offset: 0 }),
    });
  });
}

// A page of 20 match rows so the table (not the empty state) renders and the
// total exceeds PAGE_SIZE, which makes the pagination footer appear.
const MATCH_ROWS: MatchRow[] = Array.from({ length: 20 }, (_, i) => ({
  id: i + 1,
  opponent_deck: `Opponent Deck ${i + 1}`,
  result: i % 2 === 0 ? 'win' : 'loss',
  format: 'Standard',
  played_at: '2026-05-01T12:00:00Z',
}));

const DRAFT_ROWS: DraftRow[] = Array.from({ length: 20 }, (_, i) => ({
  id: i + 1,
  set_code: 'TDM',
  wins: 7,
  losses: 2,
  drafted_at: '2026-05-01T12:00:00Z',
}));

// ---------------------------------------------------------------------------
// Match history — /match-history (BffMatchHistory)
// ---------------------------------------------------------------------------

test.describe('History: /match-history', () => {
  test.describe('Unauthenticated', () => {
    test('unauthenticated access does not show match history content @smoke', async ({ page }) => {
      await setClerkSignedOut(page);
      await page.goto('/match-history');

      // Give the page time to settle — auth guard must have resolved.
      await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {/* ignore timeout */});

      // Match history content must NOT be rendered for an unauthenticated user.
      const table = page.locator('[data-testid="match-history-table"]');
      const empty = page.locator('[data-testid="match-history-empty"]');

      await expect(table).not.toBeVisible();
      await expect(empty).not.toBeVisible();

      // ProtectedRoute must show the sign-in prompt instead.
      await expect(page.locator('[data-testid="protected-route-prompt"]')).toBeVisible();
    });
  });

  test.describe('Authenticated — with data', () => {
    test.beforeEach(async ({ page }) => {
      await setClerkSignedIn(page);
      await mockMatchHistory(page, MATCH_ROWS);
    });

    test('page loads without error and shows the match table @smoke', async ({ page }) => {
      await page.goto('/match-history');

      await expect(page.locator('[data-testid="match-history-table"]')).toBeVisible();
    });

    test('page title is "Match History"', async ({ page }) => {
      await page.goto('/match-history');

      await expect(page.locator('[data-testid="match-history-table"]')).toBeVisible();
      await expect(page.locator('h1.page-title')).toHaveText('Match History');
    });

    test('no error state is shown on initial load', async ({ page }) => {
      await page.goto('/match-history');

      await expect(page.locator('[data-testid="match-history-table"]')).toBeVisible();
      await expect(page.locator('.error-state')).not.toBeVisible();
    });

    test('table renders the expected column headers', async ({ page }) => {
      await page.goto('/match-history');

      const table = page.locator('[data-testid="match-history-table"]');
      await expect(table).toBeVisible();

      // BffMatchHistory renders four columns: Date, Format, Opponent Deck, Result.
      await expect(table.locator('thead th').nth(0)).toHaveText('Date');
      await expect(table.locator('thead th').nth(1)).toHaveText('Format');
      await expect(table.locator('thead th').nth(2)).toHaveText('Opponent Deck');
      await expect(table.locator('thead th').nth(3)).toHaveText('Result');
    });

    test('pagination controls render when total exceeds the page size', async ({ page }) => {
      // 20 rows with total = 41 → more than one page → footer must render.
      await mockMatchHistory(page, MATCH_ROWS, 41);
      await page.goto('/match-history');

      await expect(page.locator('[data-testid="match-history-table"]')).toBeVisible();

      const prevBtn = page.locator('.pagination-btn', { hasText: 'Previous' });
      const nextBtn = page.locator('.pagination-btn', { hasText: 'Next' });
      const pageInfo = page.locator('.pagination-info');

      await expect(prevBtn).toBeVisible({ timeout: 5_000 });
      await expect(nextBtn).toBeVisible({ timeout: 5_000 });
      await expect(pageInfo).toContainText('Page');
    });
  });

  test.describe('Authenticated — empty', () => {
    test('empty state renders when the BFF returns no matches', async ({ page }) => {
      await setClerkSignedIn(page);
      await mockMatchHistory(page, [], 0);

      await page.goto('/match-history');

      await expect(page.locator('[data-testid="match-history-empty"]')).toBeVisible();
      await expect(page.locator('[data-testid="match-history-table"]')).not.toBeVisible();
    });
  });

  test.describe('Authenticated — API error', () => {
    test('error state is shown when the API returns an error', async ({ page }) => {
      await setClerkSignedIn(page);
      // Intercept the BFF match-history endpoint and return a 500 before load.
      await page.route('**/api/v1/history/matches**', (route) => {
        void route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'internal server error' }),
        });
      });

      await page.goto('/match-history');

      // After the failed request the error state div must be visible.
      await expect(page.locator('.error-state')).toBeVisible();

      // The match table and empty state must NOT appear alongside the error.
      await expect(page.locator('[data-testid="match-history-table"]')).not.toBeVisible();
      await expect(page.locator('[data-testid="match-history-empty"]')).not.toBeVisible();
    });
  });
});

// ---------------------------------------------------------------------------
// Draft history — /history/drafts (BffDraftHistory)
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

      await expect(page.locator('[data-testid="protected-route-prompt"]')).toBeVisible();
    });
  });

  test.describe('Authenticated — with data', () => {
    test.beforeEach(async ({ page }) => {
      await setClerkSignedIn(page);
      await mockDraftHistory(page, DRAFT_ROWS);
    });

    test('page loads without error and shows the draft table @smoke', async ({ page }) => {
      await page.goto('/history/drafts');

      await expect(page.locator('[data-testid="draft-history-table"]')).toBeVisible();
    });

    test('page title is "Draft History"', async ({ page }) => {
      await page.goto('/history/drafts');

      await expect(page.locator('[data-testid="draft-history-table"]')).toBeVisible();
      await expect(page.locator('h1.page-title')).toHaveText('Draft History');
    });

    test('no error state is shown on initial load', async ({ page }) => {
      await page.goto('/history/drafts');

      await expect(page.locator('[data-testid="draft-history-table"]')).toBeVisible();
      await expect(page.locator('.error-state')).not.toBeVisible();
    });

    test('table renders the expected column headers', async ({ page }) => {
      await page.goto('/history/drafts');

      const table = page.locator('[data-testid="draft-history-table"]');
      await expect(table).toBeVisible();

      // BffDraftHistory renders four columns: Date, Set, Wins, Losses.
      await expect(table.locator('thead th').nth(0)).toHaveText('Date');
      await expect(table.locator('thead th').nth(1)).toHaveText('Set');
      await expect(table.locator('thead th').nth(2)).toHaveText('Wins');
      await expect(table.locator('thead th').nth(3)).toHaveText('Losses');
    });

    test('pagination controls render when total exceeds the page size', async ({ page }) => {
      await mockDraftHistory(page, DRAFT_ROWS, 41);
      await page.goto('/history/drafts');

      await expect(page.locator('[data-testid="draft-history-table"]')).toBeVisible();

      const prevBtn = page.locator('.pagination-btn', { hasText: 'Previous' });
      const nextBtn = page.locator('.pagination-btn', { hasText: 'Next' });
      const pageInfo = page.locator('.pagination-info');

      await expect(prevBtn).toBeVisible({ timeout: 5_000 });
      await expect(nextBtn).toBeVisible({ timeout: 5_000 });
      await expect(pageInfo).toContainText('Page');
    });
  });

  test.describe('Authenticated — empty', () => {
    test('empty state renders when the BFF returns no drafts', async ({ page }) => {
      await setClerkSignedIn(page);
      await mockDraftHistory(page, [], 0);

      await page.goto('/history/drafts');

      await expect(page.locator('[data-testid="draft-history-empty"]')).toBeVisible();
      await expect(page.locator('[data-testid="draft-history-table"]')).not.toBeVisible();
    });
  });

  test.describe('Authenticated — API error', () => {
    test('error state is shown when the API returns an error', async ({ page }) => {
      await setClerkSignedIn(page);
      await page.route('**/api/v1/history/drafts**', (route) => {
        void route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'internal server error' }),
        });
      });

      await page.goto('/history/drafts');

      await expect(page.locator('.error-state')).toBeVisible();

      await expect(page.locator('[data-testid="draft-history-table"]')).not.toBeVisible();
      await expect(page.locator('[data-testid="draft-history-empty"]')).not.toBeVisible();
    });
  });
});
