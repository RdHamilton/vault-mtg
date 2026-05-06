import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import AuthBar from './AuthBar';

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

describe('AuthBar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
