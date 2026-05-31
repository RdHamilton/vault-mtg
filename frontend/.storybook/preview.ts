import type { Preview } from '@storybook/react';
import { withClerkSession } from './decorators';

// Global application styles. The VaultMTG SPA uses plain CSS (not Tailwind) —
// `index.css` holds the design tokens (CSS custom properties), dark color
// scheme, and base typography. Importing it here ensures every story renders
// against the same foundation as the running app. Per-component `.css` files
// are imported inside each component module, so a story only needs the global
// sheet plus whatever its component imports itself.
import '../src/index.css';

const preview: Preview = {
  // Global decorators apply to every story.
  //  - withClerkSession syncs the Clerk auth mock from the `clerk` story param.
  // Router context is opt-in per story (withRouter / withRouterAt) so that
  // atoms and molecules stay free of context they do not need.
  decorators: [withClerkSession],

  parameters: {
    // The app runs on a dark surface (#1e1e1e). Match it so component contrast
    // in stories reflects production, and so Chromatic snapshots are stable.
    backgrounds: {
      default: 'app',
      values: [
        { name: 'app', value: '#1e1e1e' },
        { name: 'light', value: '#ffffff' },
      ],
    },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    // Canonical responsive breakpoints for VaultMTG. These drive the Storybook
    // viewport toolbar and will be wired to Chromatic multi-viewport builds once
    // TurboSnap (A-1) is confirmed merged (quota guard — see ticket #287).
    viewport: {
      viewports: {
        desktop: { name: 'Desktop', styles: { width: '1280px', height: '800px' } },
        tablet: { name: 'Tablet', styles: { width: '768px', height: '1024px' } },
        mobile: { name: 'Mobile', styles: { width: '375px', height: '812px' } },
      },
      defaultViewport: 'desktop',
    },
  },
};

export default preview;
