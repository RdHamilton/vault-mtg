/**
 * Setup Page — Component Tests
 *
 * Covers: #1644 (install warnings), #1645 (PKCE pairing states)
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Setup from './Setup';

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderSetup() {
  return render(
    <MemoryRouter>
      <Setup />
    </MemoryRouter>
  );
}

function setNavigatorPlatform(platform: string, ua: string) {
  Object.defineProperty(navigator, 'platform', { value: platform, configurable: true });
  Object.defineProperty(navigator, 'userAgent', { value: ua, configurable: true });
}

// ---------------------------------------------------------------------------
// Suites
// ---------------------------------------------------------------------------

describe('Setup — page structure', () => {
  beforeEach(() => {
    // Default: daemon never responds
    global.fetch = vi.fn(() => Promise.reject(new Error('daemon not running')));
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('renders the main heading', () => {
    renderSetup();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Install the VaultMTG Daemon'
    );
  });

  it('renders the setup container', () => {
    renderSetup();
    expect(screen.getByTestId('setup-container')).toBeInTheDocument();
  });

  it('renders Step 1 (Download) section', () => {
    renderSetup();
    expect(screen.getByTestId('setup-download-section')).toBeInTheDocument();
  });

  it('renders Step 2 (Warnings) section', () => {
    renderSetup();
    expect(screen.getByTestId('setup-warnings-section')).toBeInTheDocument();
  });

  it('renders Step 3 (Pairing) section', () => {
    renderSetup();
    expect(screen.getByTestId('setup-pairing-section')).toBeInTheDocument();
  });

  it('renders a link to the download page', () => {
    renderSetup();
    const link = screen.getByTestId('download-page-link');
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', '/download');
  });

  it('explains why unsigned beta is normal for indie software', () => {
    renderSetup();
    const container = screen.getByTestId('setup-container');
    expect(container.textContent).toMatch(/indie beta software/i);
  });
});

// ---------------------------------------------------------------------------
// Platform: macOS
// ---------------------------------------------------------------------------

describe('Setup — macOS platform', () => {
  beforeEach(() => {
    global.fetch = vi.fn(() => Promise.reject(new Error('daemon not running')));
    vi.useFakeTimers();
    setNavigatorPlatform('MacIntel', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)');
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows Gatekeeper warning on macOS', () => {
    renderSetup();
    expect(screen.getByTestId('gatekeeper-warning')).toBeInTheDocument();
  });

  it('Gatekeeper warning contains right-click instruction', () => {
    renderSetup();
    const section = screen.getByTestId('gatekeeper-warning');
    expect(section).toHaveTextContent(/right-click/i);
  });

  it('Gatekeeper warning contains Open Anyway instruction', () => {
    renderSetup();
    const section = screen.getByTestId('gatekeeper-warning');
    expect(section).toHaveTextContent(/open anyway/i);
  });

  it('SmartScreen warning is in a collapsed details element on macOS', () => {
    renderSetup();
    expect(screen.getByTestId('smartscreen-details')).toBeInTheDocument();
  });

  it('does not render top-level SmartScreen warning on macOS', () => {
    renderSetup();
    // The top-level testid should not be present (only inside details)
    const topLevel = screen
      .getAllByTestId('smartscreen-warning')
      .filter((el) => !el.closest('details'));
    expect(topLevel).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// Platform: Windows
// ---------------------------------------------------------------------------

describe('Setup — Windows platform', () => {
  beforeEach(() => {
    global.fetch = vi.fn(() => Promise.reject(new Error('daemon not running')));
    vi.useFakeTimers();
    setNavigatorPlatform(
      'Win32',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
    );
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows SmartScreen warning on Windows', () => {
    renderSetup();
    // At least one direct (non-details) SmartScreen warning
    expect(screen.getByTestId('smartscreen-warning')).toBeInTheDocument();
  });

  it('SmartScreen warning contains More info instruction', () => {
    renderSetup();
    const section = screen.getByTestId('smartscreen-warning');
    expect(section).toHaveTextContent(/more info/i);
  });

  it('SmartScreen warning contains Run anyway instruction', () => {
    renderSetup();
    const section = screen.getByTestId('smartscreen-warning');
    expect(section).toHaveTextContent(/run anyway/i);
  });

  it('Gatekeeper warning is in a collapsed details element on Windows', () => {
    renderSetup();
    expect(screen.getByTestId('gatekeeper-details')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// PKCE pairing — waiting state
// ---------------------------------------------------------------------------

describe('Setup — pairing: waiting state', () => {
  beforeEach(() => {
    global.fetch = vi.fn(() => Promise.reject(new Error('daemon not running')));
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows waiting state on initial render', () => {
    renderSetup();
    expect(screen.getByTestId('pairing-waiting')).toBeInTheDocument();
  });

  it('shows "Waiting for auth..." label in waiting state', () => {
    renderSetup();
    expect(screen.getByTestId('pairing-waiting')).toHaveTextContent(/waiting for auth/i);
  });

  it('renders the pairing spinner in waiting state', () => {
    renderSetup();
    // The spinner div is inside pairing-waiting
    const waiting = screen.getByTestId('pairing-waiting');
    expect(waiting.querySelector('.setup-pairing-spinner')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// PKCE pairing — success state
// ---------------------------------------------------------------------------

describe('Setup — pairing: success state', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('transitions to success when daemon returns configured: true', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ configured: true }),
    });

    renderSetup();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_100);
    });

    expect(screen.getByTestId('pairing-success')).toBeInTheDocument();
    expect(screen.getByTestId('pairing-success')).toHaveTextContent(/auth complete/i);
  });

  it('transitions to success when daemon returns status: ok', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    });

    renderSetup();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_100);
    });

    expect(screen.getByTestId('pairing-success')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// PKCE pairing — error / timeout state
// ---------------------------------------------------------------------------

describe('Setup — pairing: timeout/error state', () => {
  beforeEach(() => {
    global.fetch = vi.fn(() => Promise.reject(new Error('daemon not running')));
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows error state after 60s timeout', async () => {
    renderSetup();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(61_000);
    });

    expect(screen.getByTestId('pairing-error')).toBeInTheDocument();
  });

  it('error state shows timeout message', async () => {
    renderSetup();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(61_000);
    });

    expect(screen.getByTestId('pairing-error')).toHaveTextContent(/setup timed out/i);
  });

  it('error state shows retry button', async () => {
    renderSetup();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(61_000);
    });

    expect(screen.getByTestId('retry-button')).toBeInTheDocument();
  });

  it('retry button resets to waiting state', async () => {
    renderSetup();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(61_000);
    });

    expect(screen.getByTestId('pairing-error')).toBeInTheDocument();

    act(() => {
      fireEvent.click(screen.getByTestId('retry-button'));
    });

    expect(screen.getByTestId('pairing-waiting')).toBeInTheDocument();
  });
});
