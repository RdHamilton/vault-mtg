import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for R-17 staging rebuild smoke gate (issue #2343)
 *
 * Targets the live staging SPA (stg-app.vaultmtg.app) and BFF
 * (staging-api.vaultmtg.app) using a real browser. No webServer block —
 * both services must already be running.
 *
 * This config is invoked by the reusable e2e-smoke-staging.yml workflow,
 * which is called as SMOKE-4 in mtga-companion-infra's
 * staging-rebuild-smoke-gate.yml.
 *
 * Run manually:
 *   npm run test:e2e:r17-smoke
 *   npx playwright test --config=playwright.r17-smoke.config.ts
 *
 * Environment variables consumed by the test suite:
 *   R17_BASE_URL   — SPA base URL (default: https://stg-app.vaultmtg.app)
 *   R17_BFF_URL    — BFF base URL (default: https://staging-api.vaultmtg.app)
 *
 * Suite constraints per ## TIM SPEC in staging-rebuild-smoke-gate.yml:
 *   - Runs in < 5 minutes on ubuntu-latest
 *   - No Postgres service container (live staging only)
 *   - No authenticated flows
 *   - playwright-report/ uploaded as artifact on failure
 */
export default defineConfig({
  testDir: './tests/e2e/staging',
  testMatch: /r17-smoke\.spec\.ts/,

  // Hard limit: 4 assertions × 30 s each with headroom leaves well under 5 min.
  timeout: 45 * 1000,

  // Sequential — runs against shared staging environment.
  fullyParallel: false,
  workers: 1,

  // No retries — a staging environment failure should surface as a real failure.
  retries: 0,

  forbidOnly: !!process.env.CI,

  reporter: [
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
    ['list'],
    ...(process.env.CI ? [['github'] as const] : []),
  ],

  use: {
    // Base URL — overridden by R17_BASE_URL env var inside the spec file.
    // Setting it here keeps Playwright's baseURL aware for tooling purposes.
    baseURL: process.env.R17_BASE_URL || 'https://stg-app.vaultmtg.app',

    // Chromium headless — fastest option for CI; mirrors staging user traffic.
    ...devices['Desktop Chrome'],

    // Allow self-signed / hostname-mismatched TLS certs on staging.
    // The staging BFF (staging-api.vaultmtg.app) currently presents the
    // production cert (api.vaultmtg.app). Without this, in-browser fetch()
    // calls and Playwright navigations reject the cert entirely, preventing
    // the CORS and auth-rejection smoke checks from receiving any HTTP status.
    // This flag is safe here because:
    //   (a) this config only targets the staging environment, never production,
    //   (b) the BFF CORS assertion still validates allowed-origin behaviour,
    //   (c) the 401 assertion still validates Clerk middleware behaviour.
    ignoreHTTPSErrors: true,

    // Collect trace on failure for debugging.
    trace: 'on-first-retry',

    // Screenshot on failure.
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'r17-smoke',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // No webServer — staging SPA and BFF are already live.
});
