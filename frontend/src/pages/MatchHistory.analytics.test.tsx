/**
 * MatchHistory — PostHog analytics event tests (#1838)
 *
 * Covers:
 *   - feature_match_history_filtered fires on filter changes
 *   - feature_match_details_opened fires on row click
 *   - NEGATIVE: trackEvent not called without user interaction (on mount with no data)
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import MatchHistory from './MatchHistory';
import { mockMatches } from '@/test/mocks/apiMock';
import { AppProvider } from '../context/AppContext';
import { models } from '@/types/models';

vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

import { trackEvent } from '@/services/analytics';

function createMockMatch(overrides: Partial<models.Match> = {}): models.Match {
  return new models.Match({
    ID: 'match-001',
    AccountID: 1,
    EventID: 'event-001',
    EventName: 'Ranked Standard',
    Timestamp: new Date('2024-01-15T10:00:00').toISOString(),
    DurationSeconds: 600,
    PlayerWins: 2,
    OpponentWins: 1,
    PlayerTeamID: 1,
    Format: 'Ladder',
    Result: 'Win',
    OpponentName: 'Opponent123',
    ...overrides,
  });
}

function renderWithProvider(ui: React.ReactElement) {
  return render(<AppProvider>{ui}</AppProvider>);
}

function getSelectByLabel(labelText: string): HTMLSelectElement {
  const label = screen.getByText(labelText);
  const filterGroup = label.closest('.filter-group');
  return filterGroup?.querySelector('select') as HTMLSelectElement;
}

describe('MatchHistory — analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMatches.getMatches.mockResolvedValue([]);
  });

  describe('feature_match_history_filtered', () => {
    it('fires with filter_type=date_range when Date Range changes', async () => {
      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.queryByText('Loading matches...')).not.toBeInTheDocument());

      fireEvent.change(getSelectByLabel('Date Range'), { target: { value: '30days' } });

      expect(trackEvent).toHaveBeenCalledWith({
        name: 'feature_match_history_filtered',
        properties: { filter_type: 'date_range', filter_value: '30days' },
      });
    });

    it('fires with filter_type=format when Card Format changes', async () => {
      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.queryByText('Loading matches...')).not.toBeInTheDocument());

      fireEvent.change(getSelectByLabel('Card Format'), { target: { value: 'Standard' } });

      expect(trackEvent).toHaveBeenCalledWith({
        name: 'feature_match_history_filtered',
        properties: { filter_type: 'format', filter_value: 'Standard' },
      });
    });

    it('fires with filter_type=result when Result changes', async () => {
      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.queryByText('Loading matches...')).not.toBeInTheDocument());

      fireEvent.change(getSelectByLabel('Result'), { target: { value: 'win' } });

      expect(trackEvent).toHaveBeenCalledWith({
        name: 'feature_match_history_filtered',
        properties: { filter_type: 'result', filter_value: 'win' },
      });
    });

    it('fires with filter_type=format when Queue Type changes', async () => {
      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.queryByText('Loading matches...')).not.toBeInTheDocument());

      fireEvent.change(getSelectByLabel('Queue Type'), { target: { value: 'Ladder' } });

      expect(trackEvent).toHaveBeenCalledWith({
        name: 'feature_match_history_filtered',
        properties: { filter_type: 'format', filter_value: 'Ladder' },
      });
    });
  });

  describe('feature_match_details_opened', () => {
    it('fires with match_result and format when a match row is clicked', async () => {
      const match = createMockMatch({ ID: 'match-win', Result: 'Win', Format: 'Ladder' });
      mockMatches.getMatches.mockResolvedValue([match]);

      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.getByRole('table')).toBeInTheDocument());

      const row = screen.getByTitle('Click to view match details');
      fireEvent.click(row);

      expect(trackEvent).toHaveBeenCalledWith({
        name: 'feature_match_details_opened',
        properties: { match_result: 'win', format: 'Ladder' },
      });
    });

    it('uses lowercase match_result', async () => {
      const match = createMockMatch({ ID: 'match-loss', Result: 'Loss', Format: 'Play' });
      mockMatches.getMatches.mockResolvedValue([match]);

      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.getByRole('table')).toBeInTheDocument());

      const row = screen.getByTitle('Click to view match details');
      fireEvent.click(row);

      expect(trackEvent).toHaveBeenCalledWith({
        name: 'feature_match_details_opened',
        properties: { match_result: 'loss', format: 'Play' },
      });
    });
  });

  describe('NEGATIVE — no spurious fires', () => {
    it('does not fire feature_match_history_filtered on mount', async () => {
      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.queryByText('Loading matches...')).not.toBeInTheDocument());

      const filteredCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_match_history_filtered',
      );
      expect(filteredCalls).toHaveLength(0);
    });

    it('does not fire feature_match_details_opened on mount', async () => {
      renderWithProvider(<MatchHistory />);
      await waitFor(() => expect(screen.queryByText('Loading matches...')).not.toBeInTheDocument());

      const openedCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_match_details_opened',
      );
      expect(openedCalls).toHaveLength(0);
    });
  });
});
