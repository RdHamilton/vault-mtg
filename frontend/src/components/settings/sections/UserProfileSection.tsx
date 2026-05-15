import { useUser } from '@clerk/react';
import { Link } from 'react-router-dom';
import './UserProfileSection.css';

/**
 * UserProfileSection — displays the authenticated Clerk user's profile info.
 *
 * Auth state is sourced exclusively from useUser() per ADR-009 and CLAUDE.md.
 * No auth state is duplicated in Redux / Context / Zustand.
 *
 * The useUserHook prop enables dependency injection in Vitest tests without
 * requiring a real Clerk publishable key or full ClerkProvider wrapping.
 */

export type UserProfileData = {
  isLoaded: boolean;
  isSignedIn: boolean | undefined;
  user: {
    fullName: string | null;
    primaryEmailAddress?: { emailAddress: string } | null;
    imageUrl: string;
    id: string;
  } | null;
};

export interface UserProfileSectionProps {
  /** Injected hook — defaults to useUser() from @clerk/react. Overridden in tests. */
  useUserHook?: () => UserProfileData;
}

/** Default adapter: delegates to the real Clerk useUser() hook. */
const defaultUseUser = (): UserProfileData => {
  // eslint-disable-next-line react-hooks/rules-of-hooks
  const { isLoaded, isSignedIn, user } = useUser();
  return {
    isLoaded,
    isSignedIn,
    user: user
      ? {
          fullName: user.fullName,
          primaryEmailAddress: user.primaryEmailAddress,
          imageUrl: user.imageUrl,
          id: user.id,
        }
      : null,
  };
};

export function UserProfileSection({
  useUserHook = defaultUseUser,
}: UserProfileSectionProps) {
  const { isLoaded, isSignedIn, user } = useUserHook();

  return (
    <div className="settings-section user-profile-section" data-testid="user-profile-section">
      <h2 className="section-title">User Profile</h2>

      {!isLoaded && (
        <div
          className="user-profile-loading"
          data-testid="user-profile-loading"
          aria-live="polite"
          aria-busy="true"
        >
          Loading profile…
        </div>
      )}

      {isLoaded && !isSignedIn && (
        <div className="user-profile-unauthenticated" data-testid="user-profile-unauthenticated">
          Not signed in.
        </div>
      )}

      {isLoaded && isSignedIn && user && (
        <div className="user-profile-content" data-testid="user-profile-content">
          {user.imageUrl && (
            <img
              className="user-profile-avatar"
              data-testid="user-profile-avatar"
              src={user.imageUrl}
              alt={user.fullName ?? 'User avatar'}
            />
          )}
          <div className="user-profile-details">
            {user.fullName && (
              <div className="user-profile-name" data-testid="user-profile-name">
                {user.fullName}
              </div>
            )}
            {user.primaryEmailAddress?.emailAddress && (
              <div className="user-profile-email" data-testid="user-profile-email">
                {user.primaryEmailAddress.emailAddress}
              </div>
            )}
            <Link
              to="/profile"
              className="user-profile-link"
              data-testid="user-profile-link"
            >
              View &amp; edit full profile →
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
