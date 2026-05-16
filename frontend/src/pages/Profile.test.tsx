import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor, fireEvent, act } from '@testing-library/react';
import { renderWithRouter } from '@/test/utils/testUtils';
import Profile from './Profile';
import type { ProfilePageProps } from './Profile';

// ---------------------------------------------------------------------------
// Helpers — construct useUserHook stubs with the same shape Profile expects
// ---------------------------------------------------------------------------

type UserStub = NonNullable<ReturnType<NonNullable<ProfilePageProps['useUserHook']>>['user']>;

const makeUser = (overrides: Partial<UserStub> = {}): UserStub => ({
  id: 'user_test_123',
  fullName: 'Jane Doe',
  firstName: 'Jane',
  lastName: 'Doe',
  primaryEmailAddress: { emailAddress: 'jane@example.com' },
  imageUrl: 'https://example.com/avatar.png',
  update: vi.fn().mockResolvedValue(undefined),
  setProfileImage: vi.fn().mockResolvedValue(undefined),
  ...overrides,
});

const loadingHook = (): ReturnType<NonNullable<ProfilePageProps['useUserHook']>> => ({
  isLoaded: false,
  isSignedIn: undefined,
  user: null,
});

const signedOutHook = (): ReturnType<NonNullable<ProfilePageProps['useUserHook']>> => ({
  isLoaded: true,
  isSignedIn: false,
  user: null,
});

const signedInHook = (userOverrides: Partial<UserStub> = {}) =>
  (): ReturnType<NonNullable<ProfilePageProps['useUserHook']>> => ({
    isLoaded: true,
    isSignedIn: true,
    user: makeUser(userOverrides),
  });

// ---------------------------------------------------------------------------
// Tests: loading state
// ---------------------------------------------------------------------------

