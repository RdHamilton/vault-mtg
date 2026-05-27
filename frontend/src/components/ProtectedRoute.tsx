import { Outlet, useLocation } from 'react-router-dom';
import { useAuth, SignInButton } from '@clerk/react';
import { trackEvent } from '@/services/analytics';
import './ProtectedRoute.css';

interface ProtectedRouteProps {
  children?: React.ReactNode;
}

/**
 * ProtectedRoute guards content that requires authentication.
 * When the user is not signed in, it renders a sign-in prompt with a modal trigger.
 *
 * Supports two usage patterns:
 *   1. Layout route (React Router v6): <Route element={<ProtectedRoute />}>
 *      — renders <Outlet /> when authenticated so nested routes render normally.
 *   2. Wrapper (legacy): <ProtectedRoute><Component /></ProtectedRoute>
 *      — renders children when authenticated.
 */
const ProtectedRoute = ({ children }: ProtectedRouteProps) => {
  const { isSignedIn, isLoaded } = useAuth();
  const location = useLocation();

  if (!isLoaded) {
    return (
      <div className="protected-route-loading" data-testid="protected-route-loading">
        <span>Loading...</span>
      </div>
    );
  }

  if (!isSignedIn) {
    const segments = location.pathname.split('/').filter(Boolean);
    const lastSegment = segments[segments.length - 1] ?? 'this page';
    const pageName = lastSegment
      .split('-')
      .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ');

    return (
      <div className="protected-route-prompt" data-testid="protected-route-prompt">
        <div className="protected-route-card">
          <p className="protected-route-title">Sign in to access {pageName}</p>
          <p className="protected-route-subtitle">
            Create an account or sign in to view your data.
          </p>
          <div className="protected-route-actions">
            <SignInButton mode="modal">
              <button
                className="protected-route-btn"
                data-testid="protected-route-sign-in-btn"
                onClick={() =>
                  trackEvent({
                    name: 'funnel_sign_up_started',
                    properties: { entry_point: 'protected_route_redirect' },
                  })
                }
              >
                Sign In
              </button>
            </SignInButton>
          </div>
        </div>
      </div>
    );
  }

  if (children === undefined) {
    return <Outlet />;
  }

  return <>{children}</>;
};

export default ProtectedRoute;
