import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'
import { sentryVitePlugin } from '@sentry/vite-plugin'

// https://vite.dev/config/
//
// @sentry/vite-plugin: uploads source maps to Sentry during production/staging builds.
// Gated on SENTRY_AUTH_TOKEN being present so local builds (no secret) are unaffected.
// SENTRY_ORG and SENTRY_PROJECT are injected from GH Actions secrets; they must not be
// hardcoded here. The plugin release value matches VITE_APP_VERSION so source maps are
// tied to the same Sentry release tag as the running application.
const sentryPlugin = process.env.SENTRY_AUTH_TOKEN
  ? [
      sentryVitePlugin({
        org: process.env.SENTRY_ORG,
        project: process.env.SENTRY_PROJECT,
        authToken: process.env.SENTRY_AUTH_TOKEN,
        release: {
          name: process.env.VITE_APP_VERSION || undefined,
        },
        sourcemaps: {
          // Upload the Vite-generated source maps and then delete them from the
          // dist/ directory so they are not served publicly.
          filesToDeleteAfterUpload: ['./dist/**/*.map'],
        },
      }),
    ]
  : []

export default defineConfig({
  plugins: [react(), ...sentryPlugin],
  build: {
    // Enable source maps for all builds so the Sentry plugin can upload them.
    // Source map files are deleted from dist/ after upload (see sentryVitePlugin config above).
    sourcemap: true,
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      // When VITE_CLERK_TEST_MODE=true (E2E tests), replace @clerk/react with a
      // lightweight mock that reads auth state from window.__CLERK_TEST_STATE__.
      // This avoids requiring a real Clerk publishable key in test environments.
      ...(process.env.VITE_CLERK_TEST_MODE === 'true'
        ? { '@clerk/react': path.resolve(__dirname, './src/test/mocks/clerkMock.tsx') }
        : {}),
    },
  },
  server: {
    port: 3000,
    // Pre-transform critical entry points before workers start.
    // Eliminates the Vite on-demand transform race when fullyParallel + 4 CI
    // workers all hit the dev server simultaneously before module graph is warm.
    warmup: {
      clientFiles: [
        './src/main.tsx',
        './src/App.tsx',
        './src/components/Layout.tsx',
        './src/context/AppContext.tsx',
        './src/services/adapter.ts',
      ],
    },
  },
  optimizeDeps: {
    // Pre-bundle heavy deps during dev server startup so the first worker
    // request does not trigger a blocking dep-optimization crawl.
    include: [
      'react',
      'react-dom',
      'react-dom/client',
      'react-router-dom',
      '@sentry/react',
    ],
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    // API service tests that use msw/node run in Node environment to avoid the
    // jsdom AbortSignal class mismatch with Node 24's undici fetch validation.
    // These tests import and exercise the real apiClient (not a mock), so they
    // do NOT need jsdom DOM APIs. setup.ts conditionally imports jest-dom only
    // when document exists.
    environmentMatchGlobs: [
      ['**/*.integration.test.ts', 'node'],
      ['**/src/services/api/*.test.ts', 'node'],
    ],
    // Exclude E2E tests (Playwright) from Vitest
    exclude: [
      '**/node_modules/**',
      '**/dist/**',
      '**/tests/e2e/**', // Exclude Playwright E2E tests
    ],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html', 'cobertura'],
      exclude: [
        'node_modules/',
        'src/test/',
        '**/*.d.ts',
        '**/*.config.*',
        '**/mockData',
        'dist/',
        'tests/e2e/', // Exclude E2E tests from coverage
      ],
    },
  },
})
