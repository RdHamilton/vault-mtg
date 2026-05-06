import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import ProtectedRoute from './ProtectedRoute';

// Control useAuth return value per test
const mockUseAuth = vi.fn();

vi.mock('@clerk/react', () => ({
  useAuth: () => mockUseAuth(),
  SignInButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// ─── Children (wrapper) usage ────────────────────────────────────────────────

describe('ProtectedRoute — children (wrapper) usage', () => {
  it('shows loading state while Clerk is loading', () => {
    mockUseAuth.mockReturnValue({ isLoaded: false, isSignedIn: false });
    render(
      <MemoryRouter>
        <ProtectedRoute>
          <div data-testid="protected-content">Protected</div>
        </ProtectedRoute>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument();
  });

  it('shows sign-in prompt when user is not authenticated', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter>
        <ProtectedRoute>
          <div data-testid="protected-content">Protected</div>
        </ProtectedRoute>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-prompt')).toBeInTheDocument();
    expect(screen.getByTestId('protected-route-sign-in-btn')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument();
  });

  it('renders children when user is signed in', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    render(
      <MemoryRouter>
        <ProtectedRoute>
          <div data-testid="protected-content">Protected</div>
        </ProtectedRoute>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-content')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-route-prompt')).not.toBeInTheDocument();
  });

  it('sign-in prompt includes a sign-in button', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter>
        <ProtectedRoute>
          <div>content</div>
        </ProtectedRoute>
      </MemoryRouter>
    );
    const btn = screen.getByTestId('protected-route-sign-in-btn');
    expect(btn).toHaveTextContent('Sign In');
  });

  it('prompt title says "Sign in to continue"', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter>
        <ProtectedRoute>
          <div>content</div>
        </ProtectedRoute>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-title')).toHaveTextContent('Sign in to continue');
  });
});

// ─── Layout route (Outlet) usage ─────────────────────────────────────────────

describe('ProtectedRoute — layout route (Outlet) usage', () => {
  it('shows loading state while Clerk is loading', () => {
    mockUseAuth.mockReturnValue({ isLoaded: false, isSignedIn: false });
    render(
      <MemoryRouter initialEntries={['/protected']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/protected" element={<div data-testid="outlet-content">Protected Page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('outlet-content')).not.toBeInTheDocument();
  });

  it('shows sign-in prompt when unauthenticated (no children passed)', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter initialEntries={['/protected']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/protected" element={<div data-testid="outlet-content">Protected Page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-prompt')).toBeInTheDocument();
    expect(screen.queryByTestId('outlet-content')).not.toBeInTheDocument();
  });

  it('renders nested route content when authenticated', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    render(
      <MemoryRouter initialEntries={['/protected']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/protected" element={<div data-testid="outlet-content">Protected Page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByTestId('outlet-content')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-route-prompt')).not.toBeInTheDocument();
  });

  it('blocks access to all nested routes when unauthenticated', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter initialEntries={['/match-history']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/match-history" element={<div data-testid="match-history">Match History</div>} />
            <Route path="/settings" element={<div data-testid="settings">Settings</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-prompt')).toBeInTheDocument();
    expect(screen.queryByTestId('match-history')).not.toBeInTheDocument();
  });
});
