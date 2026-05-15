import { useState, useRef } from 'react';
import { useUser } from '@clerk/react';
import { useNavigate } from 'react-router-dom';
import './Profile.css';

/**
 * Profile page — dedicated route at /profile for viewing and editing the
 * authenticated Clerk user's identity (display name, avatar, email).
 *
 * Auth state is sourced exclusively from useUser() per ADR-009 and CLAUDE.md.
 * Mutations use user.update() and user.setProfileImage() from the Clerk SDK.
 * No auth state is duplicated in Redux / Context / Zustand.
 */

export interface ProfilePageProps {
  /** Dependency-injected hook for tests — defaults to useUser() from @clerk/react. */
  useUserHook?: () => {
    isLoaded: boolean;
    isSignedIn: boolean | undefined;
    user: {
      id: string;
      fullName: string | null;
      firstName: string | null;
      lastName: string | null;
      primaryEmailAddress?: { emailAddress: string } | null;
      imageUrl: string;
      update: (params: { firstName?: string; lastName?: string }) => Promise<unknown>;
      setProfileImage: (params: { file: File | null }) => Promise<unknown>;
    } | null;
  };
}

const defaultUseUser = () => {
  // eslint-disable-next-line react-hooks/rules-of-hooks
  const { isLoaded, isSignedIn, user } = useUser();
  return { isLoaded, isSignedIn, user };
};

