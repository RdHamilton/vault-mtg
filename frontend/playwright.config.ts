import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for E2E testing of MTGA Companion
 *
 * This configuration uses the REST API backend for testing.
 *
 * Test Projects:
 * - smoke: Quick critical path tests (@smoke tagged) - for post-merge validation
 * - full: All E2E tests - for release validation
 * - firefox: Smoke tests on Firefox - for cross-browser release testing
 * - webkit: Smoke tests on WebKit - for cross-browser release testing
 *
 * The webServer config starts:
 * 1. Go REST API server on port 8080
 * 2. Vite dev server on port 5173 (with REST API mode enabled)
 */
export default defineConfig({
  testDir: './tests/e2e',

  // Maximum time one test can run for
  timeout: 30 * 1000,

  // Run tests in parallel for faster CI execution
  fullyParallel: true,
  workers: process.env.CI ? 4 : undefined,

  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,

  // Retry on CI only
  retries: process.env.CI ? 2 : 0,

  // Global assertion timeout for expect().toBeVisible(), toHaveText(), etc.
  // actionTimeout (below) only governs page actions (click, fill, waitForSelector).
  // Without this, Playwright's default of 5 s governs — too short for cold CI.
  expect: { timeout: 30_000 },

  // Reporter to use
  reporter: [
    ['html', { open: 'never' }],
    ['list'],
    ...(process.env.CI ? [['github'] as const] : []),
  ],

  // Shared settings for all the projects below
  use: {
    // Base URL for the Vite dev server
    baseURL: 'http://localhost:3000',

    // Timeout for each Playwright action (click, fill, waitForSelector, etc.)
    actionTimeout: 30_000,

    // Collect trace on failure for debugging
    trace: 'on-first-retry',

    // Take screenshot on failure
    screenshot: 'only-on-failure',

    // Record video on failure
    video: 'retain-on-failure',
  },

  // Configure projects for different test scenarios
  projects: [
    // Smoke tests - quick critical path validation
    {
      name: 'smoke',
      grep: /@smoke/,
      use: { ...devices['Desktop Chrome'] },
    },
    // Full test suite - all E2E tests
    {
      name: 'full',
      use: { ...devices['Desktop Chrome'] },
    },
    // Cross-browser smoke tests for releases
    {
      name: 'firefox',
      grep: /@smoke/,
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      grep: /@smoke/,
      use: { ...devices['Desktop Safari'] },
    },
    // Screenshots for documentation
    {
      name: 'screenshots',
      testMatch: /screenshots\.spec\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1280, height: 800 },
      },
    },
    // Pipeline tests - uses log fixtures to test full data flow.
    // Timeout raised to 60 s: on a cold CI runner, Vite transforms all modules
    // on-demand for the first request from each of the 4 parallel workers.
    // The combined page.goto + initializeServices + React mount can exceed 30 s.
    {
      name: 'pipeline',
      testMatch: /pipeline\.spec\.ts/,
      timeout: 60_000,
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // Start both servers for E2E testing
  webServer: [
    // Go REST API server on port 8080
    // In CI, use a temp database with fixtures; locally, reuse existing server
    // For pipeline tests (USE_LOG_FIXTURES=true), use daemon with log file
    {
      command: process.env.CI
        ? '../bin/mtga-bff'
        : 'cd .. && go run ./services/bff/cmd/main.go',
      url: 'http://localhost:8080/health',
      timeout: 120 * 1000,
      reuseExistingServer: !process.env.CI,
      stdout: 'pipe',
      stderr: 'pipe',
    },
    // On CI: build then preview. vite preview serves pre-compiled static files —
    // no on-demand module transforms. This eliminates the cold-start bottleneck
    // where 4 parallel Playwright workers each wait 30+ s for Vite to transform
    // hundreds of TypeScript modules on first request.
    //
    // On local: continue using vite dev (HMR, instant feedback).
    //
    // Both env vars must be set at BUILD time so Vite bakes them into the bundle:
    //   VITE_USE_REST_API=true  — enables REST API adapter
    //   VITE_CLERK_TEST_MODE=true — aliases @clerk/react → clerkMock.tsx
    //   VITE_BFF_URL — must be overridden at build time in CI so the preview
    //     bundle points at localhost:8080 (the BFF webServer below) instead of
    //     .env.production's value (https://api.vaultmtg.app/api/v1). Without
    //     this override, initializeServices() in the browser calls the remote
    //     production API, which is unreachable from the CI runner and blocks
    //     renderApp() until the network request times out — preventing the React
    //     tree (and app-container) from ever mounting during the 60 s window.
    {
      command: process.env.CI
        ? 'VITE_USE_REST_API=true VITE_CLERK_TEST_MODE=true VITE_BFF_URL=http://localhost:8080/api/v1 npm run build && npx vite preview --port 3000'
        : 'VITE_USE_REST_API=true VITE_CLERK_TEST_MODE=true npm run dev',
      url: 'http://localhost:3000',
      timeout: 180 * 1000,
      reuseExistingServer: !process.env.CI,
      stdout: 'pipe',
      stderr: 'pipe',
    },
  ],
});
