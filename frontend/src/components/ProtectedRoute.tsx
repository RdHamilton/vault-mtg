import { Outlet } from 'react-router-dom';
import { useAuth, SignInButton } from '@clerk/react';
import './ProtectedRoute.css';

interface ProtectedRouteProps {
  children?: React.ReactNode;
}

/**
 * ProtectedRoute guards content that requires authentication.
 * When the user is not signed in, it renders a sign-in prompt instead.
 *
 * Supports two usage patterns:
 *   1. Layout route (React Router v6): <Route element={<ProtectedRoute />}>
 *      — renders <Outlet /> when authenticated so nested routes render normally.
 *   2. Wrapper (legacy): <ProtectedRoute><Component /></ProtectedRoute>
 *      — renders children when authenticated.
 */
const ProtectedRoute = ({ children }: ProtectedRouteProps) => {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) {
    return (
      <div className="protected-route-loading" data-testid="protected-route-loading">
        <span>Loading...</span>
      </div>
    );
  }

  if (!isSignedIn) {
    return (
      <div className="protected-route-prompt" data-testid="protected-route-prompt">
        <div className="protected-route-card">
          <h2 className="protected-route-title" data-testid="protected-route-title">
            Sign in to continue
          </h2>
          <p className="protected-route-subtitle">
            Create an account or sign in to access this page.
          </p>
          <div className="protected-route-actions">
            <SignInButton mode="modal">
              <button className="protected-route-btn" data-testid="protected-route-sign-in-btn">
                Sign In
              </button>
            </SignInButton>
          </div>
        </div>
      </div>
    );
  }

  // Layout route: render nested routes via Outlet
  if (children === undefined) {
    return <Outlet />;
  }

  // Wrapper usage: render provided children
  return <>{children}</>;
};

export default ProtectedRoute;
