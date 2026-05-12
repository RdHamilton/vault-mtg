import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as settings from '../settings';

// Phase 2 PR #12 — settings now hit the BFF via apiClient.
vi.mock('../../apiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

import { get, put } from '../../apiClient';

describe('settings API (BFF routes)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getSettings', () => {
    it('calls GET /settings', async () => {
      vi.mocked(get).mockResolvedValue({ theme: 'dark' });
      const result = await settings.getSettings();
      expect(get).toHaveBeenCalledWith('/settings');
      expect(result).toEqual({ theme: 'dark' });
    });
  });

  describe('updateSettings', () => {
    it('calls PUT /settings with the full settings object', async () => {
      vi.mocked(put).mockResolvedValue(undefined);
      const body = { theme: 'light', autoRefresh: true } as unknown as settings.AppSettings;
      await settings.updateSettings(body);
      expect(put).toHaveBeenCalledWith('/settings', body);
    });
  });

  describe('getSetting', () => {
    it('encodes the key in the path', async () => {
      vi.mocked(get).mockResolvedValue({ value: 'dark' });
      await settings.getSetting('theme');
      expect(get).toHaveBeenCalledWith('/settings/theme');
    });

    it('percent-encodes keys with special characters', async () => {
      vi.mocked(get).mockResolvedValue({ value: 1 });
      await settings.getSetting('color/palette');
      expect(get).toHaveBeenCalledWith('/settings/color%2Fpalette');
    });
  });

  describe('updateSetting', () => {
    it('PUTs the value wrapped in { value }', async () => {
      vi.mocked(put).mockResolvedValue(undefined);
      await settings.updateSetting('theme', 'dark');
      expect(put).toHaveBeenCalledWith('/settings/theme', { value: 'dark' });
    });

    it('accepts complex values verbatim', async () => {
      vi.mocked(put).mockResolvedValue(undefined);
      await settings.updateSetting('colorPalette', { primary: 'blue', weight: 0.6 });
      expect(put).toHaveBeenCalledWith('/settings/colorPalette', {
        value: { primary: 'blue', weight: 0.6 },
      });
    });
  });
});
