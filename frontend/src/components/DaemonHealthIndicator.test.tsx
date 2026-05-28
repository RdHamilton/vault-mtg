import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import DaemonHealthIndicator from './DaemonHealthIndicator';

// ---------------------------------------------------------------------------
// Mock @clerk/react
// ---------------------------------------------------------------------------
const mockGetToken = vi.fn(() => Promise.resolve('clerk-test-token'));
vi.mock('@clerk/react', () => ({
  useAuth: () => ({
    isLoaded: true,
    isSignedIn: true,
    getToken: mockGetToken,
  }),
}));

// ---------------------------------------------------------------------------
// Mock the BFF health adapter
// ---------------------------------------------------------------------------
const mockGetDaemonHealth = vi.fn();
vi.mock('@/services/api/bffHealth', () => ({
  getDaemonHealth: (...args: unknown[]) => mockGetDaemonHealth(...args),
}));

// ---------------------------------------------------------------------------
// Mock analytics
// ---------------------------------------------------------------------------
const mockTrackEvent = vi.fn();
vi.mock('@/services/analytics', () => ({
  trackEvent: (...args: unknown[]) => mockTrackEvent(...args),
}));

describe('DaemonHealthIndicator', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
    vi.clearAllMocks();
    mockGetToken.mockResolvedValue('clerk-test-token');
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  it('renders the dot element with data-testid', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    expect(screen.getByTestId('daemon-health-indicator')).toBeInTheDocument();
  });

  it('shows green dot (connected class) when API returns connected', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.classList.contains('daemon-health-connected')).toBe(true);
  });

  it('shows red dot (disconnected class) when API returns disconnected', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'disconnected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.classList.contains('daemon-health-disconnected')).toBe(true);
  });

  it('shows gray dot (error class) when API throws', async () => {
    mockGetDaemonHealth.mockRejectedValue(new Error('Network error'));

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.classList.contains('daemon-health-error')).toBe(true);
  });

  it('shows gray dot while loading (initial state before first fetch)', () => {
    // Make the fetch never resolve during this check
    mockGetDaemonHealth.mockReturnValue(new Promise(() => {}));

    render(<DaemonHealthIndicator />);

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.classList.contains('daemon-health-loading')).toBe(true);
  });

  it('shows tooltip "Daemon connected" when connected', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.getAttribute('title')).toBe('Daemon connected');
  });

  it('shows tooltip "Daemon not connected — data may be stale" when disconnected', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'disconnected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.getAttribute('title')).toBe('Daemon not connected — data may be stale');
  });

  it('shows yellow dot (reconnecting class) when API returns reconnecting', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'reconnecting' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.classList.contains('daemon-health-reconnecting')).toBe(true);
  });

  it('shows tooltip "Daemon reconnecting..." when state is reconnecting', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'reconnecting' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.getAttribute('title')).toBe('Daemon reconnecting...');
  });

  it('shows tooltip "Checking..." while loading', () => {
    mockGetDaemonHealth.mockReturnValue(new Promise(() => {}));

    render(<DaemonHealthIndicator />);

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.getAttribute('title')).toBe('Checking...');
  });

  it('shows tooltip "Checking..." on error', async () => {
    mockGetDaemonHealth.mockRejectedValue(new Error('fail'));

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.getAttribute('title')).toBe('Checking...');
  });

  it('polls again after 30 seconds', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    expect(mockGetDaemonHealth).toHaveBeenCalledTimes(1);

    // Advance timer by 30s and flush promises
    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });

    expect(mockGetDaemonHealth).toHaveBeenCalledTimes(2);
  });

  it('cleans up interval on unmount (no memory leak)', async () => {
    mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });

    let unmount!: () => void;
    await act(async () => {
      const result = render(<DaemonHealthIndicator />);
      unmount = result.unmount;
    });

    expect(mockGetDaemonHealth).toHaveBeenCalledTimes(1);

    unmount();

    // Advance 30s — should NOT trigger another call after unmount
    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });

    expect(mockGetDaemonHealth).toHaveBeenCalledTimes(1);
  });

  it('shows gray dot (error) when no token is returned', async () => {
    mockGetToken.mockResolvedValue(null);
    mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const dot = screen.getByTestId('daemon-health-indicator');
    expect(dot.classList.contains('daemon-health-error')).toBe(true);

    // Adapter should NOT be called when there's no token
    expect(mockGetDaemonHealth).not.toHaveBeenCalled();
  });
});

// ── error_daemon_connection_failed analytics ──────────────────────────────────

describe('DaemonHealthIndicator — error_daemon_connection_failed', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
    vi.clearAllMocks();
    mockGetToken.mockResolvedValue('clerk-test-token');
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  it('fires error_daemon_connection_failed when transitioning from connected to disconnected', async () => {
    mockGetDaemonHealth
      .mockResolvedValueOnce({ status: 'connected' })
      .mockResolvedValueOnce({ status: 'disconnected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    // Advance to second poll
    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_daemon_connection_failed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0].properties.previous_status).toBe('connected');
    expect(typeof calls[0][0].properties.duration_connected_seconds).toBe('number');
    expect(calls[0][0].properties.duration_connected_seconds).toBeGreaterThanOrEqual(0);
  });

  it('fires error_daemon_connection_failed with previous_status reconnecting when transitioning from reconnecting to disconnected', async () => {
    mockGetDaemonHealth
      .mockResolvedValueOnce({ status: 'reconnecting' })
      .mockResolvedValueOnce({ status: 'disconnected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_daemon_connection_failed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0].properties.previous_status).toBe('reconnecting');
  });

  it('does NOT fire error_daemon_connection_failed when daemon is stably disconnected (never connected)', async () => {
    mockGetDaemonHealth
      .mockResolvedValueOnce({ status: 'disconnected' })
      .mockResolvedValueOnce({ status: 'disconnected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_daemon_connection_failed',
    );
    expect(calls).toHaveLength(0);
  });

  it('does NOT fire error_daemon_connection_failed when transitioning from loading to disconnected (initial state)', async () => {
    // Initial state is loading; first poll returns disconnected — should not fire
    mockGetDaemonHealth.mockResolvedValueOnce({ status: 'disconnected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_daemon_connection_failed',
    );
    expect(calls).toHaveLength(0);
  });

  it('does NOT fire error_daemon_connection_failed when transitioning connected→connected (stable)', async () => {
    mockGetDaemonHealth
      .mockResolvedValueOnce({ status: 'connected' })
      .mockResolvedValueOnce({ status: 'connected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_daemon_connection_failed',
    );
    expect(calls).toHaveLength(0);
  });

  it('NEGATIVE: error_daemon_connection_failed payload never contains user_id', async () => {
    mockGetDaemonHealth
      .mockResolvedValueOnce({ status: 'connected' })
      .mockResolvedValueOnce({ status: 'disconnected' });

    await act(async () => {
      render(<DaemonHealthIndicator />);
    });

    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_daemon_connection_failed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0].properties).not.toHaveProperty('user_id');
  });
});
