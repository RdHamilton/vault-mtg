import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Settings from './Settings';

// Mock scrollIntoView (not available in jsdom)
Element.prototype.scrollIntoView = vi.fn();

// NOTE: @clerk/react and @/services/api/bffHealth are mocked globally in
// src/test/setup.ts. This file uses vi.mocked() to override per-test.

// Mock all API modules used by Settings and its hooks
vi.mock('@/services/api', () => ({
  settings: {
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
  },
  system: {
    getStatus: vi.fn(),
    getVersion: vi.fn(),
    connectDaemon: vi.fn(),
  },
  matches: {
    exportMatches: vi.fn(),
    exportMatchesCsv: vi.fn(),
  },
}));

vi.mock('@/services/websocketClient', () => ({
  EventsOn: vi.fn(() => () => {}),
  WindowReloadApp: vi.fn(),
}));

// Mock the App module for replay state
vi.mock('../App', () => ({
  subscribeToReplayState: vi.fn(() => () => {}),
  getReplayState: vi.fn(() => ({
    isActive: false,
    isPaused: false,
    progress: null,
  })),
}));

// Mock the ToastContainer
vi.mock('../components/ToastContainer', () => ({
  showToast: {
    show: vi.fn(),
  },
}));

import { showToast } from '../components/ToastContainer';
import { settings, system, matches } from '@/services/api';
import { getDaemonHealth } from '@/services/api/bffHealth';

const mockGetDaemonHealth = vi.mocked(getDaemonHealth);

// Default mock connection status (kept for reference / skipped tests)
const defaultConnectionStatus = {
  status: 'standalone',
  connected: false,
  mode: 'standalone',
  url: 'ws://localhost:9999',
  port: 9999,
};

// Default mock settings
const defaultSettings = {
  autoRefresh: false,
  refreshInterval: 30,
  showNotifications: true,
  theme: 'dark',
};

