import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist', '.vite', '**/*.test.{ts,tsx}', '**/test/**', 'src/types/models.ts', 'coverage/**', 'storybook-static/**']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
  },
  {
    // Storybook configuration & tooling files are not part of the app bundle,
    // so Vite Fast Refresh does not apply to them. The Clerk mock in
    // .storybook/clerk-mock.tsx must mirror @clerk/react's export shape, which
    // legitimately mixes component and non-component exports.
    files: ['.storybook/**/*.{ts,tsx}'],
    rules: {
      'react-refresh/only-export-components': 'off',
    },
  },
])
