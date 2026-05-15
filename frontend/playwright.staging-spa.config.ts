import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for staging SPA browser smoke tests (#1933)
 *
 * Targets the live staging SPA at stg-app.vaultmtg.app using a real browser.
 * No webServer block — the staging SPA is already live.
 *
 * Run via:
 *   npm run test:e2e:staging-spa
 *   npx playwright test --config=playwright.staging-spa.config.ts
 *
 * Triggered by deploy-spa-staging.yml after a successful CloudFront invalidation.
 * Failures here must cause the deploy workflow post-step to fail.
 *
 * Required environment variables:
 *   STAGING_SPA_URL        — override the staging SPA base URL (optional)
 *   SMOKE_CLERK_EMAIL      — Clerk test account email (required for auth tests)
 *   SMOKE_CLERK_PASSWORD   — Clerk test account password (required for auth tests)
 *
 * Suite constraints:
 *   - 30 s per test timeout
 *   - Sequential (workers: 1) — avoids hammering staging with parallel sessions
 *   - No retries — staging instability should surface as a real failure
 */
export default defineConfig({
  testDir: './tests/e2e/staging',
  testMatch: /staging-spa-smoke\.spec\.ts/,

  // Individual test timeout
  timeout: 30 * 1000,

  // Sequential — one worker against shared staging environment
  fullyParallel: false,
  workers: 1,

  // No retries — a flaky staging env should surface as a real failure
  retries: 0,

  forbidOnly: !!process.env.CI,

  reporter: [
    ['html', { open: 'never', outputFolder: 'playwright-report-staging-spa' }],
    ['list'],
    ...(process.env.CI ? [['github'] as const] : []),
  ],

  use: {
    // Staging SPA base URL — override with STAGING_SPA_URL env var if needed
    baseURL: process.env.STAGING_SPA_URL ?? 'https://stg-app.vaultmtg.app',

    // Collect trace on failure for debugging
    trace: 'on-first-retry',

    // Screenshot on failure
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'staging-spa',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // No webServer — staging SPA is already live
});
