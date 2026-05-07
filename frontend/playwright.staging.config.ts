import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for staging smoke tests (#1444)
 *
 * Targets the live staging BFF at staging-api.vaultmtg.app.
 * No webServer block — the staging BFF is already running.
 *
 * Run via:
 *   npm run test:staging
 *   npx playwright test --config=playwright.staging.config.ts
 *
 * Triggered by the INFRA-7 staging deploy workflow after a successful deploy.
 * Failures here must cause the deploy workflow post-step to fail.
 *
 * Suite is intentionally small (< 60 s total):
 *   1. Health check — GET /health returns 200 + expected shape
 *   2. Auth-gated routes return 401 without a token
 *   3. SSE endpoint accepts a connection (or returns 401, not a network error)
 */
export default defineConfig({
  testDir: './tests/e2e/staging',

  // Individual test timeout — keep the suite well under 60 s total.
  timeout: 20 * 1000,

  // Run staging tests sequentially to avoid overwhelming a shared staging env.
  fullyParallel: false,
  workers: 1,

  // No retries — a flaky staging env should surface as a real failure.
  retries: 0,

  forbidOnly: !!process.env.CI,

  reporter: [
    ['html', { open: 'never', outputFolder: 'playwright-report-staging' }],
    ['list'],
    ...(process.env.CI ? [['github'] as const] : []),
  ],

  use: {
    // Staging BFF base URL — override with STAGING_API_URL env var if needed.
    baseURL: process.env.STAGING_API_URL ?? 'https://staging-api.vaultmtg.app',

    // No browser needed; tests use the `request` fixture (APIRequestContext).
    // Still keep a browser project so Playwright can schedule tests normally.
    trace: 'on-first-retry',
  },

  projects: [
    {
      name: 'staging-smoke',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // No webServer — staging BFF is already live.
});
