import { test, expect } from '@playwright/test';

/**
 * Draft Analytics Page E2E Tests
 *
 * Tests the Draft Analytics page functionality including navigation,
 * sub-tabs, filters, and analytics component display.
 * Uses REST API backend for testing.
 */
test.describe('Draft Analytics', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Navigate to Draft tab first
    await page.click('a[href="/draft"]');
    await page.waitForURL('**/draft');
  });

  test.describe('Sub-navigation', () => {
    test('@smoke should display Draft sub-navigation bar', async ({ page }) => {
      const subTabBar = page.locator('.sub-tab-bar');
      await expect(subTabBar).toBeVisible();
    });

    test('should have Current Draft and Analytics sub-tabs', async ({ page }) => {
      const subTabBar = page.locator('.sub-tab-bar');
      await expect(subTabBar).toBeVisible();

      await expect(subTabBar.locator('a[href="/draft"]')).toBeVisible();
      await expect(subTabBar.locator('a[href="/draft-analytics"]')).toBeVisible();
    });

    test('should have Current Draft active by default', async ({ page }) => {
      const subTabBar = page.locator('.sub-tab-bar');
      await expect(subTabBar).toBeVisible();

      const activeSubTab = subTabBar.locator('a.active');
      await expect(activeSubTab).toContainText(/Current Draft/i);
    });

    test('@smoke should navigate to Analytics via sub-nav', async ({ page }) => {
      const subTabBar = page.locator('.sub-tab-bar');
      await expect(subTabBar).toBeVisible();

      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      const activeSubTab = subTabBar.locator('a.active');
      await expect(activeSubTab).toContainText(/Analytics/i);
    });

    test('should navigate back to Current Draft from Analytics', async ({ page }) => {
      // First navigate to Analytics
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Then navigate back
      await page.click('.sub-tab-bar a[href="/draft"]');
      await page.waitForURL(/\/draft$/);

      const activeSubTab = page.locator('.sub-tab-bar a.active');
      await expect(activeSubTab).toContainText(/Current Draft/i);
    });
  });

  test.describe('Page Header and Title', () => {
    test('should display Draft Analytics page title', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      const pageTitle = page.locator('h1:has-text("Draft Analytics")');
      await expect(pageTitle).toBeVisible();
    });
  });

  test.describe('Loading and Empty States', () => {
    test('should display loading or content state', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for either loading, content, or empty state
      const loadingState = page.locator('.draft-analytics--loading');
      const contentState = page.locator('.draft-analytics__content');
      const emptyState = page.locator('.draft-analytics--empty');

      await expect(
        loadingState.or(contentState).or(emptyState)
      ).toBeVisible();
    });

    test('should display empty state message when no data', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // If empty state is shown, verify the message
      const emptyState = page.locator('.draft-analytics--empty');
      const isEmpty = await emptyState.isVisible({ timeout: 5000 }).catch(() => false);

      if (isEmpty) {
        await expect(page.locator('text=No Draft Data Available')).toBeVisible();
        await expect(
          page.locator('text=Complete some drafts to see your analytics')
        ).toBeVisible();
      }
    });
  });

  test.describe('Filters', () => {
    test('should display Set filter dropdown when data available', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for content to load
      const contentState = page.locator('.draft-analytics__content');
      const hasContent = await contentState.isVisible({ timeout: 10000 }).catch(() => false);

      if (hasContent) {
        const setSelect = page.locator('select#set-select');
        await expect(setSelect).toBeVisible();
      }
    });

    test('should display Format filter dropdown when data available', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for content to load
      const contentState = page.locator('.draft-analytics__content');
      const hasContent = await contentState.isVisible({ timeout: 10000 }).catch(() => false);

      if (hasContent) {
        const formatSelect = page.locator('select#format-select');
        await expect(formatSelect).toBeVisible();
      }
    });

    test('should allow changing set filter', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for content to load
      const contentState = page.locator('.draft-analytics__content');
      const hasContent = await contentState.isVisible({ timeout: 10000 }).catch(() => false);

      if (hasContent) {
        const setSelect = page.locator('select#set-select');
        await expect(setSelect).toBeVisible();

        // Get available options
        const options = await setSelect.locator('option').allTextContents();
        if (options.length > 1) {
          // Select a different option
          await setSelect.selectOption({ index: 1 });

          // Verify change
          const selectedValue = await setSelect.inputValue();
          expect(selectedValue).toBeTruthy();
        }
      }
    });

    test('should allow changing format filter', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for content to load
      const contentState = page.locator('.draft-analytics__content');
      const hasContent = await contentState.isVisible({ timeout: 10000 }).catch(() => false);

      if (hasContent) {
        const formatSelect = page.locator('select#format-select');
        await expect(formatSelect).toBeVisible();

        // Should have Premier Draft selected by default
        const defaultValue = await formatSelect.inputValue();
        expect(defaultValue).toBe('PremierDraft');

        // Select Quick Draft
        await formatSelect.selectOption('QuickDraft');

        const newValue = await formatSelect.inputValue();
        expect(newValue).toBe('QuickDraft');
      }
    });

    test('should not display per-page auto-refresh checkbox (#2023)', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // auto-refresh is now controlled globally via Settings, not per-page
      const autoRefreshLabel = page.locator('label:has-text("Auto-refresh")');
      await expect(autoRefreshLabel).not.toBeVisible();
    });
  });

  test.describe('Analytics Components', () => {
    test('should display Temporal Trends component section when data available', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for content to load
      const contentState = page.locator('.draft-analytics__content');
      const hasContent = await contentState.isVisible({ timeout: 10000 }).catch(() => false);

      if (hasContent) {
        // Check for TemporalTrends component (may be loading or have content)
        const temporalTrends = page.locator('.temporal-trends');
        await expect(temporalTrends).toBeVisible({ timeout: 5000 });
      }
    });

    test('should display Community Comparison component section when data available', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for content to load
      const contentState = page.locator('.draft-analytics__content');
      const hasContent = await contentState.isVisible({ timeout: 10000 }).catch(() => false);

      if (hasContent) {
        // Check for CommunityComparison component
        const communityComparison = page.locator('.community-comparison');
        await expect(communityComparison).toBeVisible({ timeout: 5000 });
      }
    });

    test('should display Format Insights component section when data available', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Wait for content to load
      const contentState = page.locator('.draft-analytics__content');
      const hasContent = await contentState.isVisible({ timeout: 10000 }).catch(() => false);

      if (hasContent) {
        // Check for FormatInsights component
        const formatInsights = page.locator('.format-insights');
        await expect(formatInsights).toBeVisible({ timeout: 5000 });
      }
    });
  });

  test.describe('Navigation Persistence', () => {
    test('should keep Draft tab active when on analytics page', async ({ page }) => {
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');

      // Main Draft tab should still be active
      const mainTab = page.locator('.tab-bar a[href="/draft"]');
      await expect(mainTab).toHaveClass(/active/);
    });

    test('should maintain sub-nav visibility across Draft pages', async ({ page }) => {
      // Start on Draft page
      await expect(page.locator('.sub-tab-bar')).toBeVisible();

      // Navigate to Analytics
      await page.click('.sub-tab-bar a[href="/draft-analytics"]');
      await page.waitForURL('**/draft-analytics');
      await expect(page.locator('.sub-tab-bar')).toBeVisible();

      // Navigate back to Draft
      await page.click('.sub-tab-bar a[href="/draft"]');
      await page.waitForURL(/\/draft$/);
      await expect(page.locator('.sub-tab-bar')).toBeVisible();
    });
  });

  test.describe('Direct Navigation', () => {
    test('should be accessible via direct URL', async ({ page }) => {
      await page.goto('/draft-analytics');

      // Should show the analytics page (loading, content, or empty)
      const loadingState = page.locator('.draft-analytics--loading');
      const contentState = page.locator('.draft-analytics__content');
      const emptyState = page.locator('.draft-analytics--empty');

      await expect(
        loadingState.or(contentState).or(emptyState)
      ).toBeVisible();
    });

    test('should show Draft sub-tabs when navigating directly', async ({ page }) => {
      await page.goto('/draft-analytics');

      const subTabBar = page.locator('.sub-tab-bar');
      await expect(subTabBar).toBeVisible();

      // Analytics should be active
      const activeSubTab = subTabBar.locator('a.active');
      await expect(activeSubTab).toContainText(/Analytics/i);
    });
  });
});

