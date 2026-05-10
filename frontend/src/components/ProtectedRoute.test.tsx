import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import ProtectedRoute from './ProtectedRoute';

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
      <MemoryRouter initialEntries={['/draft']}>
        <ProtectedRoute>
          <div data-testid="protected-content">Protected</div>
        </ProtectedRoute>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-prompt')).toBeInTheDocument();
    expect(screen.getByTestId('protected-route-sign-in-btn')).toBeInTheDocument();
    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument();
  });

  it('prompt title names the page from the URL', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter initialEntries={['/match-history']}>
        <ProtectedRoute>
          <div data-testid="protected-content">Protected</div>
        </ProtectedRoute>
      </MemoryRouter>
    );
    expect(screen.getByText('Sign in to access Match History')).toBeInTheDocument();
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
      <MemoryRouter initialEntries={['/draft']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/draft" element={<div data-testid="outlet-content">Draft Page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByTestId('protected-route-prompt')).toBeInTheDocument();
    expect(screen.getByTestId('protected-route-sign-in-btn')).toBeInTheDocument();
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
