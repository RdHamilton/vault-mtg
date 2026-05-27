/**
 * Chart Pages — PostHog analytics event tests (#1838)
 *
 * Covers (all 4 chart pages):
 *   - WinRateTrend, RankProgression, FormatDistribution, ResultBreakdown
 *
 * Per Ray Q1: feature_chart_interacted is debounced 300ms trailing (inline useRef timer).
 * These tests use vi.useFakeTimers() to assert debounce behaviour.
 *
 * NEGATIVE: event does NOT fire without a filter change.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { renderWithRouter } from '@/test/utils/testUtils';
import WinRateTrend from './WinRateTrend';
import RankProgression from './RankProgression';
import FormatDistribution from './FormatDistribution';
import ResultBreakdown from './ResultBreakdown';
import { mockMatches } from '@/test/mocks/apiMock';
import { AppProvider } from '../context/AppContext';
import { render } from '@testing-library/react';

vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

import { trackEvent } from '@/services/analytics';

function renderChart(ui: React.ReactElement) {
  return render(<AppProvider>{ui}</AppProvider>);
}

// Helper to get the first select in a filter-group by label text
function getSelectByLabel(labelText: string): HTMLSelectElement {
  const label = screen.getByText(labelText);
  const filterGroup = label.closest('.filter-group');
  return filterGroup?.querySelector('select') as HTMLSelectElement;
}

describe('Chart Pages — feature_chart_interacted debounce (Ray Q1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    mockMatches.getTrendAnalysis.mockResolvedValue({ Periods: [] });
    mockMatches.getRankProgressionTimeline.mockResolvedValue({ entries: [] });
    mockMatches.getFormatDistribution.mockResolvedValue({});
    mockMatches.getStats.mockResolvedValue(null);
    mockMatches.statsFilterToRequest.mockImplementation((f) => f);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('WinRateTrend', () => {
    it('fires feature_chart_interacted after 300ms on date range change', async () => {
      renderChart(<WinRateTrend />);

      // Wait for initial load (using real timers for async, then fake for debounce)
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: '30days' } });

      // Before debounce fires — no event
      expect(vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted')).toHaveLength(0);

      // Advance 300ms
      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(1);
      expect(calls[0][0]).toMatchObject({
        name: 'feature_chart_interacted',
        properties: { chart: 'win_rate_trend', interaction: 'time_range_changed' },
      });
    });

    it('debounces — only fires once for rapid consecutive changes', async () => {
      renderChart(<WinRateTrend />);
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: '30days' } });
      vi.advanceTimersByTime(100);
      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: '90days' } });
      vi.advanceTimersByTime(100);
      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: 'all' } });
      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(1);
    });

    it('does not fire feature_chart_interacted on mount without interaction', async () => {
      renderChart(<WinRateTrend />);
      await vi.runAllTimersAsync();

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(0);
    });

    it('fires with interaction=format_changed on Format change', async () => {
      renderChart(<WinRateTrend />);
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Format'), { target: { value: 'Ladder' } });
      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls.length).toBeGreaterThan(0);
      expect(calls[calls.length - 1][0].properties.interaction).toBe('format_changed');
    });
  });

  describe('RankProgression', () => {
    it('fires feature_chart_interacted after 300ms on date range change', async () => {
      renderChart(<RankProgression />);
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: '30days' } });
      expect(vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted')).toHaveLength(0);

      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(1);
      expect(calls[0][0]).toMatchObject({
        name: 'feature_chart_interacted',
        properties: { chart: 'rank_progression', interaction: 'time_range_changed' },
      });
    });

    it('does not fire on mount without interaction', async () => {
      renderChart(<RankProgression />);
      await vi.runAllTimersAsync();

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(0);
    });
  });

  describe('FormatDistribution', () => {
    it('fires feature_chart_interacted after 300ms on date range change', async () => {
      renderChart(<FormatDistribution />);
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: 'all' } });
      expect(vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted')).toHaveLength(0);

      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(1);
      expect(calls[0][0]).toMatchObject({
        name: 'feature_chart_interacted',
        properties: { chart: 'format_distribution', interaction: 'time_range_changed' },
      });
    });

    it('does not fire on mount without interaction', async () => {
      renderChart(<FormatDistribution />);
      await vi.runAllTimersAsync();

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(0);
    });
  });

  describe('ResultBreakdown', () => {
    it('fires feature_chart_interacted after 300ms on date range change', async () => {
      renderChart(<ResultBreakdown />);
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: 'all' } });
      expect(vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted')).toHaveLength(0);

      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(1);
      expect(calls[0][0]).toMatchObject({
        name: 'feature_chart_interacted',
        properties: { chart: 'result_breakdown', interaction: 'time_range_changed' },
      });
    });

    it('fires with interaction=format_changed on Format change', async () => {
      renderChart(<ResultBreakdown />);
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Format'), { target: { value: 'Ladder' } });
      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls.length).toBeGreaterThan(0);
      expect(calls[calls.length - 1][0].properties.interaction).toBe('format_changed');
    });

    it('does not fire on mount without interaction', async () => {
      renderChart(<ResultBreakdown />);
      await vi.runAllTimersAsync();

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      expect(calls).toHaveLength(0);
    });
  });

  describe('NEGATIVE — no PII in payloads', () => {
    it('WinRateTrend: feature_chart_interacted does not include user_id', async () => {
      renderChart(<WinRateTrend />);
      await vi.runAllTimersAsync();

      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: '90days' } });
      vi.advanceTimersByTime(300);

      const calls = vi.mocked(trackEvent).mock.calls.filter(([e]) => e.name === 'feature_chart_interacted');
      for (const [event] of calls) {
        expect(event).not.toHaveProperty('properties.user_id');
      }
    });
  });
});
