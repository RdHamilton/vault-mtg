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
  },
  optimizeDeps: {
    force: true, // Always re-bundle dependencies on startup (avoids stale cache issues)
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
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
