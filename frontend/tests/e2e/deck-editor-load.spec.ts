import { test, expect } from '@playwright/test';

/**
 * Deck Editor Load E2E Tests — Issue #2009
 *
 * Regression tests covering the bug where clicking to edit any existing deck
 * triggered the error boundary with "Invalid deck data".
 *
 * Root cause: The BFF `GET /decks/:id` returns a flat camelCase structure
 * (e.g. { id, name, cards: [{cardId, quantity, ...}] }) but the frontend
 * DeckWithCards type expected a nested { deck: {...}, cards: [...] } shape
 * with PascalCase card fields.  The fix adds a mapping layer in
 * `services/api/decks.ts` (mapBffDeckDetail) that normalises the response.
 *
 * AC3: No "Invalid deck data" text must appear in the DOM when navigating to
 * edit an existing deck.
 */
test.describe('Deck Editor Load (#2009)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();
  });

  test('@smoke should not show "Invalid deck data" when opening any deck', async ({ page }) => {
    // Navigate to the Decks list page.
    await page.click('a[href="/decks"]');
    await page.waitForURL('**/decks');

    // Wait for the deck list to settle (either cards or empty state).
    const deckCard = page.locator('.deck-card');
    const emptyState = page.locator('.empty-state');
    await expect(deckCard.first().or(emptyState)).toBeVisible();

    const hasDecks = await deckCard.first().isVisible();

    if (!hasDecks) {
      // No decks exist in the test environment — test is not applicable.
      // AC3 cannot be violated when there are no decks to open.
      test.skip();
      return;
    }

    // Click the first available deck to open it in the editor.
    await deckCard.first().click();
    await page.waitForURL('**/decks/**', { timeout: 10_000 });

    // Allow time for the deck to load (loading spinner then content or error).
    const deckBuilder = page.locator('.deck-builder');
    await expect(deckBuilder).toBeVisible({ timeout: 15_000 });

    // AC3: The text "Invalid deck data" must never appear in the DOM.
    await expect(page.locator('text=Invalid deck data')).not.toBeVisible();

    // The error boundary "Error Loading Deck" heading must not be present.
    await expect(page.locator('h2:has-text("Error Loading Deck")')).not.toBeVisible();
  });

  test('should load deck editor without error state for first deck', async ({ page }) => {
    // Navigate to Decks page.
    await page.click('a[href="/decks"]');
    await page.waitForURL('**/decks');

    const deckCard = page.locator('.deck-card');
    const emptyState = page.locator('.empty-state');
    await expect(deckCard.first().or(emptyState)).toBeVisible();

    const hasDecks = await deckCard.first().isVisible();

    if (!hasDecks) {
      test.skip();
      return;
    }

    // Open the first deck.
    await deckCard.first().click();
    await page.waitForURL('**/decks/**', { timeout: 10_000 });

    // Wait for deck builder container to appear.
    const deckBuilder = page.locator('.deck-builder');
    await expect(deckBuilder).toBeVisible({ timeout: 15_000 });

    // Verify the deck builder rendered the editor (not the error state).
    // The error state has class "error-state"; the loaded state does not.
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
    const emptyState = page.locator('.empty-state');
    await expect(deckCard.first().or(emptyState)).toBeVisible();

    const hasDecks = await deckCard.first().isVisible();

    if (!hasDecks) {
      test.skip();
      return;
    }

    // Click first deck to discover the URL pattern.
    await deckCard.first().click();
    await page.waitForURL('**/decks/**', { timeout: 10_000 });

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
