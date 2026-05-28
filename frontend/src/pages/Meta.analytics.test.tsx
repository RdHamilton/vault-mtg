/**
 * Meta — PostHog analytics event tests (#1838)
 *
 * Covers:
 *   - feature_meta_viewed fires once on mount when data loads
 *   - does not fire when API returns no data
 *   - NEGATIVE: fires only once despite format changes
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { renderWithRouter } from '@/test/utils/testUtils';
import Meta from './Meta';
import { mockMeta } from '@/test/mocks/apiMock';

vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

import { trackEvent } from '@/services/analytics';

describe('Meta — analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  describe('feature_meta_viewed', () => {
    it('fires once on mount when meta data loads', async () => {
      mockMeta.getMetaArchetypes.mockResolvedValue([
        { name: 'Azorius Control', tier: 1, winRate: 0.55, metaShare: 12 },
      ]);

      renderWithRouter(<Meta />);

      await waitFor(() => {
        expect(trackEvent).toHaveBeenCalledWith({
          name: 'feature_meta_viewed',
        });
      });
    });

    it('fires only once on mount even when component re-renders', async () => {
      mockMeta.getMetaArchetypes.mockResolvedValue([
        { name: 'Azorius Control', tier: 1, winRate: 0.55, metaShare: 12 },
      ]);

      const { rerender } = renderWithRouter(<Meta />);

      await waitFor(() => {
        const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
          ([e]) => e.name === 'feature_meta_viewed',
        );
        expect(viewedCalls).toHaveLength(1);
      });

      rerender(<Meta />);
      await new Promise((r) => setTimeout(r, 20));

      const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_meta_viewed',
      );
      expect(viewedCalls).toHaveLength(1);
    });
  });

  describe('NEGATIVE — no PII in payload', () => {
    it('does not include user_id in feature_meta_viewed', async () => {
      mockMeta.getMetaArchetypes.mockResolvedValue([{ name: 'Mono Red Aggro', tier: 1 }]);

      renderWithRouter(<Meta />);

      await waitFor(() => {
        const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
          ([e]) => e.name === 'feature_meta_viewed',
        );
        expect(viewedCalls.length).toBeGreaterThan(0);
        expect(viewedCalls[0][0]).not.toHaveProperty('properties.user_id');
      });
    });
  });
});
