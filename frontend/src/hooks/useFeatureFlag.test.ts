import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import posthog from 'posthog-js';
import { useFeatureFlag } from './useFeatureFlag';

// ---------------------------------------------------------------------------
// Mock posthog-js
// ---------------------------------------------------------------------------
vi.mock('posthog-js', () => ({
  default: {
    __loaded: false,
    isFeatureEnabled: vi.fn(),
    onFeatureFlags: vi.fn(),
  },
}));

const mockPosthog = posthog as {
  __loaded: boolean;
  isFeatureEnabled: ReturnType<typeof vi.fn>;
  onFeatureFlags: ReturnType<typeof vi.fn>;
};

describe('useFeatureFlag', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: PostHog not loaded
    mockPosthog.__loaded = false;
    mockPosthog.isFeatureEnabled.mockReturnValue(undefined);
    mockPosthog.onFeatureFlags.mockReturnValue(() => {});
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  // -------------------------------------------------------------------------
  // Test 1: PostHog not initialized → default to true (dev/test fallback)
  // -------------------------------------------------------------------------
  it('returns { enabled: true } when posthog is not loaded (dev/test default)', () => {
    mockPosthog.__loaded = false;

    const { result } = renderHook(() => useFeatureFlag('some_flag'));

    expect(result.current.enabled).toBe(true);
    // isFeatureEnabled must NOT be called when PostHog is not loaded
    expect(mockPosthog.isFeatureEnabled).not.toHaveBeenCalled();
  });

  // -------------------------------------------------------------------------
  // Test 2: PostHog loaded, flag is true
  // -------------------------------------------------------------------------
  it('returns { enabled: true } when posthog is loaded and flag is on', () => {
    mockPosthog.__loaded = true;
    mockPosthog.isFeatureEnabled.mockReturnValue(true);
    // Simulate onFeatureFlags calling the callback synchronously
    mockPosthog.onFeatureFlags.mockImplementation((cb: () => void) => {
      cb();
      return () => {};
    });

    const { result } = renderHook(() => useFeatureFlag('my_flag'));

    expect(result.current.enabled).toBe(true);
  });

  // -------------------------------------------------------------------------
  // Test 3: PostHog loaded, flag is false
  // -------------------------------------------------------------------------
  it('returns { enabled: false } when posthog is loaded and flag is off', () => {
    mockPosthog.__loaded = true;
    mockPosthog.isFeatureEnabled.mockReturnValue(false);
    mockPosthog.onFeatureFlags.mockImplementation((cb: () => void) => {
      cb();
      return () => {};
    });

    const { result } = renderHook(() => useFeatureFlag('beta_invite_only'));

    expect(result.current.enabled).toBe(false);
  });

  // -------------------------------------------------------------------------
  // Test 4: PostHog loaded, flags not yet arrived → null (loading state)
  // -------------------------------------------------------------------------
  it('returns { enabled: null } when posthog is loaded but flags have not arrived', () => {
    mockPosthog.__loaded = true;
    // isFeatureEnabled returns undefined = not yet loaded
    mockPosthog.isFeatureEnabled.mockReturnValue(undefined);
    // onFeatureFlags registered but not yet called (flags in flight)
    mockPosthog.onFeatureFlags.mockReturnValue(() => {});

    const { result } = renderHook(() => useFeatureFlag('my_flag'));

    // Initial state from resolveFlag: PostHog loaded but isFeatureEnabled → undefined → null
    expect(result.current.enabled).toBeNull();
  });

  // -------------------------------------------------------------------------
  // Test 5: $feature_flag_called fires when onFeatureFlags callback runs
  //
  // posthog-js auto-emits $feature_flag_called inside isFeatureEnabled via
  // getFeatureFlagResult. The unit test verifies isFeatureEnabled is called
  // from within the subscription callback (the necessary SDK call-site that
  // triggers auto-emission). Direct assertion on $feature_flag_called is
  // encapsulated inside the SDK — the call-site is what we can assert here.
  // -------------------------------------------------------------------------
  it('calls isFeatureEnabled inside onFeatureFlags callback (triggers $feature_flag_called auto-emission)', () => {
    mockPosthog.__loaded = true;
    mockPosthog.isFeatureEnabled.mockReturnValue(true);

    let capturedCallback: (() => void) | null = null;
    mockPosthog.onFeatureFlags.mockImplementation((cb: () => void) => {
      capturedCallback = cb;
      return () => {};
    });

    renderHook(() => useFeatureFlag('beta_invite_only'));

    // Callback not yet called — isFeatureEnabled only called once from resolveFlag
    const callsAfterMount = mockPosthog.isFeatureEnabled.mock.calls.length;

    // Fire the subscription callback (simulates flags arriving from PostHog server)
    act(() => {
      capturedCallback?.();
    });

    // isFeatureEnabled is called again inside the callback (this is the call
    // that triggers $feature_flag_called auto-emission in the real SDK)
    expect(mockPosthog.isFeatureEnabled.mock.calls.length).toBeGreaterThan(callsAfterMount);
    expect(mockPosthog.isFeatureEnabled).toHaveBeenCalledWith('beta_invite_only');
  });

  // -------------------------------------------------------------------------
  // Test 6: onFeatureFlags callback registered only once per mount (not per render)
  // -------------------------------------------------------------------------
  it('registers onFeatureFlags subscription only once per hook mount', () => {
    mockPosthog.__loaded = true;
    mockPosthog.isFeatureEnabled.mockReturnValue(true);
    mockPosthog.onFeatureFlags.mockReturnValue(() => {});

    const { rerender } = renderHook(() => useFeatureFlag('my_flag'));

    // Subscription registered once on mount
    expect(mockPosthog.onFeatureFlags).toHaveBeenCalledTimes(1);

    // Re-render with same flagKey must not register a new subscription
    rerender();
    expect(mockPosthog.onFeatureFlags).toHaveBeenCalledTimes(1);
  });

  // -------------------------------------------------------------------------
  // Test 7: Cleanup — unsubscribe is called on unmount
  // -------------------------------------------------------------------------
  it('calls the unsubscribe function returned by onFeatureFlags on unmount', () => {
    mockPosthog.__loaded = true;
    mockPosthog.isFeatureEnabled.mockReturnValue(true);

    const unsubscribe = vi.fn();
    mockPosthog.onFeatureFlags.mockReturnValue(unsubscribe);

    const { unmount } = renderHook(() => useFeatureFlag('my_flag'));

    expect(unsubscribe).not.toHaveBeenCalled();

    unmount();

    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  // -------------------------------------------------------------------------
  // Test 8: flagKey change triggers re-subscription
  // -------------------------------------------------------------------------
  it('re-subscribes when flagKey changes', () => {
    mockPosthog.__loaded = true;
    mockPosthog.isFeatureEnabled.mockReturnValue(true);
    mockPosthog.onFeatureFlags.mockReturnValue(() => {});

    const { rerender } = renderHook(({ key }: { key: string }) => useFeatureFlag(key), {
      initialProps: { key: 'flag_a' },
    });

    expect(mockPosthog.onFeatureFlags).toHaveBeenCalledTimes(1);

    // Change the flagKey — useEffect dependency changes → cleanup + re-subscribe
    rerender({ key: 'flag_b' });
    expect(mockPosthog.onFeatureFlags).toHaveBeenCalledTimes(2);
  });

  // -------------------------------------------------------------------------
  // Test 9: window.__POSTHOG_TEST_FLAGS__ override — flag off
  //
  // When the test override is set to false, useFeatureFlag returns false
  // regardless of PostHog load state. This is the E2E determinism mechanism
  // that prevents CI failures when VITE_POSTHOG_KEY is absent.
  // -------------------------------------------------------------------------
  it('returns { enabled: false } when __POSTHOG_TEST_FLAGS__ overrides flag to false (PostHog not loaded)', () => {
    mockPosthog.__loaded = false;
    (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__ = { beta_invite_only: false };

    const { result } = renderHook(() => useFeatureFlag('beta_invite_only'));

    expect(result.current.enabled).toBe(false);
    // PostHog must not be consulted when the override is present
    expect(mockPosthog.isFeatureEnabled).not.toHaveBeenCalled();

    // Cleanup
    delete (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__;
  });

  // -------------------------------------------------------------------------
  // Test 10: window.__POSTHOG_TEST_FLAGS__ override — flag on
  //
  // When the test override is set to true, useFeatureFlag returns true
  // regardless of PostHog load state.
  // -------------------------------------------------------------------------
  it('returns { enabled: true } when __POSTHOG_TEST_FLAGS__ overrides flag to true (PostHog not loaded)', () => {
    mockPosthog.__loaded = false;
    (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__ = { my_flag: true };

    const { result } = renderHook(() => useFeatureFlag('my_flag'));

    expect(result.current.enabled).toBe(true);
    expect(mockPosthog.isFeatureEnabled).not.toHaveBeenCalled();

    // Cleanup
    delete (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__;
  });

  // -------------------------------------------------------------------------
  // Test 11: window.__POSTHOG_TEST_FLAGS__ override takes precedence over PostHog
  //
  // Even when PostHog is loaded and returns a different value, the test override
  // wins. This prevents staging-environment flag values from leaking into tests.
  // -------------------------------------------------------------------------
  it('__POSTHOG_TEST_FLAGS__ override takes precedence over posthog.isFeatureEnabled when PostHog is loaded', () => {
    mockPosthog.__loaded = true;
    // PostHog says flag is ON
    mockPosthog.isFeatureEnabled.mockReturnValue(true);
    mockPosthog.onFeatureFlags.mockImplementation((cb: () => void) => {
      cb();
      return () => {};
    });
    // But test override says flag is OFF
    (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__ = { beta_invite_only: false };

    const { result } = renderHook(() => useFeatureFlag('beta_invite_only'));

    // Override wins — flag is false despite PostHog returning true
    expect(result.current.enabled).toBe(false);

    // Cleanup
    delete (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__;
  });

  // -------------------------------------------------------------------------
  // Test 12: __POSTHOG_TEST_FLAGS__ only overrides keys that are present
  //
  // If a key is NOT in __POSTHOG_TEST_FLAGS__, useFeatureFlag falls through to
  // normal PostHog / not-loaded behaviour for that key.
  // -------------------------------------------------------------------------
  it('falls through to PostHog when the requested flag key is absent from __POSTHOG_TEST_FLAGS__', () => {
    mockPosthog.__loaded = true;
    mockPosthog.isFeatureEnabled.mockReturnValue(false);
    mockPosthog.onFeatureFlags.mockImplementation((cb: () => void) => {
      cb();
      return () => {};
    });
    // Override is present but for a DIFFERENT key
    (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__ = { other_flag: true };

    const { result } = renderHook(() => useFeatureFlag('beta_invite_only'));

    // PostHog value used because 'beta_invite_only' is not in the override
    expect(result.current.enabled).toBe(false);
    expect(mockPosthog.isFeatureEnabled).toHaveBeenCalledWith('beta_invite_only');

    // Cleanup
    delete (window as unknown as Record<string, unknown>).__POSTHOG_TEST_FLAGS__;
  });
});
