import { expect, userEvent, within } from 'storybook/test';
import type { Meta, StoryObj } from '@storybook/react';
import { CopyDiagnosticsSection } from './CopyDiagnosticsSection';

/**
 * CopyDiagnosticsSection — Settings accordion section that lets users copy a
 * support diagnostics bundle to their clipboard for pasting into Discord or
 * GitHub issues.
 *
 * Fetches from the daemon's local-API endpoint (127.0.0.1:9001) via the
 * `fetchFn` prop (REST API adapter pattern) so stories exercise every state
 * without a live daemon process.
 *
 * The `clipboardWriteFn` prop is injected so stories can assert the written
 * text without touching the real clipboard API.
 */

const mockDiagnostics = {
  daemon_version: '0.3.6',
  os: 'darwin',
  arch: 'arm64',
  uptime_seconds: 7200,
  started_at: '2026-05-31T10:00:00Z',
  cloud_api_url: 'https://api.vaultmtg.app',
  session_id: 'sess_storybook_mock',
  log_path: '/Users/planeswalker/Library/Logs/vaultmtg-daemon.log',
  log_tail: [
    '2026-05-31T10:00:00Z INFO  daemon started version=0.3.6',
    '2026-05-31T10:00:01Z INFO  localapi listening addr=127.0.0.1:9001',
    '2026-05-31T11:59:59Z INFO  heartbeat ok latency_ms=14',
  ],
};

function makeFetch(data = mockDiagnostics): typeof fetch {
  return () =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve(data),
    } as Response);
}

function makeSlowFetch(delayMs = 1500): typeof fetch {
  return () =>
    new Promise<Response>((resolve) =>
      setTimeout(
        () =>
          resolve({
            ok: true,
            json: () => Promise.resolve(mockDiagnostics),
          } as Response),
        delayMs,
      ),
    );
}

function makeErrorFetch(message = 'Failed to fetch'): typeof fetch {
  return () => Promise.reject(new TypeError(message));
}

function makeHttpErrorFetch(status = 503): typeof fetch {
  return () =>
    Promise.resolve({
      ok: false,
      status,
      json: () => Promise.resolve({}),
    } as Response);
}

const meta: Meta<typeof CopyDiagnosticsSection> = {
  title: 'Organisms/CopyDiagnosticsSection',
  component: CopyDiagnosticsSection,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof CopyDiagnosticsSection>;

/**
 * Idle state — daemon reachable, clipboard write succeeds immediately.
 */
export const Default: Story = {
  args: {
    fetchFn: makeFetch(),
    clipboardWriteFn: () => Promise.resolve(),
  },
};

/**
 * Play function: clicks "Copy Diagnostics" and verifies no error is shown.
 * Chromatic snapshots the button post-click (success path).
 */
export const CopySuccess: Story = {
  args: {
    fetchFn: makeFetch(),
    clipboardWriteFn: () => Promise.resolve(),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const btn = canvas.getByRole('button', { name: /copy diagnostics/i });
    await userEvent.click(btn);

    // After the async operations, no error should be visible.
    await expect(canvas.queryByTestId('copy-diagnostics-error')).toBeNull();
  },
};

/**
 * Loading state — fetch is artificially slow so Chromatic can snapshot the
 * in-progress spinner.
 */
export const Loading: Story = {
  args: {
    fetchFn: makeSlowFetch(30_000),
    clipboardWriteFn: () => Promise.resolve(),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const btn = canvas.getByRole('button', { name: /copy diagnostics/i });
    await userEvent.click(btn);

    await expect(
      await canvas.findByText(/fetching diagnostics/i),
    ).toBeInTheDocument();
  },
};

/**
 * Daemon offline — fetch rejects with a TypeError (network error).
 * Shows the "Daemon is not running" error banner.
 */
export const DaemonDown: Story = {
  args: {
    fetchFn: makeErrorFetch(),
    clipboardWriteFn: () => Promise.resolve(),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const btn = canvas.getByRole('button', { name: /copy diagnostics/i });
    await userEvent.click(btn);

    await expect(
      await canvas.findByTestId('copy-diagnostics-error'),
    ).toBeInTheDocument();

    await expect(
      await canvas.findByText(/daemon is not running/i),
    ).toBeInTheDocument();
  },
};

/**
 * Daemon returned a non-200 HTTP status.
 */
export const HttpError: Story = {
  args: {
    fetchFn: makeHttpErrorFetch(503),
    clipboardWriteFn: () => Promise.resolve(),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole('button', { name: /copy diagnostics/i }));

    await expect(await canvas.findByTestId('copy-diagnostics-error')).toBeInTheDocument();
    await expect(await canvas.findByText(/503/)).toBeInTheDocument();
  },
};
