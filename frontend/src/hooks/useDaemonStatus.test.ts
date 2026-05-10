/**
 * useDaemonStatus hook tests
 *
 * Verifies daemon connectivity detection from the BFF health endpoint.
 *
 * NOTE: setup.ts globally mocks @/hooks/useDaemonStatus — this test file
 * tests the real implementation by unregistering the global mock and
 * controlling the bffHealth dependency via its own vi.mock call.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';

// Unmock the hook itself so we test the real implementation
vi.unmock('@/hooks/useDaemonStatus');

// Mock the BFF health adapter (overrides the global setup.ts mock for this test file)
vi.mock('@/services/api/bffHealth', () => ({
  getDaemonHealth: vi.fn(),
}));

// Import after mocking is declared
import { useDaemonStatus } from './useDaemonStatus';
import { getDaemonHealth } from '@/services/api/bffHealth';

const mockGetDaemonHealth = getDaemonHealth as ReturnType<typeof vi.fn>;

describe('useDaemonStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts with daemonChecked=false and daemonConnected=false', () => {
    // Resolve never so the hook stays in its initial state
    mockGetDaemonHealth.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useDaemonStatus());
    expect(result.current.daemonChecked).toBe(false);
    expect(result.current.daemonConnected).toBe(false);
  });

  it('sets daemonConnected=true and daemonChecked=true when daemon returns connected', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });
    const { result } = renderHook(() => useDaemonStatus());

    await waitFor(() => {
      expect(result.current.daemonChecked).toBe(true);
    }, { timeout: 3000 });

    expect(result.current.daemonConnected).toBe(true);
  });

  it('sets daemonConnected=false when daemon returns disconnected', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'disconnected' });
    const { result } = renderHook(() => useDaemonStatus());

    await waitFor(() => {
      expect(result.current.daemonChecked).toBe(true);
    }, { timeout: 3000 });

    expect(result.current.daemonConnected).toBe(false);
  });

  it('sets daemonConnected=false when daemon returns reconnecting', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'reconnecting' });
    const { result } = renderHook(() => useDaemonStatus());

    await waitFor(() => {
      expect(result.current.daemonChecked).toBe(true);
    }, { timeout: 3000 });

    expect(result.current.daemonConnected).toBe(false);
  });

  it('sets daemonConnected=false and daemonChecked=true on network error', async () => {
    mockGetDaemonHealth.mockRejectedValue(new Error('Connection refused'));
    const { result } = renderHook(() => useDaemonStatus());

    await waitFor(() => {
      expect(result.current.daemonChecked).toBe(true);
    }, { timeout: 3000 });

    expect(result.current.daemonConnected).toBe(false);
  });
});
