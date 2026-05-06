import { test, expect } from '@playwright/test'

test.describe('Daemon health indicator', () => {
  test('indicator is visible in nav @smoke', async ({ page }) => {
    await page.goto('/download')
    await expect(page.locator('[data-testid="daemon-health-indicator"]')).toBeVisible()
  })
})
