/**
 * DraftAnalytics — PostHog analytics event tests (#1838)
 *
 * Covers:
 *   - feature_draft_analytics_viewed fires once on mount when data is non-empty
 *   - does not fire when no draft data available
 *   - NEGATIVE: does not fire again on re-render
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import DraftAnalytics from './DraftAnalytics';
import { mockDrafts } from '@/test/mocks/apiMock';

vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

vi.mock('@/hooks/useSettings', () => ({
  useSettings: vi.fn(() => ({ autoRefresh: false, refreshInterval: 30 })),
}));

// Mock sub-components to avoid deep rendering
vi.mock('@/components/TemporalTrends', () => ({ default: () => null }));
vi.mock('@/components/CommunityComparison', () => ({ default: () => null }));
vi.mock('@/components/FormatInsights', () => ({ default: () => null }));

import { trackEvent } from '@/services/analytics';

describe('DraftAnalytics — analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('feature_draft_analytics_viewed', () => {
    it('fires once on mount when draft formats are available', async () => {
      mockDrafts.getDraftFormats.mockResolvedValue(['DSK', 'FDN', 'BLB']);

      render(<DraftAnalytics />);

      await waitFor(() => {
        expect(trackEvent).toHaveBeenCalledWith({
          name: 'feature_draft_analytics_viewed',
          properties: { draft_count: 3 },
        });
      });
    });

    it('does not fire when no draft formats are available', async () => {
      mockDrafts.getDraftFormats.mockResolvedValue([]);

      render(<DraftAnalytics />);

      // Give async effects time to resolve
      await new Promise((r) => setTimeout(r, 50));

      const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_draft_analytics_viewed',
      );
      expect(viewedCalls).toHaveLength(0);
    });

    it('fires only once even if the effect re-runs', async () => {
      mockDrafts.getDraftFormats.mockResolvedValue(['DSK']);

      const { rerender } = render(<DraftAnalytics />);
      await waitFor(() => {
        expect(trackEvent).toHaveBeenCalledWith({
          name: 'feature_draft_analytics_viewed',
          properties: { draft_count: 1 },
        });
      });

      rerender(<DraftAnalytics />);
      await new Promise((r) => setTimeout(r, 20));

      const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_draft_analytics_viewed',
      );
      expect(viewedCalls).toHaveLength(1);
    });
  });

  describe('NEGATIVE — no PII in event payload', () => {
    it('does not include user_id in feature_draft_analytics_viewed', async () => {
      mockDrafts.getDraftFormats.mockResolvedValue(['DSK', 'FDN']);

      render(<DraftAnalytics />);

      await waitFor(() => {
        const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
          ([e]) => e.name === 'feature_draft_analytics_viewed',
        );
        expect(viewedCalls).toHaveLength(1);
        expect(viewedCalls[0][0]).not.toHaveProperty('properties.user_id');
      });
    });
  });
});
