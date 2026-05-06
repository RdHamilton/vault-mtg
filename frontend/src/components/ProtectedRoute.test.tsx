import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import ProtectedRoute from './ProtectedRoute';

// We'll control useAuth return value per test
const mockUseAuth = vi.fn();

vi.mock('@clerk/react', () => ({
  useAuth: () => mockUseAuth(),
  SignInButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

describe('ProtectedRoute', () => {
  it('shows loading state while Clerk is loading', () => {
    mockUseAuth.mockReturnValue({ isLoaded: false, isSignedIn: false });
    render(
      <ProtectedRoute>
        <div data-testid="protected-content">Protected</div>
      </ProtectedRoute>
    );
    expect(screen.getByTestId('protected-route-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument();
  });

  it('shows sign-in prompt when user is not authenticated', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <ProtectedRoute>
        <div data-testid="protected-content">Protected</div>
      </ProtectedRoute>
    );
    expect(screen.getByTestId('protected-route-prompt')).toBeInTheDocument();
    expect(screen.getByTestId('protected-route-sign-in-btn')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument();
  });

  it('renders children when user is signed in', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    render(
      <ProtectedRoute>
        <div data-testid="protected-content">Protected</div>
      </ProtectedRoute>
    );
    expect(screen.getByTestId('protected-content')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-route-prompt')).not.toBeInTheDocument();
  });

  it('sign-in prompt includes a sign-in button', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <ProtectedRoute>
        <div>content</div>
      </ProtectedRoute>
    );
    const btn = screen.getByTestId('protected-route-sign-in-btn');
    expect(btn).toHaveTextContent('Sign In');
  });

  it('prompt title mentions Draft', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <ProtectedRoute>
        <div>content</div>
      </ProtectedRoute>
    );
    expect(screen.getByText(/Sign in to access Draft/i)).toBeInTheDocument();
  });
});