describe('Profile — loading state', () => {
  it('renders loading indicator when Clerk is not yet loaded', () => {
    renderWithRouter(<Profile useUserHook={loadingHook} />);
    expect(screen.getByTestId('profile-loading')).toBeInTheDocument();
  });

  it('does NOT render profile content while loading', () => {
    renderWithRouter(<Profile useUserHook={loadingHook} />);
    expect(screen.queryByTestId('profile-avatar-section')).not.toBeInTheDocument();
    expect(screen.queryByTestId('profile-name-section')).not.toBeInTheDocument();
    expect(screen.queryByTestId('profile-email-section')).not.toBeInTheDocument();
  });

  it('renders page container while loading', () => {
    renderWithRouter(<Profile useUserHook={loadingHook} />);
    expect(screen.getByTestId('profile-page')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: unauthenticated state
// ---------------------------------------------------------------------------

describe('Profile — unauthenticated state', () => {
  it('renders unauthenticated message when not signed in', () => {
    renderWithRouter(<Profile useUserHook={signedOutHook} />);
    expect(screen.getByTestId('profile-unauthenticated')).toBeInTheDocument();
  });

  it('does NOT render profile sections when signed out', () => {
    renderWithRouter(<Profile useUserHook={signedOutHook} />);
    expect(screen.queryByTestId('profile-avatar-section')).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: signed-in — display (AC1)
// ---------------------------------------------------------------------------

describe('Profile — display (AC1)', () => {
  it('renders the page title', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-title')).toHaveTextContent('User Profile');
  });

  it('renders the avatar with imageUrl as src', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    const avatar = screen.getByTestId('profile-avatar');
    expect(avatar).toHaveAttribute('src', 'https://example.com/avatar.png');
  });

  it('avatar alt text matches full name', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-avatar')).toHaveAttribute('alt', 'Jane Doe');
  });

  it('renders placeholder initials when imageUrl is empty', () => {
    renderWithRouter(<Profile useUserHook={signedInHook({ imageUrl: '' })} />);
    expect(screen.queryByTestId('profile-avatar')).not.toBeInTheDocument();
    expect(screen.getByTestId('profile-avatar-placeholder')).toBeInTheDocument();
    expect(screen.getByTestId('profile-avatar-placeholder')).toHaveTextContent('J');
  });

  it('renders the display name', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-name-value')).toHaveTextContent('Jane Doe');
  });

  it('shows em-dash when fullName is null', () => {
    renderWithRouter(<Profile useUserHook={signedInHook({ fullName: null, firstName: null, lastName: null })} />);
    expect(screen.getByTestId('profile-name-value')).toHaveTextContent('—');
  });

  it('renders the email address', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-email-value')).toHaveTextContent('jane@example.com');
  });

  it('shows em-dash when email is null', () => {
    renderWithRouter(<Profile useUserHook={signedInHook({ primaryEmailAddress: null })} />);
    expect(screen.getByTestId('profile-email-value')).toHaveTextContent('—');
  });

  it('renders all three profile sections', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-avatar-section')).toBeInTheDocument();
    expect(screen.getByTestId('profile-name-section')).toBeInTheDocument();
    expect(screen.getByTestId('profile-email-section')).toBeInTheDocument();
  });

  it('renders a back button', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-back-button')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: display name editing (AC2)
// ---------------------------------------------------------------------------

describe('Profile — display name editing (AC2)', () => {
  it('shows Edit button in name display mode', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-edit-name-button')).toBeInTheDocument();
    expect(screen.queryByTestId('profile-name-form')).not.toBeInTheDocument();
  });

  it('switches to edit form when Edit is clicked', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    fireEvent.click(screen.getByTestId('profile-edit-name-button'));
    expect(screen.getByTestId('profile-name-form')).toBeInTheDocument();
    expect(screen.queryByTestId('profile-name-display')).not.toBeInTheDocument();
  });

  it('pre-fills first and last name inputs with current values', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    fireEvent.click(screen.getByTestId('profile-edit-name-button'));
    expect(screen.getByTestId('profile-first-name-input')).toHaveValue('Jane');
    expect(screen.getByTestId('profile-last-name-input')).toHaveValue('Doe');
  });

  it('cancels edit and returns to display mode on Cancel', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    fireEvent.click(screen.getByTestId('profile-edit-name-button'));
    expect(screen.getByTestId('profile-name-form')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('profile-cancel-name-button'));
    expect(screen.queryByTestId('profile-name-form')).not.toBeInTheDocument();
    expect(screen.getByTestId('profile-name-display')).toBeInTheDocument();
  });

  it('calls user.update() with new name when Save is clicked', async () => {
    const user = makeUser();
    renderWithRouter(<Profile useUserHook={() => ({ isLoaded: true, isSignedIn: true, user })} />);

    fireEvent.click(screen.getByTestId('profile-edit-name-button'));
    fireEvent.change(screen.getByTestId('profile-first-name-input'), { target: { value: 'Janet' } });
    fireEvent.change(screen.getByTestId('profile-last-name-input'), { target: { value: 'Smith' } });

    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-save-name-button'));
    });

    expect(user.update).toHaveBeenCalledWith({ firstName: 'Janet', lastName: 'Smith' });
  });

  it('returns to display mode after successful save', async () => {
    const user = makeUser();
    renderWithRouter(<Profile useUserHook={() => ({ isLoaded: true, isSignedIn: true, user })} />);

    fireEvent.click(screen.getByTestId('profile-edit-name-button'));

    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-save-name-button'));
    });

    await waitFor(() => {
      expect(screen.queryByTestId('profile-name-form')).not.toBeInTheDocument();
      expect(screen.getByTestId('profile-name-display')).toBeInTheDocument();
    });
  });

  it('shows success banner after successful save', async () => {
    const user = makeUser();
    renderWithRouter(<Profile useUserHook={() => ({ isLoaded: true, isSignedIn: true, user })} />);

    fireEvent.click(screen.getByTestId('profile-edit-name-button'));

    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-save-name-button'));
    });

    await waitFor(() => {
      expect(screen.getByTestId('profile-name-success')).toBeInTheDocument();
    });
  });

  it('shows error message when user.update() rejects', async () => {
    const user = makeUser({
      update: vi.fn().mockRejectedValue(new Error('Update failed')),
    });
    renderWithRouter(<Profile useUserHook={() => ({ isLoaded: true, isSignedIn: true, user })} />);

    fireEvent.click(screen.getByTestId('profile-edit-name-button'));

    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-save-name-button'));
    });

    await waitFor(() => {
      expect(screen.getByTestId('profile-name-error')).toHaveTextContent('Update failed');
    });
    // Stays in edit mode so user can try again
    expect(screen.getByTestId('profile-name-form')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: avatar upload (AC2)
// ---------------------------------------------------------------------------

describe('Profile — avatar upload (AC2)', () => {
  it('renders Change Avatar button', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    expect(screen.getByTestId('profile-avatar-upload-button')).toBeInTheDocument();
  });

  it('renders hidden file input', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    const input = screen.getByTestId('profile-avatar-input');
    expect(input).toHaveAttribute('type', 'file');
    expect(input).toHaveAttribute('accept', 'image/*');
  });

  it('calls user.setProfileImage() when a file is selected', async () => {
    const user = makeUser();
    renderWithRouter(<Profile useUserHook={() => ({ isLoaded: true, isSignedIn: true, user })} />);

    const file = new File(['png'], 'avatar.png', { type: 'image/png' });
    const input = screen.getByTestId('profile-avatar-input');

    await act(async () => {
      fireEvent.change(input, { target: { files: [file] } });
    });

    expect(user.setProfileImage).toHaveBeenCalledWith({ file });
  });

  it('shows success banner after successful avatar upload', async () => {
    const user = makeUser();
    renderWithRouter(<Profile useUserHook={() => ({ isLoaded: true, isSignedIn: true, user })} />);

    const file = new File(['png'], 'avatar.png', { type: 'image/png' });
    const input = screen.getByTestId('profile-avatar-input');

    await act(async () => {
      fireEvent.change(input, { target: { files: [file] } });
    });

    await waitFor(() => {
      expect(screen.getByTestId('profile-avatar-success')).toBeInTheDocument();
    });
  });

  it('shows error when setProfileImage() rejects', async () => {
    const user = makeUser({
      setProfileImage: vi.fn().mockRejectedValue(new Error('Upload failed')),
    });
    renderWithRouter(<Profile useUserHook={() => ({ isLoaded: true, isSignedIn: true, user })} />);

    const file = new File(['png'], 'avatar.png', { type: 'image/png' });
    const input = screen.getByTestId('profile-avatar-input');

    await act(async () => {
      fireEvent.change(input, { target: { files: [file] } });
    });

    await waitFor(() => {
      expect(screen.getByTestId('profile-avatar-error')).toHaveTextContent('Upload failed');
    });
  });
});

// ---------------------------------------------------------------------------
// Tests: nav / routing (AC3)
// ---------------------------------------------------------------------------

describe('Profile — navigation (AC3)', () => {
  it('renders the back button that navigates', () => {
    renderWithRouter(<Profile useUserHook={signedInHook()} />);
    // Back button is present; clicking it calls navigate(-1) which is a no-op in jsdom
    const back = screen.getByTestId('profile-back-button');
    expect(back).toBeInTheDocument();
    // Should not throw when clicked
    fireEvent.click(back);
  });
});
