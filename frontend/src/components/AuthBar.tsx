import { Show, SignInButton, SignUpButton, UserButton } from '@clerk/react';
import './AuthBar.css';

/**
 * AuthBar renders sign-in/sign-up buttons when the user is signed out,
 * and the Clerk UserButton (avatar/menu) when signed in.
 * Placed in the Layout tab-bar right section.
 */
const AuthBar = () => {
  return (
    <div className="auth-bar" data-testid="auth-bar">
      <Show when="signed-out">
        <div className="auth-bar-signed-out" data-testid="auth-signed-out">
          <SignInButton mode="modal">
            <button className="auth-btn auth-btn-signin" data-testid="sign-in-btn">
              Sign In
            </button>
          </SignInButton>
          <SignUpButton mode="modal">
            <button className="auth-btn auth-btn-signup" data-testid="sign-up-btn">
              Sign Up
            </button>
          </SignUpButton>
        </div>
      </Show>
      <Show when="signed-in">
        <div className="auth-bar-signed-in" data-testid="auth-signed-in">
          <UserButton afterSignOutUrl="/" />
        </div>
      </Show>
    </div>
  );
};

export default AuthBar;
