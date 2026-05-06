import { test, expect } from '@playwright/test'

test.describe('Daemon health indicator', () => {
  test('indicator is visible on a protected route @smoke', async ({ page }) => {
    // Use the /download public route — Layout renders on all routes including
    // public ones, so the nav (and daemon indicator) is always present.
    await page.goto('/download')
    await expect(page.locator('[data-testid="daemon-health-indicator"]')).toBeVisible()
  })
})