describe('Settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    window.location.hash = '';

    // Default mock implementations
    // useDaemonConnection now calls getDaemonHealth (BFF proxy), not system.getStatus
    mockGetDaemonHealth.mockResolvedValue({ status: 'disconnected' });
    (settings.getSettings as ReturnType<typeof vi.fn>).mockResolvedValue(defaultSettings);
    (settings.updateSettings as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (system.getVersion as ReturnType<typeof vi.fn>).mockResolvedValue('v1.3.1');
  });

  describe('rendering', () => {
    it('renders the Settings page title', async () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeInTheDocument();
    });

    it('renders accordion with all sections', async () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Check accordion section headers
      expect(screen.getByRole('button', { name: /connection/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /preferences/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Export▼?$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /data recovery/i })).toBeInTheDocument();
      // 17Lands and About sections removed in v0.3.1 cleanup (#1976)
      expect(screen.queryByRole('button', { name: /17lands integration/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /^about/i })).not.toBeInTheDocument();
    });

    it('renders Danger Zone as a top-level accordion section (AC1 #2027)', async () => {
      render(<Settings />);
      // Danger Zone is now its own top-level accordion item, separate from Data Recovery
      expect(screen.getByRole('button', { name: /danger zone/i })).toBeInTheDocument();
    });

    it('Data Recovery section is separate from Danger Zone (AC2 #2027)', async () => {
      render(<Settings />);

      const dataRecoveryHeader = screen.getByRole('button', { name: /data recovery/i });
      const dangerZoneHeader = screen.getByRole('button', { name: /danger zone/i });

      // Both exist as distinct accordion buttons — they are not the same element
      expect(dataRecoveryHeader).not.toBe(dangerZoneHeader);
    });

    it('renders Expand All and Collapse All buttons', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      expect(screen.getByRole('button', { name: /expand all/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /collapse all/i })).toBeInTheDocument();
    });

    it('renders Save Settings and Reset to Defaults buttons', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      expect(screen.getByRole('button', { name: /save settings/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /reset to defaults/i })).toBeInTheDocument();
    });

    it('expands connection section by default', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Connection section should be expanded by default
      const connectionHeader = screen.getByRole('button', { name: /connection/i });
      expect(connectionHeader).toHaveAttribute('aria-expanded', 'true');
    });
  });

  describe('accordion navigation', () => {
    it('expands section when clicked', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      expect(preferencesHeader).toHaveAttribute('aria-expanded', 'false');

      fireEvent.click(preferencesHeader);

      expect(preferencesHeader).toHaveAttribute('aria-expanded', 'true');
    });

    it('expands all sections when Expand All is clicked', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      const expandAllButton = screen.getByRole('button', { name: /expand all/i });
      fireEvent.click(expandAllButton);

      // All sections should be expanded
      expect(screen.getByRole('button', { name: /connection/i })).toHaveAttribute('aria-expanded', 'true');
      expect(screen.getByRole('button', { name: /preferences/i })).toHaveAttribute('aria-expanded', 'true');
      expect(screen.getByRole('button', { name: /Export▼?$/i })).toHaveAttribute('aria-expanded', 'true');
    });

    it('collapses all sections when Collapse All is clicked', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // First expand all
      const expandAllButton = screen.getByRole('button', { name: /expand all/i });
      fireEvent.click(expandAllButton);

      // Then collapse all
      const collapseAllButton = screen.getByRole('button', { name: /collapse all/i });
      fireEvent.click(collapseAllButton);

      // All sections should be collapsed
      expect(screen.getByRole('button', { name: /connection/i })).toHaveAttribute('aria-expanded', 'false');
      expect(screen.getByRole('button', { name: /preferences/i })).toHaveAttribute('aria-expanded', 'false');
    });
  });

  describe('Connection section', () => {
    it('displays connection status', async () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand connection section if not already
      const connectionHeader = screen.getByRole('button', { name: /connection/i });
      if (connectionHeader.getAttribute('aria-expanded') === 'false') {
        fireEvent.click(connectionHeader);
      }

      await waitFor(() => {
        expect(screen.getByText('Standalone Mode')).toBeInTheDocument();
      });
    });

    it('displays connected status when daemon is connected', async () => {
      // useDaemonConnection now calls getDaemonHealth (BFF proxy) in all contexts
      // (no isDesktopApp() guard — #2020 / #2021).
      mockGetDaemonHealth.mockResolvedValue({ status: 'connected' });

      render(<MemoryRouter><Settings /></MemoryRouter>);

      await waitFor(() => {
        expect(screen.getByText('Connected to Daemon')).toBeInTheDocument();
      });
    });

    it('displays reconnecting status', async () => {
      // useDaemonConnection now calls getDaemonHealth (BFF proxy) in all contexts
      // (no isDesktopApp() guard — #2020 / #2021).
      mockGetDaemonHealth.mockResolvedValue({ status: 'reconnecting' });

      render(<MemoryRouter><Settings /></MemoryRouter>);

      await waitFor(() => {
        expect(screen.getByText('Reconnecting...')).toBeInTheDocument();
      });
    });

    // AC1–AC3: connection mode dropdown, daemon port input, reconnect button are removed (#2021).
    it('does not render Connection Mode selector (AC1)', async () => {
      render(<Settings />);

      // Expand connection section
      const connectionHeader = screen.getByRole('button', { name: /connection/i });
      if (connectionHeader.getAttribute('aria-expanded') === 'false') {
        fireEvent.click(connectionHeader);
      }

      await waitFor(() => {
        expect(screen.queryByText('Connection Mode')).not.toBeInTheDocument();
      });
    });

    // Skip: Daemon control is not implemented in REST API - useDaemonConnection uses no-op functions
    it.skip('handles reconnect button click', async () => {
      (system.getStatus as ReturnType<typeof vi.fn>).mockResolvedValue({
        ...defaultConnectionStatus,
        status: 'connected',
        connected: true,
        mode: 'daemon',
      });
      (system.connectDaemon as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);

      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Wait for initial load
      await waitFor(() => {
        expect(system.getStatus).toHaveBeenCalled();
      });

      // Expand connection section
      const connectionHeader = screen.getByRole('button', { name: /connection/i });
      if (connectionHeader.getAttribute('aria-expanded') === 'false') {
        fireEvent.click(connectionHeader);
      }

      const reconnectButton = screen.getByRole('button', { name: /reconnect to daemon/i });
      fireEvent.click(reconnectButton);

      await waitFor(() => {
        expect(system.connectDaemon).toHaveBeenCalled();
      });
    });

    // Test removed: reconnect error handling uses no-op functions in useDaemonConnection

    // Tests removed: Daemon control functions (switchToStandaloneMode, switchToDaemonMode, setDaemonPort)
    // are not implemented in REST API - useDaemonConnection uses no-op functions
  });

  describe('Preferences section', () => {
    it('renders theme selector', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand preferences section
      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      fireEvent.click(preferencesHeader);

      expect(screen.getByText('Theme')).toBeInTheDocument();
    });

    it('renders auto-refresh toggle', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand preferences section
      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      fireEvent.click(preferencesHeader);

      expect(screen.getByText('Auto-refresh data')).toBeInTheDocument();
    });

    it('shows refresh interval when auto-refresh is enabled', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand preferences section
      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      fireEvent.click(preferencesHeader);

      // Find and click the auto-refresh toggle
      const autoRefreshCheckbox = screen.getByRole('checkbox', { name: /auto-refresh data/i });
      fireEvent.click(autoRefreshCheckbox);

      expect(screen.getByText('Refresh Interval (seconds)')).toBeInTheDocument();
    });

    it('renders notifications toggle', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand preferences section
      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      fireEvent.click(preferencesHeader);

      expect(screen.getByText('Show notifications')).toBeInTheDocument();
    });
  });

  describe('Import/Export section', () => {
    it('renders export buttons', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand import/export section
      const importExportHeader = screen.getByRole('button', { name: /Export▼?$/i });
      fireEvent.click(importExportHeader);

      expect(screen.getByRole('button', { name: /export to json/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /export to csv/i })).toBeInTheDocument();
    });

    it('handles export to JSON', async () => {
      (matches.exportMatches as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);

      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand import/export section
      const importExportHeader = screen.getByRole('button', { name: /Export▼?$/i });
      fireEvent.click(importExportHeader);

      const exportJsonButton = screen.getByRole('button', { name: /export to json/i });
      fireEvent.click(exportJsonButton);

      await waitFor(() => {
        expect(matches.exportMatches).toHaveBeenCalled();
      });
    });

    it('handles export to CSV', async () => {
      (matches.exportMatches as ReturnType<typeof vi.fn>).mockResolvedValue('csv data');

      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand import/export section
      const importExportHeader = screen.getByRole('button', { name: /Export▼?$/i });
      fireEvent.click(importExportHeader);

      const exportCsvButton = screen.getByRole('button', { name: /export to csv/i });
      fireEvent.click(exportCsvButton);

      await waitFor(() => {
        expect(matches.exportMatches).toHaveBeenCalledWith('csv');
      });
    });

    // Test removed: importFromFile requires native file picker integration - useDataManagement uses no-op functions

    it('shows error when export fails', async () => {
      (matches.exportMatches as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Export failed'));

      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand import/export section
      const importExportHeader = screen.getByRole('button', { name: /Export▼?$/i });
      fireEvent.click(importExportHeader);

      const exportJsonButton = screen.getByRole('button', { name: /export to json/i });
      fireEvent.click(exportJsonButton);

      await waitFor(() => {
        expect(showToast.show).toHaveBeenCalledWith(
          expect.stringContaining('Failed to export'),
          'error'
        );
      });
    });
  });

  describe('Data Recovery section', () => {
    it('renders replay historical logs button', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand data recovery section
      const dataRecoveryHeader = screen.getByRole('button', { name: /data recovery/i });
      fireEvent.click(dataRecoveryHeader);

      expect(screen.getByRole('button', { name: /replay historical logs/i })).toBeInTheDocument();
    });

    it('shows clear data before replay checkbox', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand data recovery section
      const dataRecoveryHeader = screen.getByRole('button', { name: /data recovery/i });
      fireEvent.click(dataRecoveryHeader);

      expect(screen.getByText(/clear all data before replay/i)).toBeInTheDocument();
    });

    it('disables replay button when not connected', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand data recovery section
      const dataRecoveryHeader = screen.getByRole('button', { name: /data recovery/i });
      fireEvent.click(dataRecoveryHeader);

      const replayButton = screen.getByRole('button', { name: /replay historical logs/i });
      expect(replayButton).toBeDisabled();
    });

    it('shows daemon warning when not connected', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Expand data recovery section
      const dataRecoveryHeader = screen.getByRole('button', { name: /data recovery/i });
      fireEvent.click(dataRecoveryHeader);

      expect(screen.getByText(/daemon must be running to replay logs/i)).toBeInTheDocument();
    });
  });

  // 17Lands Integration section removed in v0.3.1 cleanup (#1976) — sync is now handled globally by Lambda.
  // About section removed in v0.3.1 cleanup (#1976) — version info was stale.

  describe('Developer Mode', () => {
    it('does not show Developer Tools section by default', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      expect(screen.queryByRole('button', { name: /developer tools/i })).not.toBeInTheDocument();
    });

    it('shows Developer Tools section when developer mode is enabled', () => {
      // Enable developer mode in localStorage
      localStorage.setItem('mtga-companion-developer-mode', 'true');

      render(<MemoryRouter><Settings /></MemoryRouter>);

      expect(screen.getByRole('button', { name: /developer tools/i })).toBeInTheDocument();
    });

    // 'activates developer mode after clicking version 5 times' removed — that interaction
    // was gated behind the About section which was removed in v0.3.1 cleanup (#1976).
    // 'shows developer mode indicator in About section when enabled' removed — About section removed (#1976).
    // 'allows disabling developer mode via toggle' removed — toggle lived in About section (#1976).
  });

  describe('Save/Reset actions', () => {
    it('shows success notification when save is clicked', async () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Wait for settings to load from backend first (button is disabled while loading)
      const saveButton = screen.getByRole('button', { name: /save settings/i });
      await waitFor(() => {
        expect(saveButton).not.toBeDisabled();
      });

      // Click save button
      fireEvent.click(saveButton);

      // Wait for the async save to complete and notification to appear
      await waitFor(() => {
        expect(screen.getByText('Settings saved successfully!')).toBeInTheDocument();
      });

      // Verify settings.updateSettings was called
      expect(settings.updateSettings).toHaveBeenCalled();
    });

    it('resets preferences when reset is clicked', async () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      // Wait for settings to load from backend (button becomes enabled)
      const resetButton = screen.getByRole('button', { name: /reset to defaults/i });
      await waitFor(() => {
        expect(resetButton).not.toBeDisabled();
      });

      // Expand preferences section
      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      fireEvent.click(preferencesHeader);

      // Enable auto-refresh
      const autoRefreshCheckbox = screen.getByRole('checkbox', { name: /auto-refresh data/i });
      fireEvent.click(autoRefreshCheckbox);

      expect(autoRefreshCheckbox).toBeChecked();

      // Reset
      fireEvent.click(resetButton);

      // Auto-refresh should be unchecked after reset
      await waitFor(() => {
        expect(autoRefreshCheckbox).not.toBeChecked();
      });
    });
  });

  describe('Keyboard navigation', () => {
    it('toggles section on Enter key', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      preferencesHeader.focus();
      fireEvent.keyDown(preferencesHeader, { key: 'Enter' });

      expect(preferencesHeader).toHaveAttribute('aria-expanded', 'true');
    });

    it('toggles section on Space key', () => {
      render(<MemoryRouter><Settings /></MemoryRouter>);

      const preferencesHeader = screen.getByRole('button', { name: /preferences/i });
      preferencesHeader.focus();
      fireEvent.keyDown(preferencesHeader, { key: ' ' });

      expect(preferencesHeader).toHaveAttribute('aria-expanded', 'true');
    });
  });
});
