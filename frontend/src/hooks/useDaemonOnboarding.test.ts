import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDaemonOnboarding } from './useDaemonOnboarding';

const STORAGE_KEY = 'vaultmtg_onboarding_dismissed';
const STORAGE_COMPLETED_KEY = 'vaultmtg_onboarding_completed';

describe('useDaemonOnboarding', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  describe('auto-show behavior', () => {
    it('shows modal when signed in and daemon is disconnected and not seen before', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );
      expect(result.current.isOpen).toBe(true);
    });

    it('does NOT show modal when daemon is connected', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('connected', true)
      );
      expect(result.current.isOpen).toBe(false);
    });

    it('does NOT show modal when daemon is loading (give it time to resolve)', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('loading', true)
      );
      expect(result.current.isOpen).toBe(false);
    });

    it('does NOT show modal when daemon is in error state', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('error', true)
      );
      expect(result.current.isOpen).toBe(false);
    });

    it('does NOT show modal when user is NOT signed in', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', false)
      );
      expect(result.current.isOpen).toBe(false);
    });

    it('does NOT show modal when previously dismissed', () => {
      localStorage.setItem(STORAGE_KEY, 'true');
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );
      expect(result.current.isOpen).toBe(false);
    });

    it('does NOT show modal when previously completed', () => {
      localStorage.setItem(STORAGE_COMPLETED_KEY, 'true');
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );
      expect(result.current.isOpen).toBe(false);
    });
  });

  describe('hasSeenOnboarding', () => {
    it('is false initially when no localStorage entry', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('connected', true)
      );
      expect(result.current.hasSeenOnboarding).toBe(false);
    });

    it('is true when dismissed key is set in localStorage', () => {
      localStorage.setItem(STORAGE_KEY, 'true');
      const { result } = renderHook(() =>
        useDaemonOnboarding('connected', true)
      );
      expect(result.current.hasSeenOnboarding).toBe(true);
    });

    it('is true when completed key is set in localStorage', () => {
      localStorage.setItem(STORAGE_COMPLETED_KEY, 'true');
      const { result } = renderHook(() =>
        useDaemonOnboarding('connected', true)
      );
      expect(result.current.hasSeenOnboarding).toBe(true);
    });
  });

  describe('open()', () => {
    it('opens the modal regardless of daemon status', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('connected', true)
      );
      expect(result.current.isOpen).toBe(false);

      act(() => {
        result.current.open();
      });

      expect(result.current.isOpen).toBe(true);
    });

    it('can re-open after dismiss', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );

      act(() => {
        result.current.dismiss();
      });
      expect(result.current.isOpen).toBe(false);

      act(() => {
        result.current.open();
      });
      expect(result.current.isOpen).toBe(true);
    });
  });

  describe('dismiss()', () => {
    it('closes the modal', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );
      expect(result.current.isOpen).toBe(true);

      act(() => {
        result.current.dismiss();
      });

      expect(result.current.isOpen).toBe(false);
    });

    it('sets hasSeenOnboarding to true', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );

      act(() => {
        result.current.dismiss();
      });

      expect(result.current.hasSeenOnboarding).toBe(true);
    });

    it('persists dismissed state to localStorage', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );

      act(() => {
        result.current.dismiss();
      });

      expect(localStorage.getItem(STORAGE_KEY)).toBe('true');
    });

    it('prevents modal from re-appearing when daemon is still disconnected', () => {
      const { result, rerender } = renderHook(
        ({ status, signed }: { status: 'disconnected' | 'connected'; signed: boolean }) =>
          useDaemonOnboarding(status, signed),
        { initialProps: { status: 'disconnected' as const, signed: true } }
      );

      act(() => {
        result.current.dismiss();
      });

      // Re-render with same disconnected status — modal should stay closed
      rerender({ status: 'disconnected', signed: true });
      expect(result.current.isOpen).toBe(false);
    });
  });

  describe('complete()', () => {
    it('closes the modal', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );

      act(() => {
        result.current.complete();
      });

      expect(result.current.isOpen).toBe(false);
    });

    it('sets hasSeenOnboarding to true', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );

      act(() => {
        result.current.complete();
      });

      expect(result.current.hasSeenOnboarding).toBe(true);
    });

    it('persists completed state to localStorage', () => {
      const { result } = renderHook(() =>
        useDaemonOnboarding('disconnected', true)
      );

      act(() => {
        result.current.complete();
      });

      expect(localStorage.getItem(STORAGE_COMPLETED_KEY)).toBe('true');
      expect(localStorage.getItem(STORAGE_KEY)).toBe('true');
    });
  });
});
