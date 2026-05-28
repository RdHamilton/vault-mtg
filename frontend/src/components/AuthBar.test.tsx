import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import AuthBar from './AuthBar';
import * as useFeatureFlagModule from '@/hooks/useFeatureFlag';

// Mock @clerk/react so tests don't need a real Clerk publishable key
vi.mock('@clerk/react', () => ({
  Show: ({ when, children }: { when: string; children: React.ReactNode }) => {
    // Simulate signed-out state for tests (default)
    if (when === 'signed-out') {
      return <>{children}</>;
    }
    return null;
  },
  SignInButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SignUpButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  UserButton: () => <div data-testid="user-button">UserButton</div>,
}));

// Mock useFeatureFlag so component tests control flag state without PostHog
vi.mock('@/hooks/useFeatureFlag', () => ({
  useFeatureFlag: vi.fn().mockReturnValue({ enabled: true }),
}));

describe('AuthBar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default for existing tests: beta_invite_only flag is ON (sign-up visible)
    vi.mocked(useFeatureFlagModule.useFeatureFlag).mockReturnValue({ enabled: true });
  });

  it('renders the auth-bar container', () => {
    render(<AuthBar />);
    expect(screen.getByTestId('auth-bar')).toBeInTheDocument();
  });

  it('shows sign-in and sign-up buttons when signed out', () => {
    render(<AuthBar />);
    expect(screen.getByTestId('sign-in-btn')).toBeInTheDocument();
    expect(screen.getByTestId('sign-up-btn')).toBeInTheDocument();
  });

  it('sign-in button has correct text', () => {
    render(<AuthBar />);
    expect(screen.getByTestId('sign-in-btn')).toHaveTextContent('Sign In');
  });

  it('sign-up button has correct text', () => {
    render(<AuthBar />);
    expect(screen.getByTestId('sign-up-btn')).toHaveTextContent('Sign Up');
  });

  // ---------------------------------------------------------------------------
  // beta_invite_only flag — three states
  // ---------------------------------------------------------------------------

  it('beta_invite_only flag OFF — sign-up button is NOT rendered', () => {
    vi.mocked(useFeatureFlagModule.useFeatureFlag).mockReturnValue({ enabled: false });
    render(<AuthBar />);
    // Sign-in must still be present
    expect(screen.getByTestId('sign-in-btn')).toBeInTheDocument();
    // Sign-up must be absent when flag is off
    expect(screen.queryByTestId('sign-up-btn')).not.toBeInTheDocument();
  });

  it('beta_invite_only flag ON — sign-up button IS rendered', () => {
    vi.mocked(useFeatureFlagModule.useFeatureFlag).mockReturnValue({ enabled: true });
    render(<AuthBar />);
    expect(screen.getByTestId('sign-up-btn')).toBeInTheDocument();
  });

  it('beta_invite_only flag loading (null) — sign-up button IS rendered (optimistic default)', () => {
    vi.mocked(useFeatureFlagModule.useFeatureFlag).mockReturnValue({ enabled: null });
    render(<AuthBar />);
    // Optimistic: show sign-up while flag is still loading to avoid flash
    expect(screen.getByTestId('sign-up-btn')).toBeInTheDocument();
  });
});

describe('AuthBar (signed-in state)', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it('shows UserButton when signed in', async () => {
    vi.doMock('@clerk/react', () => ({
      Show: ({ when, children }: { when: string; children: React.ReactNode }) => {
        if (when === 'signed-in') {
          return <>{children}</>;
        }
        return null;
      },
      SignInButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      SignUpButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      UserButton: () => <div data-testid="user-button">UserButton</div>,
    }));

    const { default: AuthBarSignedIn } = await import('./AuthBar');
    render(<AuthBarSignedIn />);
    expect(screen.getByTestId('user-button')).toBeInTheDocument();
  });
});
