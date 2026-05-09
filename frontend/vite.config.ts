import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
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
