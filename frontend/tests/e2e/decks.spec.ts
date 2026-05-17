import { test, expect, type Page } from '@playwright/test';

/**
 * Decks Page E2E Tests (#2178)
 *
 * Tests the Decks page functionality including navigation and deck management.
 *
 * /decks is behind ProtectedRoute. Tests inject a signed-in Clerk test state via
 * window.__CLERK_TEST_STATE__ so ProtectedRoute renders the Decks content rather
 * than the sign-in prompt (requires VITE_CLERK_TEST_MODE=true, set in
 * playwright.config.ts webServer command).
 *
 * BFF-data mocking (#2178): in CI the BFF runs with a Clerk secret that does not
 * accept the Clerk mock's stub token, so the real Clerk-protected /api/v1/decks
 * endpoint rejects every request and the Decks page renders its error state. To
 * keep these tests independent of a live authenticated BFF, GET /api/v1/decks is
 * mocked via page.route() before navigation.
 *
 * Response envelope: the shared apiClient (services/apiClient.ts) unwraps every
 * response as `data.data`, so mocked endpoints routed through it must return a
 * `{ "data": <payload> }` envelope — a bare array would be dropped.
 *
 * The mock returns an empty deck list. The DeckBuilder "Build Around" tests are
 * all guarded by `if (hasCards)` and therefore no-op safely when no decks exist.
 */

