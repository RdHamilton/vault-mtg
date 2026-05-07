import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';

// Mock posthog-js before any module that imports it.
vi.mock('posthog-js', () => ({
  default: {
    init: vi.fn(),
    capture: vi.fn(),
    identify: vi.fn(),
    reset: vi.fn(),
    register: vi.fn(),
  },
}));

// Mock analytics module so we can spy on identifyUser / captureEvent.
const mockIdentifyUser = vi.fn();
const mockCaptureEvent = vi.fn();
const mockResetIdentity = vi.fn();

vi.mock('@/services/analytics', () => ({
  identifyUser: (...args: unknown[]) => mockIdentifyUser(...args),
  captureEvent: (...args: unknown[]) => mockCaptureEvent(...args),
  resetIdentity: () => mockResetIdentity(),
  Events: {
    FUNNEL_SIGN_UP_COMPLETED: 'funnel_sign_up_completed',
  },
}));

// Clerk mock — controlled per test.
const mockUseUser = vi.fn();
vi.mock('@clerk/react', () => ({
  useUser: () => mockUseUser(),
}));

const SESSION_KEY = 'vaultmtg_ph_funnel_sign_up_completed_fired';

describe('usePostHogIdentity', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
  });

  it('does nothing when Clerk is not yet loaded', async () => {
    mockUseUser.mockReturnValue({ isLoaded: false, isSignedIn: false, user: null });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockIdentifyUser).not.toHaveBeenCalled();
    expect(mockCaptureEvent).not.toHaveBeenCalled();
  });

  it('calls identifyUser with clerk user id when signed in', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockIdentifyUser).toHaveBeenCalledWith('user_abc');
  });

  it('fires funnel_sign_up_completed once per session when signed in', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockCaptureEvent).toHaveBeenCalledWith('funnel_sign_up_completed', {});
    expect(sessionStorage.getItem(SESSION_KEY)).toBe('1');
  });

  it('does NOT fire funnel_sign_up_completed if sessionStorage guard is already set', async () => {
    sessionStorage.setItem(SESSION_KEY, '1');
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockCaptureEvent).not.toHaveBeenCalled();
  });

  it('calls resetIdentity when user is signed out after being signed in', async () => {
    // First render: signed in
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    const { rerender } = renderHook(() => usePostHogIdentity());

    // Second render: signed out
    mockUseUser.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    rerender();

    expect(mockResetIdentity).toHaveBeenCalledOnce();
  });
});
