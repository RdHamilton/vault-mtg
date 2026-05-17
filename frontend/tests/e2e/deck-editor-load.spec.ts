import { test, expect, type Page } from '@playwright/test';

/**
 * Deck Editor Load E2E Tests — Issue #2009, #2178
 *
 * Regression tests covering the bug where clicking to edit any existing deck
 * triggered the error boundary with "Invalid deck data".
 *
 * Root cause of the original bug: the BFF `GET /decks/:id` returns a flat
 * camelCase structure (e.g. { id, name, cards: [{cardId, quantity, ...}] }) but
 * the frontend DeckWithCards type expected a nested { deck: {...}, cards: [...] }
 * shape with PascalCase card fields. The fix added a mapping layer in
 * `services/api/decks.ts` (mapBffDeckDetail) that normalises the response.
 *
 * AC3: No "Invalid deck data" text must appear in the DOM when navigating to
 * edit an existing deck.
 *
 * Auth + BFF mocking (#2178): /decks is behind ProtectedRoute, so a signed-in
 * Clerk test state is injected via window.__CLERK_TEST_STATE__. In CI the BFF
 * runs with a Clerk secret that does not accept the Clerk mock's stub token, so
 * the real /api/v1/decks* endpoints reject every request. The deck list and
 * deck detail endpoints are mocked via page.route() before navigation so this
 * regression test exercises the mapBffDeckDetail path deterministically without
 * a live authenticated BFF — the BFF detail response shape is reproduced
 * exactly so the mapping layer is genuinely covered.
 */

const DECK_ID = 'deck-2009-fixture';

/** Inject signed-in Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedIn(page: Page): Promise<void> {
  await page.addInitScript(() => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = { isSignedIn: true };
  });
}

/**
 * Mock the BFF deck endpoints:
 *  - GET /api/v1/decks      → a one-deck list summary
 *  - GET /api/v1/decks/:id  → the flat camelCase deck-detail envelope the BFF
 *                             actually returns (drives mapBffDeckDetail)
 * Registered before page.goto().
 */
async function mockDeckEndpoints(page: Page): Promise<void> {
  // List summary row — gui.DeckListItem shape.
  const listRow = {
    id: DECK_ID,
    name: 'Mono-Red Aggro',
    format: 'standard',
    source: 'manual',
    colorIdentity: 'R',
    cardCount: 1,
    matchesPlayed: 0,
    matchWinRate: 0,
    modifiedAt: '2026-05-01T12:00:00Z',
    currentStreak: 0,
  };

  // Flat camelCase detail envelope — exactly the BffDeckDetailRaw shape.
  const detail = {
    id: DECK_ID,
    name: 'Mono-Red Aggro',
    format: 'standard',
    source: 'manual',
    draftEventId: null,
    matchesPlayed: 0,
    matchesWon: 0,
    gamesPlayed: 0,
    gamesWon: 0,
    winRate: 0,
    isAppCreated: true,
    createdAt: '2026-05-01T12:00:00Z',
    modifiedAt: '2026-05-01T12:00:00Z',
    lastPlayed: null,
    colorIdentity: 'R',
    description: '',
    cardCount: 1,
    tags: [],
    cards: [
      {
        cardId: 12345,
        quantity: 4,
        board: 'main',
        fromDraftPick: false,
        name: 'Lightning Strike',
        setCode: 'DMU',
        manaCost: '{1}{R}',
        cmc: 2,
        typeLine: 'Instant',
        rarity: 'common',
        imageUri: '',
        colors: ['R'],
      },
    ],
  };

  // apiClient unwraps response.data — every body below is a { data } envelope.
  await page.route('**/api/v1/decks', (route) => {
    if (route.request().method() === 'GET') {
      void route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [listRow] }),
      });
      return;
    }
    void route.continue();
  });

  await page.route(`**/api/v1/decks/${DECK_ID}`, (route) => {
    if (route.request().method() === 'GET') {
      void route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: detail }),
      });
      return;
    }
    void route.continue();
  });

  // Deck stats / curve / colors etc. — return an empty { data } envelope so any
  // auxiliary fetch the editor fires resolves instead of erroring against the
  // live BFF.
  await page.route(`**/api/v1/decks/${DECK_ID}/**`, (route) => {
    void route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: {} }),
    });
  });
}

test.describe('Deck Editor Load (#2009)', () => {
  test.beforeEach(async ({ page }) => {
    await setClerkSignedIn(page);
    await mockDeckEndpoints(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('@smoke should not show "Invalid deck data" when opening any deck', async ({ page }) => {
    // Navigate to the Decks list page.
    await page.click('a[href="/decks"]');
    await page.waitForURL('**/decks');

    // The mocked deck list provides exactly one deck.
    const deckCard = page.locator('.deck-card');
    await expect(deckCard.first()).toBeVisible();

    // Click the first deck to open it in the editor.
    await deckCard.first().click();
    await page.waitForURL('**/deck-builder/**', { timeout: 10_000 });

    // Allow time for the deck to load (loading spinner then content or error).
    const deckBuilder = page.locator('.deck-builder');
    await expect(deckBuilder).toBeVisible({ timeout: 15_000 });

    // AC3: The text "Invalid deck data" must never appear in the DOM.
    await expect(page.locator('text=Invalid deck data')).not.toBeVisible();

    // The error boundary "Error Loading Deck" heading must not be present.
    await expect(page.locator('h2:has-text("Error Loading Deck")')).not.toBeVisible();
  });

  test('should load deck editor without error state for the first deck', async ({ page }) => {
    // Navigate to Decks page.
    await page.click('a[href="/decks"]');
    await page.waitForURL('**/decks');

    const deckCard = page.locator('.deck-card');
    await expect(deckCard.first()).toBeVisible();

    // Open the first deck.
    await deckCard.first().click();
    await page.waitForURL('**/deck-builder/**', { timeout: 10_000 });

    // Wait for deck builder container to appear.
    const deckBuilder = page.locator('.deck-builder');
    await expect(deckBuilder).toBeVisible({ timeout: 15_000 });

    // Verify the deck builder rendered the editor (not the error state).
    const errorState = deckBuilder.locator('.error-state');
    const hasError = await errorState.isVisible().catch(() => false);

    // "Invalid deck data" is the exact text from the DeckBuilder error path.
    const pageText = await page.content();
    expect(pageText).not.toContain('Invalid deck data');
    expect(hasError).toBe(false);
  });

  test('should render Deck Builder header when navigating to /deck-builder/:id', async ({ page }) => {
    // Navigate to the Decks list first to discover a real deck ID.
    await page.click('a[href="/decks"]');
    await page.waitForURL('**/decks');

    const deckCard = page.locator('.deck-card');
    await expect(deckCard.first()).toBeVisible();

    // Click the first deck to discover the URL pattern.
    await deckCard.first().click();
    await page.waitForURL('**/deck-builder/**', { timeout: 10_000 });

    const currentURL = page.url();

    // Navigate directly to the discovered URL (simulates deep-link / page refresh).
    await page.goto(currentURL);

    // Deck Builder container must be present.
    const deckBuilder = page.locator('.deck-builder');
    await expect(deckBuilder).toBeVisible({ timeout: 20_000 });

    // "Invalid deck data" must not appear anywhere in the rendered output.
    await expect(page.locator('text=Invalid deck data')).not.toBeVisible();

    // The Deck Builder header block should render.
    const header = page.locator('.deck-builder-header');
    await expect(header).toBeVisible();
  });
});
