import { useAuth, SignInButton } from '@clerk/react';
import './ProtectedRoute.css';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

/**
 * ProtectedRoute wraps content that requires authentication.
 * When the user is not signed in, it renders a sign-in prompt instead.
 * Used for routes that require a Clerk session (e.g., Draft).
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
          <h2 className="protected-route-title">Sign in to access Draft</h2>
          <p className="protected-route-subtitle">
            Create an account or sign in to use the live draft assistant.
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

  return <>{children}</>;
};

export default ProtectedRoute;
