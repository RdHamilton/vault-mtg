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
 *   CLERK_SECRET_KEY       — Clerk secret key for generating testing tokens (required for auth tests)
 *
 * Suite constraints:
 *   - 60 s per test timeout (increased from 30 s — CI networkidle too strict, see #1949)
 *   - Sequential (workers: 1) — avoids hammering staging with parallel sessions
 *   - No retries — staging instability should surface as a real failure
 */
export default defineConfig({
  testDir: './tests/e2e/staging',
  testMatch: /staging-spa-smoke\.spec\.ts/,

  // Individual test timeout — 60 s to handle CI runner latency (#1949)
  timeout: 60 * 1000,

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
    // Staging SPA base URL — override with STAGING_SPA_URL env var if needed.
    // Use `||` so an empty-string CI secret falls back to the default — `??`
    // only treats `undefined`/`null` as missing and left baseURL = '' in CI
    // when the secret was set-but-empty (#1933).
    baseURL: process.env.STAGING_SPA_URL || 'https://stg-app.vaultmtg.app',

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
