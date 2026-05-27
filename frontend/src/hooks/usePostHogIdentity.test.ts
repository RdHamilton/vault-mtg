import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';

// Mock posthog-js before any module that imports it.
vi.mock('posthog-js', () => ({
  default: {
    init: vi.fn(),
    capture: vi.fn(),
    identify: vi.fn(),
    reset: vi.fn(),
    register: vi.fn(),
    startSessionRecording: vi.fn(),
    stopSessionRecording: vi.fn(),
  },
}));

// Mock analytics module so we can spy on identifyUser / trackEvent.
const mockIdentifyUser = vi.fn();
const mockTrackEvent = vi.fn();
const mockResetIdentity = vi.fn();
const mockStartSessionReplay = vi.fn();
const mockStopSessionReplay = vi.fn();
const mockRegisterSuperProperties = vi.fn();

vi.mock('@/services/analytics', () => ({
  identifyUser: (...args: unknown[]) => mockIdentifyUser(...args),
  trackEvent: (...args: unknown[]) => mockTrackEvent(...args),
  resetIdentity: () => mockResetIdentity(),
  startSessionReplay: () => mockStartSessionReplay(),
  stopSessionReplay: () => mockStopSessionReplay(),
  registerSuperProperties: (...args: unknown[]) => mockRegisterSuperProperties(...args),
}));

// Clerk mock — controlled per test.
const mockUseUser = vi.fn();
vi.mock('@clerk/react', () => ({
  useUser: () => mockUseUser(),
}));

const SESSION_KEY = 'vaultmtg_ph_funnel_sign_up_completed_fired';
const SESSION_STARTED_KEY = 'vaultmtg_ph_app_session_started_fired';

describe('usePostHogIdentity', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    vi.resetModules();
  });

  // ── Pre-existing behaviour (preserved) ─────────────────────────────────────

  it('does nothing when Clerk is not yet loaded', async () => {
    mockUseUser.mockReturnValue({ isLoaded: false, isSignedIn: false, user: null });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockIdentifyUser).not.toHaveBeenCalled();
    expect(mockTrackEvent).not.toHaveBeenCalled();
    expect(mockStartSessionReplay).not.toHaveBeenCalled();
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

  it('starts session replay when user is signed in', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockStartSessionReplay).toHaveBeenCalledOnce();
  });

  it('does NOT start session replay when user is not signed in', async () => {
    mockUseUser.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockStartSessionReplay).not.toHaveBeenCalled();
  });

  it('fires funnel_sign_up_completed once per session when signed in', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    const funnelCalls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'funnel_sign_up_completed',
    );
    expect(funnelCalls).toHaveLength(1);
    expect(funnelCalls[0][0].properties.auth_method).toBe('email');
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

    const funnelCalls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'funnel_sign_up_completed',
    );
    expect(funnelCalls).toHaveLength(0);
  });

  it('calls resetIdentity when user is signed out after being signed in', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    const { rerender } = renderHook(() => usePostHogIdentity());

    mockUseUser.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    rerender();

    expect(mockResetIdentity).toHaveBeenCalledOnce();
  });

  it('stops session replay when user signs out', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    const { rerender } = renderHook(() => usePostHogIdentity());

    mockUseUser.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    rerender();

    expect(mockStopSessionReplay).toHaveBeenCalledOnce();
  });

  // ── New behaviour (session lifecycle) ──────────────────────────────────────

  it('fires app_user_identified on successful identify (no user_id in payload)', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    const identifiedCalls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'app_user_identified',
    );
    expect(identifiedCalls).toHaveLength(1);
    // CRITICAL: user_id must NOT appear in the payload (Ray adj. Q3)
    expect(identifiedCalls[0][0].properties).not.toHaveProperty('user_id');
  });

  it('NEGATIVE: app_user_identified event never contains user_id field', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_secret_id' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    mockTrackEvent.mock.calls
      .filter(([e]: [{ name: string }]) => e.name === 'app_user_identified')
      .forEach(([e]: [{ properties: Record<string, unknown> }]) => {
        expect(Object.keys(e.properties)).not.toContain('user_id');
        // Explicitly confirm no value was snuck in
        expect(e.properties.user_id).toBeUndefined();
      });
  });

  it('fires app_user_signed_out BEFORE resetIdentity on sign-out', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    const { rerender } = renderHook(() => usePostHogIdentity());

    const callOrder: string[] = [];
    mockTrackEvent.mockImplementation((e: { name: string }) => {
      callOrder.push(`trackEvent:${e.name}`);
    });
    mockResetIdentity.mockImplementation(() => {
      callOrder.push('resetIdentity');
    });

    mockUseUser.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    rerender();

    const signedOutIdx = callOrder.indexOf('trackEvent:app_user_signed_out');
    const resetIdx = callOrder.indexOf('resetIdentity');
    expect(signedOutIdx).toBeGreaterThanOrEqual(0);
    expect(resetIdx).toBeGreaterThan(signedOutIdx);
  });

  it('fires app_user_signed_out exactly once on sign-out', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    const { rerender } = renderHook(() => usePostHogIdentity());

    mockUseUser.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    rerender();

    const signedOutCalls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'app_user_signed_out',
    );
    expect(signedOutCalls).toHaveLength(1);
  });

  // ── Super-properties ────────────────────────────────────────────────────────

  it('registers super-properties after successful identify', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    expect(mockRegisterSuperProperties).toHaveBeenCalledOnce();
    const props = mockRegisterSuperProperties.mock.calls[0][0] as Record<string, unknown>;
    // Narrowed set per Ray adj. #3: app_version, is_signed_in, platform only
    expect(props).toHaveProperty('app_version');
    expect(props).toHaveProperty('is_signed_in', true);
    expect(props).toHaveProperty('platform');
    // daemon_status must NOT be a super-property in this ticket (Ray adj. #3)
    expect(props).not.toHaveProperty('daemon_status');
  });

  it('super-properties include app_version string (unknown fallback allowed)', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    renderHook(() => usePostHogIdentity());

    const props = mockRegisterSuperProperties.mock.calls[0][0] as Record<string, unknown>;
    expect(typeof props.app_version).toBe('string');
    // 'unknown' is an allowed fallback per Ray adj. #5
    expect(props.app_version).toBeTruthy();
  });

  it('updates is_signed_in super-property to false after sign-out', async () => {
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_abc' },
    });
    const { usePostHogIdentity } = await import('./usePostHogIdentity');
    const { rerender } = renderHook(() => usePostHogIdentity());

    mockUseUser.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    rerender();

    // Last registerSuperProperties call must reflect signed-out state
    const lastCall = mockRegisterSuperProperties.mock.calls.at(-1)!;
    const props = lastCall[0] as Record<string, unknown>;
    expect(props.is_signed_in).toBe(false);
  });
});
