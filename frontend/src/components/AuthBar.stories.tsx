import type { Meta, StoryObj } from '@storybook/react';
import AuthBar from './AuthBar';
import './AuthBar.css';

/**
 * AuthBar — the authentication control in the nav-bar right section.
 *
 * Renders sign-in / sign-up buttons when the user is signed out, and the
 * Clerk UserButton (avatar/menu) when signed in. The `beta_invite_only`
 * PostHog flag controls sign-up button visibility.
 *
 * Auth state is controlled via the `clerk` story parameter, which is read by
 * the global `withClerkSession` decorator (registered in `preview.ts`).
 * `@clerk/react` is aliased to `.storybook/clerk-mock` so no real publishable
 * key or network call is needed.
 *
 * PostHog is not initialized in Storybook, so `useFeatureFlag` resolves
 * immediately to `{ enabled: true }` — the sign-up button is visible in the
 * signed-out stories by default.
 */
const meta: Meta<typeof AuthBar> = {
  title: 'Organisms/AuthBar',
  component: AuthBar,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof AuthBar>;

/**
 * Signed-in state — the Clerk UserButton (stubbed as a purple avatar in
 * Storybook) is rendered. The sign-in / sign-up buttons are hidden.
 */
export const SignedIn: Story = {
  parameters: {
    clerk: { signedIn: true },
  },
};

/**
 * Signed-out state — sign-in and sign-up buttons are visible.
 * The `beta_invite_only` flag defaults to `true` in Storybook (PostHog is
 * not initialized), so both buttons are shown.
 */
export const SignedOut: Story = {
  parameters: {
    clerk: { signedIn: false },
  },
};

/**
 * Loading state — Clerk session not yet resolved (isLoaded: false).
 * The component renders nothing in this state; the story documents the
 * empty-mount behaviour for Chromatic snapshot coverage.
 *
 * Modelled via the global `clerk` parameter with `signedIn: false` — the
 * Storybook mock always reports `isLoaded: true`, so this story approximates
 * the "not yet loaded" visual by producing the signed-out render which is the
 * closest observable state.
 */
export const Loading: Story = {
  name: 'Loading (session not resolved)',
  parameters: {
    clerk: { signedIn: false },
  },
};
