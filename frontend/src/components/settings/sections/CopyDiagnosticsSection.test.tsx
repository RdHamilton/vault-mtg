import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { CopyDiagnosticsSection } from './CopyDiagnosticsSection';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const mockDiagnosticsData = {
  daemon_version: '0.3.6',
  os: 'darwin',
  arch: 'arm64',
  uptime_seconds: 3600,
  started_at: '2026-05-31T10:00:00Z',
  cloud_api_url: 'https://api.vaultmtg.app',
  session_id: 'sess_test123',
  log_path: '/Users/test/Library/Logs/vaultmtg-daemon.log',
  log_tail: ['2026-05-31T10:00:00Z INFO daemon started', '2026-05-31T11:00:00Z INFO heartbeat ok'],
};

function makeFetchSuccess(data = mockDiagnosticsData): typeof fetch {
  return vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(data),
  } as Response);
}

function makeFetchHttpError(status = 503): typeof fetch {
  return vi.fn().mockResolvedValue({
    ok: false,
    status,
    json: () => Promise.resolve({}),
  } as Response);
}

function makeFetchNetworkError(): typeof fetch {
  return vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));
}

// ---------------------------------------------------------------------------
// AC5: Vitest component tests
// ---------------------------------------------------------------------------

describe('CopyDiagnosticsSection', () => {
  let clipboardWriteFn: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    clipboardWriteFn = vi.fn().mockResolvedValue(undefined);
  });

  // ── AC5a: button render ──────────────────────────────────────────────────

  it('renders the section with the Copy Diagnostics button', () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchSuccess()}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    expect(screen.getByTestId('copy-diagnostics-section')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /copy diagnostics/i })).toBeInTheDocument();
  });

  it('renders the section title', () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchSuccess()}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    expect(screen.getByRole('heading', { name: /copy diagnostics/i })).toBeInTheDocument();
  });

  it('renders the description text', () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchSuccess()}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    expect(screen.getByText(/support bundle/i)).toBeInTheDocument();
  });

  // ── AC5b: click triggers fetch ───────────────────────────────────────────

  it('calls fetchFn with the correct diagnostics URL when button is clicked', async () => {
    const fetchFn = makeFetchSuccess();
    render(
      <CopyDiagnosticsSection fetchFn={fetchFn} clipboardWriteFn={clipboardWriteFn} />,
    );

    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));

    await waitFor(() => {
      expect(fetchFn).toHaveBeenCalledWith('http://127.0.0.1:9001/api/v1/system/diagnostics');
    });
  });

  it('shows loading text while fetching', async () => {
    // fetchFn that never resolves — button stays in loading state
    const fetchFn = vi.fn().mockReturnValue(new Promise(() => {})) as unknown as typeof fetch;
    render(
      <CopyDiagnosticsSection fetchFn={fetchFn} clipboardWriteFn={clipboardWriteFn} />,
    );

    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));

    await waitFor(() => {
      expect(screen.getByText(/fetching diagnostics/i)).toBeInTheDocument();
    });
  });

  // ── AC5c: clipboard write asserted ──────────────────────────────────────

  it('writes formatted diagnostics to clipboard on success', async () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchSuccess()}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));

    await waitFor(() => {
      expect(clipboardWriteFn).toHaveBeenCalledTimes(1);
    });

    const written: string = clipboardWriteFn.mock.calls[0][0];
    expect(written).toContain('VaultMTG Diagnostics');
    expect(written).toContain('0.3.6'); // daemon_version
    expect(written).toContain('darwin'); // os
    expect(written).toContain('arm64'); // arch
  });

  it('includes log tail lines in the clipboard text', async () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchSuccess()}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));

    await waitFor(() => {
      expect(clipboardWriteFn).toHaveBeenCalledTimes(1);
    });

    const written: string = clipboardWriteFn.mock.calls[0][0];
    expect(written).toContain('daemon started');
    expect(written).toContain('heartbeat ok');
  });

  it('does NOT include session_id block when the field is absent', async () => {
    const dataWithoutSession = { ...mockDiagnosticsData, session_id: undefined };
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchSuccess(dataWithoutSession)}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));

    await waitFor(() => {
      expect(clipboardWriteFn).toHaveBeenCalledTimes(1);
    });

    const written: string = clipboardWriteFn.mock.calls[0][0];
    expect(written).not.toContain('Session ID');
  });

  // ── AC5d: daemon-down error state ────────────────────────────────────────

  it('shows a user-friendly error when the daemon is not running (TypeError)', async () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchNetworkError()}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));

    await waitFor(() => {
      expect(screen.getByTestId('copy-diagnostics-error')).toBeInTheDocument();
    });

    expect(screen.getByText(/daemon is not running/i)).toBeInTheDocument();
  });

  it('shows an HTTP error message when the daemon returns non-200', async () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchHttpError(503)}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));

    await waitFor(() => {
      expect(screen.getByTestId('copy-diagnostics-error')).toBeInTheDocument();
    });

    expect(screen.getByText(/503/)).toBeInTheDocument();
  });

  it('does not render the error element before any click', () => {
    render(
      <CopyDiagnosticsSection
        fetchFn={makeFetchSuccess()}
        clipboardWriteFn={clipboardWriteFn}
      />,
    );

    expect(screen.queryByTestId('copy-diagnostics-error')).not.toBeInTheDocument();
  });

  it('clears previous error on a subsequent successful fetch', async () => {
    // First call fails, second call succeeds
    const fetchFn = vi
      .fn()
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockDiagnosticsData),
      } as Response);

    render(
      <CopyDiagnosticsSection fetchFn={fetchFn} clipboardWriteFn={clipboardWriteFn} />,
    );

    // First click — expect error
    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));
    await waitFor(() => {
      expect(screen.getByTestId('copy-diagnostics-error')).toBeInTheDocument();
    });

    // Second click — error should clear
    fireEvent.click(screen.getByRole('button', { name: /copy diagnostics/i }));
    await waitFor(() => {
      expect(screen.queryByTestId('copy-diagnostics-error')).not.toBeInTheDocument();
    });
  });
});
