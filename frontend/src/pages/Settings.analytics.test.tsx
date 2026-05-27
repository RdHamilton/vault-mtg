/**
 * Settings — PostHog analytics event tests (#1838)
 *
 * Covers:
 *   - feature_settings_changed fires when Save Settings succeeds
 *   - does not fire when save fails
 *   - NEGATIVE: no PII in payload
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Settings from './Settings';

vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

// Override @/services/api with minimal mocks for Settings hooks
vi.mock('@/services/api', () => ({
  settings: {
    getSettings: vi.fn(() =>
      Promise.resolve({
        autoRefresh: false,
        refreshInterval: 30,
        showNotifications: true,
        theme: 'dark',
      })
    ),
    updateSettings: vi.fn(() => Promise.resolve()),
  },
  system: {
    getStatus: vi.fn(() => Promise.resolve({ status: 'disconnected' })),
    getVersion: vi.fn(() => Promise.resolve('v1.3.1')),
    connectDaemon: vi.fn(() => Promise.resolve()),
  },
  matches: {
    exportMatches: vi.fn(() => Promise.resolve([])),
    exportMatchesCsv: vi.fn(() => Promise.resolve('')),
  },
}));

vi.mock('@/services/websocketClient', () => ({
  EventsOn: vi.fn(() => () => {}),
  WindowReloadApp: vi.fn(),
}));

vi.mock('../App', () => ({
  subscribeToReplayState: vi.fn(() => () => {}),
  getReplayState: vi.fn(() => ({ isActive: false, isPaused: false, progress: null })),
}));

vi.mock('../components/ToastContainer', () => ({
  showToast: { show: vi.fn() },
}));

// Mock scrollIntoView (not available in jsdom)
Element.prototype.scrollIntoView = vi.fn();

import { settings } from '@/services/api';
import { trackEvent } from '@/services/analytics';

function renderSettings() {
  return render(
    <MemoryRouter>
      <Settings />
    </MemoryRouter>
  );
}

describe('Settings — analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (settings.getSettings as ReturnType<typeof vi.fn>).mockResolvedValue({
      autoRefresh: false,
      refreshInterval: 30,
      showNotifications: true,
      theme: 'dark',
    });
    (settings.updateSettings as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
  });

  describe('feature_settings_changed', () => {
    it('fires when Save Settings is clicked and save succeeds', async () => {
      renderSettings();
      await waitFor(() => screen.getByText('Save Settings'));

      fireEvent.click(screen.getByText('Save Settings'));

      await waitFor(() => {
        expect(trackEvent).toHaveBeenCalledWith({
          name: 'feature_settings_changed',
          properties: { setting_section: 'preferences', setting_key: 'save' },
        });
      });
    });

    it('does not fire when save fails', async () => {
      (settings.updateSettings as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Save failed'));

      renderSettings();
      await waitFor(() => screen.getByText('Save Settings'));

      fireEvent.click(screen.getByText('Save Settings'));

      await new Promise((r) => setTimeout(r, 100));

      const changedCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_settings_changed',
      );
      expect(changedCalls).toHaveLength(0);
    });
  });

  describe('NEGATIVE — no PII in payload', () => {
    it('does not include user_id in feature_settings_changed', async () => {
      renderSettings();
      await waitFor(() => screen.getByText('Save Settings'));

      fireEvent.click(screen.getByText('Save Settings'));

      await waitFor(() => {
        const changedCalls = vi.mocked(trackEvent).mock.calls.filter(
          ([e]) => e.name === 'feature_settings_changed',
        );
        expect(changedCalls.length).toBeGreaterThan(0);
        expect(changedCalls[0][0]).not.toHaveProperty('properties.user_id');
      });
    });
  });
});
