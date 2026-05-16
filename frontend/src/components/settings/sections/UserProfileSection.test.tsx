import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithRouter } from '@/test/utils/testUtils';
import { UserProfileSection } from './UserProfileSection';
import type { UserProfileData } from './UserProfileSection';

// Alias renderWithRouter as render for brevity
const render = renderWithRouter;

// ---------------------------------------------------------------------------
// Helpers — inject pre-built state via the useUserHook prop (adapter pattern)
// ---------------------------------------------------------------------------

const loadingState = (): UserProfileData => ({
  isLoaded: false,
  isSignedIn: undefined,
  user: null,
});

const signedOutState = (): UserProfileData => ({
  isLoaded: true,
  isSignedIn: false,
  user: null,
});

const signedInState = (overrides?: Partial<UserProfileData['user']>): UserProfileData => ({
  isLoaded: true,
  isSignedIn: true,
  user: {
    id: 'user_test_123',
    fullName: 'Jane Doe',
    primaryEmailAddress: { emailAddress: 'jane@example.com' },
    imageUrl: 'https://example.com/avatar.png',
    ...overrides,
  },
});

// ---------------------------------------------------------------------------
// Tests: loading state
// ---------------------------------------------------------------------------

describe('UserProfileSection — loading state', () => {
  it('shows loading indicator when Clerk is not yet loaded', () => {
    render(<UserProfileSection useUserHook={loadingState} />);
    expect(screen.getByTestId('user-profile-loading')).toBeInTheDocument();
  });

  it('does NOT show profile content while loading', () => {
    render(<UserProfileSection useUserHook={loadingState} />);
    expect(screen.queryByTestId('user-profile-content')).not.toBeInTheDocument();
  });

  it('does NOT show unauthenticated message while loading', () => {
    render(<UserProfileSection useUserHook={loadingState} />);
    expect(screen.queryByTestId('user-profile-unauthenticated')).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: unauthenticated state (handles ProtectedRoute redirect scenario)
// ---------------------------------------------------------------------------

describe('UserProfileSection — unauthenticated state', () => {
  it('shows unauthenticated message when not signed in', () => {
    render(<UserProfileSection useUserHook={signedOutState} />);
    expect(screen.getByTestId('user-profile-unauthenticated')).toBeInTheDocument();
  });

  it('does NOT show profile content when signed out', () => {
    render(<UserProfileSection useUserHook={signedOutState} />);
    expect(screen.queryByTestId('user-profile-content')).not.toBeInTheDocument();
  });

  it('does NOT show loading indicator when signed out (auth loaded)', () => {
    render(<UserProfileSection useUserHook={signedOutState} />);
    expect(screen.queryByTestId('user-profile-loading')).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: signed-in state with full user data
// ---------------------------------------------------------------------------

describe('UserProfileSection — renders with user data', () => {
  it('renders the section heading', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    expect(screen.getByRole('heading', { name: /user profile/i })).toBeInTheDocument();
  });

  it('renders the profile content container', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    expect(screen.getByTestId('user-profile-content')).toBeInTheDocument();
  });

  it('renders the display name', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    expect(screen.getByTestId('user-profile-name')).toHaveTextContent('Jane Doe');
  });

  it('renders the email address', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    expect(screen.getByTestId('user-profile-email')).toHaveTextContent('jane@example.com');
  });

  it('renders the avatar image with src from user.imageUrl', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    const avatar = screen.getByTestId('user-profile-avatar');
    expect(avatar).toBeInTheDocument();
    expect(avatar).toHaveAttribute('src', 'https://example.com/avatar.png');
  });

  it('avatar alt text falls back to full name', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    const avatar = screen.getByTestId('user-profile-avatar');
    expect(avatar).toHaveAttribute('alt', 'Jane Doe');
  });

  it('does NOT render avatar when imageUrl is empty', () => {
    render(<UserProfileSection useUserHook={() => signedInState({ imageUrl: '' })} />);
    expect(screen.queryByTestId('user-profile-avatar')).not.toBeInTheDocument();
  });

  it('does NOT render name element when fullName is null', () => {
    render(<UserProfileSection useUserHook={() => signedInState({ fullName: null })} />);
    expect(screen.queryByTestId('user-profile-name')).not.toBeInTheDocument();
  });

  it('does NOT render email element when primaryEmailAddress is null', () => {
    render(
      <UserProfileSection
        useUserHook={() => signedInState({ primaryEmailAddress: null })}
      />
    );
    expect(screen.queryByTestId('user-profile-email')).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: section container
// ---------------------------------------------------------------------------

describe('UserProfileSection — container', () => {
  it('renders the section data-testid attribute', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    expect(screen.getByTestId('user-profile-section')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: profile link (AC4 — settings section links to /profile)
// ---------------------------------------------------------------------------

describe('UserProfileSection — profile link (AC4)', () => {
  it('renders a link to /profile when signed in', () => {
    render(<UserProfileSection useUserHook={() => signedInState()} />);
    const link = screen.getByTestId('user-profile-link');
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', '/profile');
  });

  it('does NOT render the profile link when signed out', () => {
    render(<UserProfileSection useUserHook={signedOutState} />);
    expect(screen.queryByTestId('user-profile-link')).not.toBeInTheDocument();
  });

  it('does NOT render the profile link while loading', () => {
    render(<UserProfileSection useUserHook={loadingState} />);
    expect(screen.queryByTestId('user-profile-link')).not.toBeInTheDocument();
  });
});
