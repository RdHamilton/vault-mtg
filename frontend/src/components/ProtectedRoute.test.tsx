import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import ProtectedRoute from './ProtectedRoute';

const mockUseAuth = vi.fn();
const mockTrackEvent = vi.fn();

vi.mock('@clerk/react', () => ({
  useAuth: () => mockUseAuth(),
  SignInButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('@/services/analytics', () => ({
  trackEvent: (...args: unknown[]) => mockTrackEvent(...args),
}));

// ─── Children (wrapper) usage ────────────────────────────────────────────────

describe('ProtectedRoute — children (wrapper) usage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

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

// ─── funnel_sign_up_started ───────────────────────────────────────────────────

describe('ProtectedRoute — funnel_sign_up_started analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fires funnel_sign_up_started with entry_point protected_route_redirect when sign-in button is clicked', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter initialEntries={['/draft']}>
        <ProtectedRoute>
          <div>Protected</div>
        </ProtectedRoute>
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByTestId('protected-route-sign-in-btn'));
    const funnelCalls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'funnel_sign_up_started',
    );
    expect(funnelCalls).toHaveLength(1);
    expect(funnelCalls[0][0]).toEqual({
      name: 'funnel_sign_up_started',
      properties: { entry_point: 'protected_route_redirect' },
    });
  });

  it('NEGATIVE: funnel_sign_up_started payload never contains user_id', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(
      <MemoryRouter initialEntries={['/collection']}>
        <ProtectedRoute>
          <div>Protected</div>
        </ProtectedRoute>
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByTestId('protected-route-sign-in-btn'));
    mockTrackEvent.mock.calls
      .filter(([e]: [{ name: string }]) => e.name === 'funnel_sign_up_started')
      .forEach(([e]: [{ properties: Record<string, unknown> }]) => {
        expect(e.properties).not.toHaveProperty('user_id');
      });
  });

  it('does NOT fire funnel_sign_up_started when user is already signed in', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    render(
      <MemoryRouter initialEntries={['/draft']}>
        <ProtectedRoute>
          <div data-testid="protected-content">Protected</div>
        </ProtectedRoute>
      </MemoryRouter>,
    );
    // No sign-in button rendered when authenticated
    expect(screen.queryByTestId('protected-route-sign-in-btn')).not.toBeInTheDocument();
    const funnelCalls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'funnel_sign_up_started',
    );
    expect(funnelCalls).toHaveLength(0);
  });
});
