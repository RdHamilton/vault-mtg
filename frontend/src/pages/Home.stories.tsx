import type { Meta, StoryObj } from '@storybook/react';
import { withRouter } from '../../.storybook/decorators';
import { clerkMockState } from '../../.storybook/clerk-mock';
import Home from './Home';
import './Home.css';

/**
 * Home — the authenticated landing page for VaultMTG (#2005).
 *
 * Rendered at the `/home` route after sign-in. Displays a personalised
 * welcome heading (sourced from `useUser()`) and a grid of feature entry
 * points (Match History, Draft, Decks, Collection) that navigate on click.
 *
 * Decorators:
 *  - withRouter (global MemoryRouter) — required because Home calls
 *    `useNavigate()` to handle feature card clicks.
 *  - withClerkSession (global) — already registered in preview.ts;
 *    `@clerk/react` is aliased to `.storybook/clerk-mock` so `useUser()`
 *    returns the mock user without a real Clerk publishable key.
 */
const meta: Meta<typeof Home> = {
  title: 'Organisms/Home',
  component: Home,
  decorators: [withRouter],
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof Home>;

/**
 * Default authenticated view — signed-in mock user "Planeswalker Mock".
 * The heading reads "Welcome back, Planeswalker".
 */
export const Authenticated: Story = {
  parameters: {
    clerk: { signedIn: true },
  },
};

/**
 * Signed-out (or null user) fallback — heading reads "Welcome back, Planeswalker"
 * because that is the component's own fallback string when `user` is null.
 *
 * In production this state should not be reachable (the route is protected),
 * but it documents the component's graceful degradation.
 */
export const SignedOut: Story = {
  decorators: [
    (Story) => {
      clerkMockState.signedIn = false;
      return <Story />;
    },
  ],
  parameters: {
    clerk: { signedIn: false },
  },
};
