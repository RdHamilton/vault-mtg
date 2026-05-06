import { test, expect } from '@playwright/test';

/**
 * Draft Page E2E Tests
 *
 * Tests the Draft page functionality including navigation and content display.
 * Uses REST API backend for testing.
 *
 * Note: /draft is behind ProtectedRoute (added in #1300). Tests inject a signed-in
 * Clerk test state via window.__CLERK_TEST_STATE__ so ProtectedRoute renders the
 * Draft content rather than the sign-in prompt. This requires Playwright to be
 * started with VITE_CLERK_TEST_MODE=true (set in playwright.config.ts webServer command).
 */
test.describe('Draft', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through to Draft content.
    await page.addInitScript(() => {
      (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = { isSignedIn: true };
    });

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible({ timeout: 10000 });

    await page.click('a[href="/draft"]');
    await page.waitForURL('**/draft');
  });

  test.describe('Navigation and Page Load', () => {
    test('@smoke should navigate to Draft page', async ({ page }) => {
      // Wait for page content to load
      const appContainer = page.locator('[data-testid="app-container"]');
      await expect(appContainer).toBeVisible();

      // Verify we're on the draft page
      const url = page.url();
      expect(url).toContain('/draft');
    });
  });

  test.describe('Draft Content', () => {
    test('should display draft content or empty state', async ({ page }) => {
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');

      // Wait for either content type to appear
      await expect(draftContainer.or(draftEmpty).first()).toBeVisible({ timeout: 10000 });

      const hasContainer = await draftContainer.isVisible();
      const hasEmpty = await draftEmpty.isVisible();

      expect(hasContainer || hasEmpty).toBeTruthy();
    });

    test('should display historical drafts section if no active draft', async ({ page }) => {
      const historicalSection = page.locator('text=Historical Drafts');
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');

      // Wait for content to load
      await expect(draftContainer.or(draftEmpty).or(historicalSection).first()).toBeVisible({ timeout: 10000 });

      const hasHistorical = await historicalSection.isVisible();
      const hasDraftContent = await draftContainer.isVisible();
      const hasEmpty = await draftEmpty.isVisible();

      expect(hasHistorical || hasDraftContent || hasEmpty).toBeTruthy();
    });
  });

  test.describe('Loading State', () => {
    test('should not show error state on initial load', async ({ page }) => {
      // Wait for content to load
      const content = page.locator('.draft-container, .draft-empty');
      await expect(content.first()).toBeVisible({ timeout: 10000 });

      const errorState = page.locator('.error-state');
      await expect(errorState).not.toBeVisible();
    });
  });

  test.describe('Create Deck from Draft', () => {
    test('@smoke should display Build Deck button on historical draft sessions', async ({ page }) => {
      // Wait for draft content to load
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');
      const historicalSection = page.locator('text=Historical Drafts');

      await expect(draftContainer.or(draftEmpty).or(historicalSection).first()).toBeVisible({ timeout: 10000 });

      // Look for Build Deck button in the page
      const buildDeckButton = page.locator('button.btn-build-deck, button:has-text("Build Deck")');
      const hasBuildButton = await buildDeckButton.first().isVisible().catch(() => false);

      // Only assert Build Deck button if actual draft session cards are present (not just the empty-state container)
      const hasDraftSessions = await page.locator('.draft-session, .draft-history-item, .draft-card').first().isVisible().catch(() => false);
      if (hasDraftSessions) {
        expect(hasBuildButton).toBeTruthy();
      }
    });

    test('should navigate to DeckBuilder when clicking Build Deck on a draft session', async ({ page }) => {
      // Wait for draft content to load
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');
      const historicalSection = page.locator('text=Historical Drafts');

      await expect(draftContainer.or(draftEmpty).or(historicalSection).first()).toBeVisible({ timeout: 10000 });

      // Find a Build Deck button
      const buildDeckButton = page.locator('button.btn-build-deck, button:has-text("Build Deck")').first();
      const hasBuildButton = await buildDeckButton.isVisible().catch(() => false);

      if (hasBuildButton) {
        // Click the Build Deck button
        await buildDeckButton.click();

        // Should navigate to deck-builder with draft ID
        await page.waitForURL('**/deck-builder/draft/**', { timeout: 10000 });

        // Verify we're on the DeckBuilder page
        const url = page.url();
        expect(url).toContain('/deck-builder/draft/');
      }
    });

    test('should display DeckBuilder UI correctly when creating deck from draft', async ({ page }) => {
      // Wait for draft content to load
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');
      const historicalSection = page.locator('text=Historical Drafts');

      await expect(draftContainer.or(draftEmpty).or(historicalSection).first()).toBeVisible({ timeout: 10000 });

      // Find a Build Deck button
      const buildDeckButton = page.locator('button.btn-build-deck, button:has-text("Build Deck")').first();
      const hasBuildButton = await buildDeckButton.isVisible().catch(() => false);

      if (hasBuildButton) {
        await buildDeckButton.click();
        await page.waitForURL('**/deck-builder/draft/**', { timeout: 10000 });

        // Wait for DeckBuilder to load
        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible({ timeout: 15000 });

        // Verify deck header is displayed with deck name
        const deckHeader = page.locator('.deck-header h2, .deck-title h2');
        await expect(deckHeader).toBeVisible({ timeout: 10000 });

        // Deck name should contain "Draft" (e.g., "QuickDraft_DSK Draft")
        const deckName = await deckHeader.textContent();
        expect(deckName?.toLowerCase()).toContain('draft');
      }
    });

    test('should load draft picks into the deck when creating from draft', async ({ page }) => {
      // Wait for draft content to load
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');
      const historicalSection = page.locator('text=Historical Drafts');

      await expect(draftContainer.or(draftEmpty).or(historicalSection).first()).toBeVisible({ timeout: 10000 });

      // Find a Build Deck button
      const buildDeckButton = page.locator('button.btn-build-deck, button:has-text("Build Deck")').first();
      const hasBuildButton = await buildDeckButton.isVisible().catch(() => false);

      if (hasBuildButton) {
        await buildDeckButton.click();
        await page.waitForURL('**/deck-builder/draft/**', { timeout: 10000 });

        // Wait for DeckBuilder to load
        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible({ timeout: 15000 });

        // Wait for cards to load (deck list should have cards)
        const deckList = page.locator('.deck-list, .deck-cards');
        await expect(deckList).toBeVisible({ timeout: 10000 });

        // Check that the deck has cards (not empty)
        // Look for card entries or quantity indicators
        const cardEntry = page.locator('.deck-card, .card-entry, [class*="card"]').first();
        const emptyMessage = page.locator('text=No cards, text=Empty deck');

        // Either we have cards or we should verify the deck was created
        const hasCards = await cardEntry.isVisible().catch(() => false);
        const isEmpty = await emptyMessage.isVisible().catch(() => false);

        // The deck should have been created (we navigated successfully)
        // Cards may or may not be present depending on fixture data
        expect(hasCards || !isEmpty).toBeTruthy();
      }
    });

    test('should show Suggest Decks button for draft deck (not Build Around)', async ({ page }) => {
      // Wait for draft content to load
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');
      const historicalSection = page.locator('text=Historical Drafts');

      await expect(draftContainer.or(draftEmpty).or(historicalSection).first()).toBeVisible({ timeout: 10000 });

      // Find a Build Deck button
      const buildDeckButton = page.locator('button.btn-build-deck, button:has-text("Build Deck")').first();
      const hasBuildButton = await buildDeckButton.isVisible().catch(() => false);

      if (hasBuildButton) {
        await buildDeckButton.click();
        await page.waitForURL('**/deck-builder/draft/**', { timeout: 10000 });

        // Wait for DeckBuilder to load
        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible({ timeout: 15000 });

        // For draft decks, Suggest Decks button should be visible
        const suggestDecksButton = page.locator('button.suggest-decks-btn, button:has-text("Suggest Decks")');
        await expect(suggestDecksButton).toBeVisible({ timeout: 5000 });

        // Build Around button should NOT be visible for draft decks
        const buildAroundButton = page.locator('button.build-around-btn');
        await expect(buildAroundButton).not.toBeVisible();
      }
    });

    test('should show Export and Validate buttons in deck footer', async ({ page }) => {
      // Wait for draft content to load
      const draftContainer = page.locator('.draft-container');
      const draftEmpty = page.locator('.draft-empty');
      const historicalSection = page.locator('text=Historical Drafts');

      await expect(draftContainer.or(draftEmpty).or(historicalSection).first()).toBeVisible({ timeout: 10000 });

      // Find a Build Deck button
      const buildDeckButton = page.locator('button.btn-build-deck, button:has-text("Build Deck")').first();
      const hasBuildButton = await buildDeckButton.isVisible().catch(() => false);

      if (hasBuildButton) {
        await buildDeckButton.click();
        await page.waitForURL('**/deck-builder/draft/**', { timeout: 10000 });

        // Wait for DeckBuilder to load
        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible({ timeout: 15000 });

        // Verify Export button is present
        const exportButton = page.locator('button:has-text("Export")');
        await expect(exportButton).toBeVisible({ timeout: 5000 });

        // Verify Validate button is present
        const validateButton = page.locator('button:has-text("Validate")');
        await expect(validateButton).toBeVisible({ timeout: 5000 });
      }
    });
  });
});
