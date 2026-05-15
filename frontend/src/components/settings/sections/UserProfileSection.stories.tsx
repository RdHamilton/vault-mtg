import type { Meta, StoryObj } from '@storybook/react';
import { MemoryRouter } from 'react-router-dom';
import { UserProfileSection } from './UserProfileSection';
import type { UserProfileData } from './UserProfileSection';

/**
 * UserProfileSection — Settings section that displays the authenticated Clerk
 * user's avatar, full name, and primary email with a link to the full profile
 * editor at /profile.
 *
 * Auth state is sourced via the `useUserHook` prop (REST API adapter + DI
 * pattern) so stories can exercise every auth state without a real Clerk
 * session or network call.
 *
 * Routing context is required because the component renders a `<Link to="/profile">`.
 */
const meta: Meta<typeof UserProfileSection> = {
  title: 'Organisms/UserProfileSection',
  component: UserProfileSection,
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof UserProfileSection>;

const signedInUser: UserProfileData = {
  isLoaded: true,
  isSignedIn: true,
  user: {
    fullName: 'Planeswalker Mock',
    primaryEmailAddress: { emailAddress: 'planeswalker@vaultmtg.test' },
    imageUrl: '',
    id: 'user_storybook_mock',
  },
};

/**
 * Loaded, signed-in user with full profile data.
 */
export const SignedIn: Story = {
  args: {
    useUserHook: () => signedInUser,
  },
};

/**
 * Signed-in user whose profile image is available via URL.
 */
export const SignedInWithAvatar: Story = {
  args: {
    useUserHook: () => ({
      ...signedInUser,
      user: {
        ...signedInUser.user!,
        imageUrl:
          'https://api.dicebear.com/7.x/bottts/svg?seed=vaultmtg',
        fullName: 'Avatar User',
        primaryEmailAddress: { emailAddress: 'avatar@vaultmtg.test' },
      },
    }),
  },
};

/**
 * Clerk session is still loading — shows the loading state.
 */
export const Loading: Story = {
  args: {
    useUserHook: () => ({
      isLoaded: false,
      isSignedIn: undefined,
      user: null,
    }),
  },
};

/**
 * User is not signed in — shows the unauthenticated fallback.
 * In production this should not be reachable (ProtectedRoute guards the page),
 * but the component handles it gracefully.
 */
export const SignedOut: Story = {
  args: {
    useUserHook: () => ({
      isLoaded: true,
      isSignedIn: false,
      user: null,
    }),
  },
};
