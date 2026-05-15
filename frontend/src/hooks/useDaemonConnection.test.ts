import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useDaemonConnection } from './useDaemonConnection';

// Mock Clerk useAuth
vi.mock('@clerk/react', () => ({
  useAuth: vi.fn(),
}));

// Mock the BFF health adapter
vi.mock('@/services/api/bffHealth', () => ({
  getDaemonHealth: vi.fn(),
}));

// Mock showToast
vi.mock('../components/ToastContainer', () => ({
  showToast: {
    show: vi.fn(),
  },
}));

import { useAuth } from '@clerk/react';
import { getDaemonHealth } from '@/services/api/bffHealth';
import { showToast } from '../components/ToastContainer';

const mockUseAuth = vi.mocked(useAuth);
const mockGetDaemonHealth = vi.mocked(getDaemonHealth);

/** Helper: set up Clerk mock as signed-in with a valid token. */
function signedInAuth(token = 'test-token') {
  mockUseAuth.mockReturnValue({
    getToken: vi.fn().mockResolvedValue(token),
    isSignedIn: true,
  } as unknown as ReturnType<typeof useAuth>);
}

/** Helper: set up Clerk mock as signed-out. */
function signedOutAuth() {
  mockUseAuth.mockReturnValue({
    getToken: vi.fn().mockResolvedValue(null),
    isSignedIn: false,
  } as unknown as ReturnType<typeof useAuth>);
}

describe('useDaemonConnection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: signed in, BFF returns disconnected.
    signedInAuth();
    mockGetDaemonHealth.mockResolvedValue({ status: 'disconnected' });
  });

  describe('initial state', () => {
    it('returns default connection status', () => {
      const { result } = renderHook(() => useDaemonConnection());

      expect(result.current.connectionStatus.status).toBe('standalone');
      expect(result.current.connectionStatus.connected).toBe(false);
      expect(result.current.connectionStatus.mode).toBe('standalone');
      expect(result.current.connectionStatus.port).toBe(9999);
    });

    it('returns default daemon mode', () => {
      const { result } = renderHook(() => useDaemonConnection());

      expect(result.current.daemonMode).toBe('auto');
    });

    it('returns default daemon port', () => {
      const { result } = renderHook(() => useDaemonConnection());

      expect(result.current.daemonPort).toBe(9999);
    });

    it('returns isReconnecting as false', () => {
      const { result } = renderHook(() => useDaemonConnection());

      expect(result.current.isReconnecting).toBe(false);
    });
  });

  // AC2 (#2020): useDaemonConnection uses the same BFF endpoint as DaemonHealthIndicator
  // regardless of desktop/browser context — no isDesktopApp() guard.
  describe('polls BFF health in all contexts — single source of truth (#2020)', () => {
    it('calls getDaemonHealth on mount when signed in (browser context)', async () => {
      // Browser context — previously this was gated by isDesktopApp(); now it must fire.
      mockGetDaemonHealth.mockResolvedValueOnce({ status: 'connected' });

      const { result } = renderHook(() => useDaemonConnection());

      await waitFor(() => {
        expect(mockGetDaemonHealth).toHaveBeenCalledWith('test-token');
      });

      expect(result.current.connectionStatus.status).toBe('connected');
      expect(result.current.connectionStatus.connected).toBe(true);
    });

    it('reflects connected status from BFF and sets connected=true', async () => {
      mockGetDaemonHealth.mockResolvedValueOnce({ status: 'connected' });

      const { result } = renderHook(() => useDaemonConnection());

      await waitFor(() => {
        expect(mockGetDaemonHealth).toHaveBeenCalled();
      });

      expect(result.current.connectionStatus.status).toBe('connected');
      expect(result.current.connectionStatus.connected).toBe(true);
      expect(result.current.connectionStatus.mode).toBe('daemon');
    });

    it('reflects disconnected status from BFF', async () => {
      mockGetDaemonHealth.mockResolvedValueOnce({ status: 'disconnected' });

      const { result } = renderHook(() => useDaemonConnection());

      await waitFor(() => {
        expect(mockGetDaemonHealth).toHaveBeenCalled();
      });

      expect(result.current.connectionStatus.connected).toBe(false);
    });

    it('handles BFF health error silently and keeps default state', async () => {
      mockGetDaemonHealth.mockRejectedValueOnce(new Error('network error'));

      const { result } = renderHook(() => useDaemonConnection());

      await waitFor(() => {
        expect(mockGetDaemonHealth).toHaveBeenCalled();
      });

      // On error we keep the default state (standalone/not-connected).
      expect(result.current.connectionStatus.status).toBe('standalone');
    });

    it('does NOT call getDaemonHealth when signed out', async () => {
      signedOutAuth();

      renderHook(() => useDaemonConnection());

      await act(async () => {
        await Promise.resolve();
      });

      expect(mockGetDaemonHealth).not.toHaveBeenCalled();
    });
  });

  describe('handleDaemonPortChange', () => {
    it('updates daemon port state', async () => {
      const { result } = renderHook(() => useDaemonConnection());

      await act(async () => {
        await result.current.handleDaemonPortChange(8080);
      });

      expect(result.current.daemonPort).toBe(8080);
    });

    it('rejects ports below 1024', async () => {
      const { result } = renderHook(() => useDaemonConnection());
      const initialPort = result.current.daemonPort;

      await act(async () => {
        await result.current.handleDaemonPortChange(1000);
      });

      expect(result.current.daemonPort).toBe(initialPort);
    });

    it('rejects ports above 65535', async () => {
      const { result } = renderHook(() => useDaemonConnection());
      const initialPort = result.current.daemonPort;

      await act(async () => {
        await result.current.handleDaemonPortChange(70000);
      });

      expect(result.current.daemonPort).toBe(initialPort);
    });
  });

  describe('handleReconnect', () => {
    it('sets isReconnecting to true during reconnection', async () => {
      const { result } = renderHook(() => useDaemonConnection());

      let reconnectPromise: Promise<void>;
      act(() => {
        reconnectPromise = result.current.handleReconnect();
      });

      expect(result.current.isReconnecting).toBe(true);

      await act(async () => {
        await reconnectPromise;
      });

      expect(result.current.isReconnecting).toBe(false);
    });

    it('shows success toast on successful reconnect', async () => {
      const { result } = renderHook(() => useDaemonConnection());

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(showToast.show).toHaveBeenCalledWith(
        'Successfully reconnected to daemon',
        'success'
      );
    });
  });

  describe('handleModeChange', () => {
    it('updates daemon mode state', async () => {
      const { result } = renderHook(() => useDaemonConnection());

      await act(async () => {
        await result.current.handleModeChange('standalone');
      });

      expect(result.current.daemonMode).toBe('standalone');
    });

    it('does not fail for auto mode', async () => {
      const { result } = renderHook(() => useDaemonConnection());

      await act(async () => {
        await result.current.handleModeChange('auto');
      });

      expect(result.current.daemonMode).toBe('auto');
    });

    it('shows success toast for standalone mode switch', async () => {
      const { result } = renderHook(() => useDaemonConnection());

      await act(async () => {
        await result.current.handleModeChange('standalone');
      });

      expect(showToast.show).toHaveBeenCalledWith(
        'Switched to standalone mode',
        'success'
      );
    });

    it('shows success toast for daemon mode switch', async () => {
      const { result } = renderHook(() => useDaemonConnection());

      await act(async () => {
        await result.current.handleModeChange('daemon');
      });

      expect(showToast.show).toHaveBeenCalledWith(
        'Switched to daemon mode',
        'success'
      );
    });
  });
});