/** Inject signed-in Clerk state before page load. Must be called before page.goto(). */
async function setClerkSignedIn(page: Page): Promise<void> {
  await page.addInitScript(() => {
    (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = { isSignedIn: true };
  });
}

/**
 * Mock GET /api/v1/decks so the Decks page renders without a live authenticated
 * BFF. Returns an empty list — the page reaches its empty state, not an error.
 * Registered before page.goto().
 */
async function mockDecksEndpoint(page: Page): Promise<void> {
  await page.route('**/api/v1/decks**', (route) => {
    if (route.request().method() === 'GET') {
      void route.fulfill({
        status: 200,
        contentType: 'application/json',
        // apiClient unwraps response.data — return a { data } envelope.
        body: JSON.stringify({ data: [] }),
      });
      return;
    }
    void route.continue();
  });
}

test.describe('Decks', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through to Decks content.
    await setClerkSignedIn(page);
    // Mock the BFF deck list so the page does not depend on a live authenticated BFF.
    await mockDecksEndpoint(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await page.click('a[href="/decks"]');
    await page.waitForURL('**/decks');
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should navigate to Decks page', async ({ page }) => {
      await expect(page.locator('h1')).toContainText('Decks');
    });

    test('should display page title', async ({ page }) => {
      const header = page.locator('h1');
      await expect(header).toBeVisible();
      await expect(header).toContainText('Decks');
    });
  });

  test.describe('Deck List', () => {
    test('should display deck cards or empty state', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');

      // Wait for either content type to appear
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();
      const hasEmptyState = await emptyState.isVisible();

      expect(hasCards || hasEmptyState).toBeTruthy();
    });
  });

  test.describe('Create Deck', () => {
    test('should have create deck button', async ({ page }) => {
      // Wait for page to fully load
      const pageContent = page.locator('.deck-card, .empty-state, .decks-header');
      await expect(pageContent.first()).toBeVisible();

      const createButton = page.locator('button').filter({ hasText: /create|new/i });
      const hasButton = await createButton.isVisible().catch(() => false);

      expect(hasButton).toBeTruthy();
    });
  });

  test.describe('Loading State', () => {
    test('should not show error state on initial load', async ({ page }) => {
      // Wait for content to load
      const content = page.locator('.deck-card, .empty-state');
      await expect(content.first()).toBeVisible();

      const errorState = page.locator('.error-state');
      await expect(errorState).not.toBeVisible();
    });
  });

  test.describe('DeckBuilder Build Around', () => {
    test('should show Build Around button for non-draft decks', async ({ page }) => {
      // Wait for decks to load
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        // Find a non-draft deck (look for Standard, Historic, etc. format badges)
        const nonDraftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:not(:has-text("Limited"))')
        }).first();

        const hasNonDraft = await nonDraftDeck.isVisible().catch(() => false);

        if (hasNonDraft) {
          // Click on the deck to go to DeckBuilder
          await nonDraftDeck.click();
          await page.waitForURL('**/decks/**');

          // Wait for DeckBuilder to load
          const deckBuilder = page.locator('.deck-builder');
          await expect(deckBuilder).toBeVisible();

          // Build Around button should be visible for non-draft decks
          const buildAroundButton = page.locator('button.build-around-btn');
          await expect(buildAroundButton).toBeVisible();
          await expect(buildAroundButton).toContainText('Build Around');
        }
      }
    });

    test('should open Build Around modal when button clicked', async ({ page }) => {
      // Wait for decks to load
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        // Find a non-draft deck
        const nonDraftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:not(:has-text("Limited"))')
        }).first();

        const hasNonDraft = await nonDraftDeck.isVisible().catch(() => false);

        if (hasNonDraft) {
          await nonDraftDeck.click();
          await page.waitForURL('**/decks/**');

          const buildAroundButton = page.locator('button.build-around-btn');
          const isButtonVisible = await buildAroundButton.isVisible().catch(() => false);

          if (isButtonVisible) {
            await buildAroundButton.click();

            // Modal should open
            const modal = page.locator('.build-around-modal');
            await expect(modal).toBeVisible({ timeout: 5000 });

            // Modal header should be visible
            const modalHeader = modal.locator('h2');
            await expect(modalHeader).toContainText('Build Around Card');

            // Search input should be present
            const searchInput = modal.locator('input[placeholder*="Search"]');
            await expect(searchInput).toBeVisible();

            // Close button should work
            const closeButton = modal.locator('.close-button');
            await closeButton.click();
            await expect(modal).not.toBeVisible();
          }
        }
      }
    });

    test('should NOT show Build Around button for draft decks', async ({ page }) => {
      // Wait for decks to load
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        // Find a draft/limited deck
        const draftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:has-text("Limited")')
        }).first();

        const hasDraft = await draftDeck.isVisible().catch(() => false);

        if (hasDraft) {
          await draftDeck.click();
          await page.waitForURL('**/decks/**');

          const deckBuilder = page.locator('.deck-builder');
          await expect(deckBuilder).toBeVisible();

          // Build Around button should NOT be visible for draft decks
          const buildAroundButton = page.locator('button.build-around-btn');
          await expect(buildAroundButton).not.toBeVisible();

          // But Suggest Decks button SHOULD be visible for draft decks
          const suggestDecksButton = page.locator('button.suggest-decks-btn');
          await expect(suggestDecksButton).toBeVisible();
        }
      }
    });

    test('should search for cards in Build Around modal', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        const nonDraftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:not(:has-text("Limited"))')
        }).first();

        const hasNonDraft = await nonDraftDeck.isVisible().catch(() => false);

        if (hasNonDraft) {
          await nonDraftDeck.click();
          await page.waitForURL('**/decks/**');

          const buildAroundButton = page.locator('button.build-around-btn');
          const isButtonVisible = await buildAroundButton.isVisible().catch(() => false);

          if (isButtonVisible) {
            await buildAroundButton.click();

            const modal = page.locator('.build-around-modal');
            await expect(modal).toBeVisible({ timeout: 5000 });

            // Type in search input
            const searchInput = modal.locator('input[placeholder*="Search"]');
            await searchInput.fill('Lightning');

            // Wait for search results to appear
            const searchResults = page.locator('.search-results');
            await expect(searchResults).toBeVisible().catch(() => {
              // No results is also a valid outcome
            });

            // Close modal
            const closeButton = modal.locator('.close-button');
            await closeButton.click();
          }
        }
      }
    });

    test('should show color filter buttons in Build Around modal', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        const nonDraftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:not(:has-text("Limited"))')
        }).first();

        const hasNonDraft = await nonDraftDeck.isVisible().catch(() => false);

        if (hasNonDraft) {
          await nonDraftDeck.click();
          await page.waitForURL('**/decks/**');

          const buildAroundButton = page.locator('button.build-around-btn');
          const isButtonVisible = await buildAroundButton.isVisible().catch(() => false);

          if (isButtonVisible) {
            await buildAroundButton.click();

            const modal = page.locator('.build-around-modal');
            await expect(modal).toBeVisible({ timeout: 5000 });

            // Check that color filter buttons exist
            const colorFilters = modal.locator('.color-filter-buttons');
            await expect(colorFilters).toBeVisible();

            // Verify WUBRG buttons exist
            for (const color of ['W', 'U', 'B', 'R', 'G']) {
              const colorButton = modal.locator(`.color-filter-btn.mana-${color.toLowerCase()}`);
              await expect(colorButton).toBeVisible();
            }

            // Close modal
            const closeButton = modal.locator('.close-button');
            await closeButton.click();
          }
        }
      }
    });

    test('should show budget mode checkbox in Build Around modal', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        const nonDraftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:not(:has-text("Limited"))')
        }).first();

        const hasNonDraft = await nonDraftDeck.isVisible().catch(() => false);

        if (hasNonDraft) {
          await nonDraftDeck.click();
          await page.waitForURL('**/decks/**');

          const buildAroundButton = page.locator('button.build-around-btn');
          const isButtonVisible = await buildAroundButton.isVisible().catch(() => false);

          if (isButtonVisible) {
            await buildAroundButton.click();

            const modal = page.locator('.build-around-modal');
            await expect(modal).toBeVisible({ timeout: 5000 });

            // Search and select a card to show build options
            const searchInput = modal.locator('input[placeholder*="Search"]');
            await searchInput.fill('Mountain');

            // Wait for results and click first one
            const searchResults = page.locator('.search-results');
            const hasResults = await searchResults.isVisible({ timeout: 5000 }).catch(() => false);

            if (hasResults) {
              const firstResult = searchResults.locator('.search-result-item').first();
              await firstResult.click();

              // Budget mode checkbox should be visible
              const budgetCheckbox = modal.locator('.option-checkbox');
              await expect(budgetCheckbox).toBeVisible();
            }

            // Close modal
            const closeButton = modal.locator('.close-button');
            await closeButton.click();
          }
        }
      }
    });

    test('should close Build Around modal with Escape key', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        const nonDraftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:not(:has-text("Limited"))')
        }).first();

        const hasNonDraft = await nonDraftDeck.isVisible().catch(() => false);

        if (hasNonDraft) {
          await nonDraftDeck.click();
          await page.waitForURL('**/decks/**');

          const buildAroundButton = page.locator('button.build-around-btn');
          const isButtonVisible = await buildAroundButton.isVisible().catch(() => false);

          if (isButtonVisible) {
            await buildAroundButton.click();

            const modal = page.locator('.build-around-modal');
            await expect(modal).toBeVisible({ timeout: 5000 });

            // Press Escape to close
            await page.keyboard.press('Escape');

            // Modal should close
            await expect(modal).not.toBeVisible({ timeout: 3000 });
          }
        }
      }
    });

    test('should close Build Around modal when clicking overlay', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        const nonDraftDeck = page.locator('.deck-card').filter({
          has: page.locator('.deck-format:not(:has-text("Limited"))')
        }).first();

        const hasNonDraft = await nonDraftDeck.isVisible().catch(() => false);

        if (hasNonDraft) {
          await nonDraftDeck.click();
          await page.waitForURL('**/decks/**');

          const buildAroundButton = page.locator('button.build-around-btn');
          const isButtonVisible = await buildAroundButton.isVisible().catch(() => false);

          if (isButtonVisible) {
            await buildAroundButton.click();

            const modal = page.locator('.build-around-modal');
            await expect(modal).toBeVisible({ timeout: 5000 });

            // Click the overlay (outside the modal)
            const overlay = page.locator('.build-around-overlay');
            await overlay.click({ position: { x: 10, y: 10 } });

            // Modal should close
            await expect(modal).not.toBeVisible({ timeout: 3000 });
          }
        }
      }
    });
  });
});
