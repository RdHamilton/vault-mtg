import type { Meta, StoryObj } from '@storybook/react';
import { withRouter } from '../../.storybook/decorators';
import Profile from './Profile';
import './Profile.css';

/**
 * Profile — dedicated page at `/profile` for viewing and editing the
 * authenticated user's identity (display name, avatar, email) (#2025).
 *
 * Auth state is sourced via the `useUserHook` prop (DI pattern per ADR-009)
 * so stories exercise every auth and loading state without a real Clerk
 * session or network call.
 *
 * Decorators:
 *  - withRouter — required because the page calls `useNavigate(-1)` for the
 *    Back button.
 */
const meta: Meta<typeof Profile> = {
  title: 'Organisms/Profile',
  component: Profile,
  decorators: [withRouter],
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof Profile>;

const mockUser = {
  id: 'user_storybook_mock',
  firstName: 'Planeswalker',
  lastName: 'Mock',
  fullName: 'Planeswalker Mock',
  primaryEmailAddress: { emailAddress: 'planeswalker@vaultmtg.test' },
  imageUrl: '',
  update: async () => {},
  setProfileImage: async () => {},
};

/**
 * Loaded, signed-in user — the full profile editor is visible.
 */
export const Authenticated: Story = {
  args: {
    useUserHook: () => ({
      isLoaded: true,
      isSignedIn: true,
      user: mockUser,
    }),
  },
};

/**
 * Signed-in user with an avatar image URL.
 */
export const AuthenticatedWithAvatar: Story = {
  args: {
    useUserHook: () => ({
      isLoaded: true,
      isSignedIn: true,
      user: {
        ...mockUser,
        imageUrl: 'https://api.dicebear.com/7.x/bottts/svg?seed=vaultmtg',
      },
    }),
  },
};

/**
 * Clerk session is still loading — shows the loading spinner/message.
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
 * User is not signed in — shows the unauthenticated fallback message.
 * In production this state should not be reachable (ProtectedRoute guards /profile).
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
