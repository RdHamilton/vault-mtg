import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, act } from '@testing-library/react';
import { SseInitializer } from './SseInitializer';

// Override the global Clerk mock from setup.ts with a controllable vi.fn() so
// individual tests can drive isLoaded / isSignedIn independently.
const mockUseAuth = vi.fn();
vi.mock('@clerk/react', async (importOriginal) => {
  const original = await importOriginal<typeof import('@clerk/react')>();
  return {
    ...original,
    useAuth: () => mockUseAuth(),
  };
});

vi.mock('../services/adapter', () => ({
  initializeSse: vi.fn().mockResolvedValue(undefined),
  disconnectSse: vi.fn(),
}));

import { initializeSse, disconnectSse } from '../services/adapter';

describe('SseInitializer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not call initializeSse when Clerk is not yet loaded', () => {
    mockUseAuth.mockReturnValue({ isLoaded: false, isSignedIn: false });
    render(<SseInitializer />);
    expect(initializeSse).not.toHaveBeenCalled();
  });

  it('does not call initializeSse when loaded but signed out', () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    render(<SseInitializer />);
    expect(initializeSse).not.toHaveBeenCalled();
  });

  it('calls initializeSse once when isLoaded && isSignedIn', async () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    render(<SseInitializer />);
    await vi.waitFor(() => {
      expect(initializeSse).toHaveBeenCalledTimes(1);
    });
  });

  it('calls disconnectSse on unmount after a successful sign-in', async () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    const { unmount } = render(<SseInitializer />);
    await vi.waitFor(() => expect(initializeSse).toHaveBeenCalled());
    unmount();
    expect(disconnectSse).toHaveBeenCalledTimes(1);
  });

  it('calls disconnectSse when the user signs out', async () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    const { rerender } = render(<SseInitializer />);
    await vi.waitFor(() => expect(initializeSse).toHaveBeenCalledTimes(1));

    // Simulate sign-out
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: false });
    act(() => { rerender(<SseInitializer />); });

    expect(disconnectSse).toHaveBeenCalled();
  });

  it('does not call initializeSse a second time when re-rendered without auth state change', async () => {
    mockUseAuth.mockReturnValue({ isLoaded: true, isSignedIn: true });
    const { rerender } = render(<SseInitializer />);
    await vi.waitFor(() => expect(initializeSse).toHaveBeenCalledTimes(1));

    rerender(<SseInitializer />);
    expect(initializeSse).toHaveBeenCalledTimes(1);
  });
});
