import { fileURLToPath } from 'node:url';
import type { StorybookConfig } from '@storybook/react-vite';

const config: StorybookConfig = {
  // Stories live alongside their component as `ComponentName.stories.tsx`.
  // Restricted to `.ts`/`.tsx` — the SPA is TypeScript-only.
  stories: ['../src/**/*.stories.@(ts|tsx)'],

  // Storybook 10 ships docs, controls, backgrounds, and actions in core, so no
  // separate addons are required for autodocs or the controls panel.
  addons: [],

  framework: {
    // Vite builder — the SPA is built with Vite (see vite.config.ts).
    // Do not switch to the Webpack builder.
    name: '@storybook/react-vite',
    options: {},
  },

  // viteFinal lets the Storybook Vite build diverge from the app build.
  // Here we alias `@clerk/react` to a local mock so every story renders fully
  // offline and deterministically — no real publishable key, no network calls,
  // no live auth session. See `.storybook/clerk-mock.tsx` for the mock surface.
  viteFinal: async (viteConfig) => {
    viteConfig.resolve = viteConfig.resolve ?? {};
    viteConfig.resolve.alias = {
      ...viteConfig.resolve.alias,
      '@clerk/react': fileURLToPath(new URL('./clerk-mock.tsx', import.meta.url)),
    };
    return viteConfig;
  },
};

export default config;