const Profile = ({ useUserHook = defaultUseUser }: ProfilePageProps) => {
  const navigate = useNavigate();
  const { isLoaded, isSignedIn, user } = useUserHook();

  // Display name editing state
  const [isEditingName, setIsEditingName] = useState(false);
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [nameSaving, setNameSaving] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [nameSuccess, setNameSuccess] = useState(false);

  // Avatar upload state
  const [avatarUploading, setAvatarUploading] = useState(false);
  const [avatarError, setAvatarError] = useState<string | null>(null);
  const [avatarSuccess, setAvatarSuccess] = useState(false);
  const avatarInputRef = useRef<HTMLInputElement>(null);

  const handleEditNameStart = () => {
    setFirstName(user?.firstName ?? '');
    setLastName(user?.lastName ?? '');
    setNameError(null);
    setNameSuccess(false);
    setIsEditingName(true);
  };

  const handleEditNameCancel = () => {
    setIsEditingName(false);
    setNameError(null);
  };

  const handleSaveName = async () => {
    if (!user) return;
    setNameSaving(true);
    setNameError(null);
    try {
      await user.update({ firstName: firstName.trim(), lastName: lastName.trim() });
      setIsEditingName(false);
      setNameSuccess(true);
      setTimeout(() => setNameSuccess(false), 3000);
    } catch (err) {
      setNameError(err instanceof Error ? err.message : 'Failed to update display name.');
    } finally {
      setNameSaving(false);
    }
  };

  const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !user) return;

    setAvatarUploading(true);
    setAvatarError(null);
    setAvatarSuccess(false);
    try {
      await user.setProfileImage({ file });
      setAvatarSuccess(true);
      setTimeout(() => setAvatarSuccess(false), 3000);
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : 'Failed to upload avatar.');
    } finally {
      setAvatarUploading(false);
      // Reset file input so the same file can be picked again if needed
      if (avatarInputRef.current) {
        avatarInputRef.current.value = '';
      }
    }
  };

  // --- Loading state ---
  if (!isLoaded) {
    return (
      <div className="page-container profile-page" data-testid="profile-page">
        <div
          className="profile-loading"
          data-testid="profile-loading"
          aria-live="polite"
          aria-busy="true"
        >
          Loading profile…
        </div>
      </div>
    );
  }

  // --- Unauthenticated (should not happen — route is protected) ---
  if (!isSignedIn || !user) {
    return (
      <div className="page-container profile-page" data-testid="profile-page">
        <div className="profile-unauthenticated" data-testid="profile-unauthenticated">
          You must be signed in to view your profile.
        </div>
      </div>
    );
  }

  return (
    <div className="page-container profile-page" data-testid="profile-page">
      <div className="profile-header">
        <button
          className="profile-back-button"
          data-testid="profile-back-button"
          onClick={() => navigate(-1)}
          aria-label="Go back"
        >
          ← Back
        </button>
        <h1 className="page-title" data-testid="profile-title">
          User Profile
        </h1>
      </div>

      <div className="profile-content">
        {/* --- Avatar section --- */}
        <section className="profile-section" data-testid="profile-avatar-section">
          <h2 className="profile-section-title">Avatar</h2>
          <div className="profile-avatar-container">
            {user.imageUrl ? (
              <img
                className="profile-avatar"
                data-testid="profile-avatar"
                src={user.imageUrl}
                alt={user.fullName ?? 'User avatar'}
              />
            ) : (
              <div className="profile-avatar-placeholder" data-testid="profile-avatar-placeholder">
                {(user.firstName?.[0] ?? user.fullName?.[0] ?? '?').toUpperCase()}
              </div>
            )}
            <div className="profile-avatar-actions">
              <button
                className="secondary-button profile-avatar-upload-button"
                data-testid="profile-avatar-upload-button"
                onClick={() => avatarInputRef.current?.click()}
                disabled={avatarUploading}
                aria-label="Upload new avatar"
              >
                {avatarUploading ? 'Uploading…' : 'Change Avatar'}
              </button>
              <input
                ref={avatarInputRef}
                type="file"
                accept="image/*"
                className="profile-avatar-input"
                data-testid="profile-avatar-input"
                onChange={handleAvatarChange}
                aria-label="Select avatar image"
              />
            </div>
          </div>
          {avatarError && (
            <div className="profile-error" data-testid="profile-avatar-error" role="alert">
              {avatarError}
            </div>
          )}
          {avatarSuccess && (
            <div className="profile-success" data-testid="profile-avatar-success" role="status">
              Avatar updated successfully!
            </div>
          )}
        </section>

        {/* --- Display name section --- */}
        <section className="profile-section" data-testid="profile-name-section">
          <h2 className="profile-section-title">Display Name</h2>
          {isEditingName ? (
            <div className="profile-name-form" data-testid="profile-name-form">
              <div className="profile-name-fields">
                <label className="profile-field-label" htmlFor="profile-first-name">
                  First Name
                  <input
                    id="profile-first-name"
                    className="profile-field-input"
                    data-testid="profile-first-name-input"
                    type="text"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="First name"
                    autoFocus
                  />
                </label>
                <label className="profile-field-label" htmlFor="profile-last-name">
                  Last Name
                  <input
                    id="profile-last-name"
                    className="profile-field-input"
                    data-testid="profile-last-name-input"
                    type="text"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="Last name"
                  />
                </label>
              </div>
              <div className="profile-name-actions">
                <button
                  className="primary-button"
                  data-testid="profile-save-name-button"
                  onClick={handleSaveName}
                  disabled={nameSaving}
                >
                  {nameSaving ? 'Saving…' : 'Save'}
                </button>
                <button
                  className="secondary-button"
                  data-testid="profile-cancel-name-button"
                  onClick={handleEditNameCancel}
                  disabled={nameSaving}
                >
                  Cancel
                </button>
              </div>
              {nameError && (
                <div className="profile-error" data-testid="profile-name-error" role="alert">
                  {nameError}
                </div>
              )}
            </div>
          ) : (
            <div className="profile-name-display" data-testid="profile-name-display">
              <span className="profile-name-value" data-testid="profile-name-value">
                {user.fullName ?? '—'}
              </span>
              <button
                className="secondary-button profile-edit-button"
                data-testid="profile-edit-name-button"
                onClick={handleEditNameStart}
                aria-label="Edit display name"
              >
                Edit
              </button>
            </div>
          )}
          {nameSuccess && (
            <div className="profile-success" data-testid="profile-name-success" role="status">
              Display name updated successfully!
            </div>
          )}
        </section>

        {/* --- Email section --- */}
        <section className="profile-section" data-testid="profile-email-section">
          <h2 className="profile-section-title">Email</h2>
          <div className="profile-email-display" data-testid="profile-email-display">
            <span className="profile-email-value" data-testid="profile-email-value">
              {user.primaryEmailAddress?.emailAddress ?? '—'}
            </span>
            <p className="profile-email-note">
              Email is managed by your Clerk account and cannot be changed here.
            </p>
          </div>
        </section>
      </div>
    </div>
  );
};

export default Profile;
