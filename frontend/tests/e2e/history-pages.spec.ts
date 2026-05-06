import { test, expect } from '@playwright/test'

/**
 * History Pages Smoke Tests
 *
 * Verifies that the cloud BFF match history and draft history pages load
 * and show either a data table or an empty state.
 */

test.describe('History pages', () => {
  test('match history page shows table or empty state @smoke', async ({ page }) => {
    await page.goto('/history/matches')

    // After loading, must show table OR empty state
    const table = page.locator('[data-testid="match-history-table"]')
    const empty = page.locator('[data-testid="match-history-empty"]')

    await expect(table.or(empty)).toBeVisible({ timeout: 15000 })
  })

  test('draft history page shows table or empty state @smoke', async ({ page }) => {
    await page.goto('/history/drafts')

    // After loading, must show table OR empty state
    const table = page.locator('[data-testid="draft-history-table"]')
    const empty = page.locator('[data-testid="draft-history-empty"]')

    await expect(table.or(empty)).toBeVisible({ timeout: 15000 })
  })
})
