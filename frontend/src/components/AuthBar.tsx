import { Show, SignInButton, SignUpButton, UserButton } from '@clerk/react';
import { useFeatureFlag } from '@/hooks/useFeatureFlag';
import './AuthBar.css';

/**
 * AuthBar renders sign-in/sign-up buttons when the user is signed out,
 * and the Clerk UserButton (avatar/menu) when signed in.
 * Placed in the Layout tab-bar right section.
 *
 * beta_invite_only gate:
 *   - flag on (true)  → SignInButton + SignUpButton (default beta experience)
 *   - flag off (false) → SignInButton only; SignUpButton hidden
 *   - flag loading (null) → optimistic: show SignUpButton (avoids flash on load)
 *
 * PostHog emits $feature_flag_called automatically for beta_invite_only on every
 * session once flags are loaded (posthog-js v1.372.9 auto-emission via
 * isFeatureEnabled → getFeatureFlagResult).
 */
const AuthBar = () => {
  const { enabled: signUpEnabled } = useFeatureFlag('beta_invite_only');

  // Optimistic: show sign-up when flag is loading (null) or on (true).
  // Hide only when flag is explicitly false.
  const showSignUp = signUpEnabled !== false;

  return (
    <div className="auth-bar" data-testid="auth-bar">
      <Show when="signed-out">
        <div className="auth-bar-signed-out" data-testid="auth-signed-out">
          <SignInButton mode="modal">
            <button className="auth-btn auth-btn-signin" data-testid="sign-in-btn">
              Sign In
            </button>
          </SignInButton>
          {showSignUp && (
            <SignUpButton mode="modal">
              <button className="auth-btn auth-btn-signup" data-testid="sign-up-btn">
                Sign Up
              </button>
            </SignUpButton>
          )}
        </div>
      </Show>
      <Show when="signed-in">
        <div className="auth-bar-signed-in" data-testid="auth-signed-in">
          <UserButton />
        </div>
      </Show>
    </div>
  );
};

export default AuthBar;
