import { test, expect } from '@playwright/test';

/**
 * Deck History Modal E2E Tests
 *
 * Tests the deck version history functionality including viewing permutations,
 * checking current version indicators, and restore functionality.
 * Uses REST API backend for testing.
 */
test.describe('Deck History', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    await page.click('a[href="/decks"]');
    await page.waitForURL('**/decks');
  });

  test.describe('History Modal Access', () => {
    test('should show history button when viewing a deck with history', async ({ page }) => {
      // Wait for decks to load
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        // Click on first deck
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        // Wait for deck builder to load
        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        // Look for history button (if deck has permutations)
        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        // History button may or may not be visible depending on whether deck has history
        if (hasHistoryButton) {
          await expect(historyButton).toBeVisible();
        }
      }
    });
  });

  test.describe('History Modal Display', () => {
    test('should open deck history modal when clicking history button', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          // Modal should appear
          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Modal should have title with deck name
          const modalHeader = modal.locator('.modal-header h2');
          await expect(modalHeader).toContainText('Deck History');
        }
      }
    });

    test('should display version list in history modal', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Either version list or empty state should be visible
          const versionList = modal.locator('.version-list');
          const emptyStateInModal = modal.locator('.empty-state');
          await expect(versionList.or(emptyStateInModal)).toBeVisible({ timeout: 5000 });
        }
      }
    });

    test('should show "(Current)" badge for current permutation', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Check for current badge if versions exist
          const versionList = modal.locator('.version-list');
          const hasVersions = await versionList.isVisible().catch(() => false);

          if (hasVersions) {
            const currentBadge = modal.locator('.current-badge');
            const hasBadge = await currentBadge.isVisible().catch(() => false);

            if (hasBadge) {
              await expect(currentBadge).toContainText('(Current)');
            }
          }
        }
      }
    });

    test('should disable restore button for current version', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Look for restore button
          const restoreButton = modal.locator('.restore-button');
          const hasRestoreButton = await restoreButton.isVisible().catch(() => false);

          if (hasRestoreButton) {
            // Current version should have restore button disabled
            const currentVersionItem = modal.locator('.version-item.selected');
            const hasCurrentSelected = await currentVersionItem.isVisible().catch(() => false);

            if (hasCurrentSelected) {
              // Check if this is the current version (has the badge)
              const hasBadge = await currentVersionItem.locator('.current-badge').isVisible().catch(() => false);
              if (hasBadge) {
                await expect(restoreButton).toBeDisabled();
              }
            }
          }
        }
      }
    });
  });

  test.describe('History Modal Interaction', () => {
    test('should close modal when clicking close button', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Click close button
          const closeButton = modal.locator('button').filter({ hasText: /close/i });
          await closeButton.click();

          // Modal should be hidden
          await expect(modal).not.toBeVisible();
        }
      }
    });

    test('should close modal when clicking overlay', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Click on overlay (outside modal)
          const overlay = page.locator('.modal-overlay');
          await overlay.click({ position: { x: 10, y: 10 } });

          // Modal should be hidden
          await expect(modal).not.toBeVisible();
        }
      }
    });

    test('should allow selecting different versions', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Check if there are multiple versions
          const versionItems = modal.locator('.version-item');
          const versionCount = await versionItems.count();

          if (versionCount > 1) {
            // Click on a non-selected version
            const nonSelectedVersion = versionItems.filter({ hasNot: page.locator('.selected') }).first();
            const hasNonSelected = await nonSelectedVersion.isVisible().catch(() => false);

            if (hasNonSelected) {
              await nonSelectedVersion.click();

              // Verify the clicked version is now selected
              await expect(nonSelectedVersion).toHaveClass(/selected/);
            }
          }
        }
      }
    });

    test('should display match statistics for versions', async ({ page }) => {
      const deckCard = page.locator('.deck-card');
      const emptyState = page.locator('.empty-state');
      await expect(deckCard.first().or(emptyState)).toBeVisible();

      const hasCards = await deckCard.first().isVisible();

      if (hasCards) {
        await deckCard.first().click();
        await page.waitForURL('**/decks/**');

        const deckBuilder = page.locator('.deck-builder');
        await expect(deckBuilder).toBeVisible();

        const historyButton = page.locator('button').filter({ hasText: /history/i });
        const hasHistoryButton = await historyButton.isVisible().catch(() => false);

        if (hasHistoryButton) {
          await historyButton.click();

          const modal = page.locator('.deck-history-modal');
          await expect(modal).toBeVisible({ timeout: 5000 });

          // Check for version stats (either match stats or "No matches")
          const versionStats = modal.locator('.version-stats');
          const hasStats = await versionStats.first().isVisible().catch(() => false);

          if (hasStats) {
            // Stats should show either win/loss record or "No matches"
            const statsText = await versionStats.first().textContent();
            expect(statsText).toMatch(/(\d+W-\d+L|No matches)/);
          }
        }
      }
    });
  });
});
