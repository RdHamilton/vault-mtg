import { useState, useEffect, useRef, useCallback } from 'react';
import { useAuth } from '@clerk/react';
import { getMatchHistory } from '@/services/api/bffMatchHistory';
import type { MatchHistoryItem } from '@/services/api/bffMatchHistory';
import { EventsOn } from '@/services/websocketClient';
import LoadingSpinner from '../components/LoadingSpinner';
import EmptyState from '../components/EmptyState';
import { trackEvent } from '@/services/analytics';
import './BffMatchHistory.css';

const FIRST_DATA_FLAG = 'vaultmtg_ph_funnel_first_data_loaded_fired';

const PAGE_SIZE = 20;

const BffMatchHistory = () => {
  const { getToken, isSignedIn } = useAuth();
  // Stable ref so useCallback / useEffect deps don't re-fire on every render
  const getTokenRef = useRef(getToken);
  useEffect(() => { getTokenRef.current = getToken; });

  const [matches, setMatches] = useState<MatchHistoryItem[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchMatches = useCallback(
    async (nextOffset: number) => {
      if (!isSignedIn) return;
      setLoading(true);
      setError(null);
      try {
        const token = await getTokenRef.current();
        if (!token) throw new Error('No auth token');
        const data = await getMatchHistory(token, { limit: PAGE_SIZE, offset: nextOffset });
        setMatches(data.matches);
        setTotal(data.total);
        setOffset(nextOffset);
        // Fire funnel_first_data_loaded once per user (localStorage guard).
        if (data.total > 0 && !localStorage.getItem(FIRST_DATA_FLAG)) {
          trackEvent({ name: 'funnel_first_data_loaded', properties: { match_count: data.total } });
          localStorage.setItem(FIRST_DATA_FLAG, '1');
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load match history');
      } finally {
        setLoading(false);
      }
    },
    [isSignedIn]
  );

  useEffect(() => {
    void fetchMatches(0);
  }, [fetchMatches]);

  // Refresh when the BFF emits stats:updated (fired after a match is processed).
  useEffect(() => {
    const unsub = EventsOn('stats:updated', () => {
      void fetchMatches(0);
    });
    return unsub;
  }, [fetchMatches]);

  const hasPrev = offset > 0;
  const hasNext = offset + PAGE_SIZE < total;
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const formatDate = (iso: string) =>
    new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });

  return (
    <div className="page-container" data-testid="match-history-page">
      <div className="bff-match-history-header">
        <h1 className="page-title">Match History</h1>
      </div>

      {loading && <LoadingSpinner message="Loading matches..." />}

      {!loading && error && (
        <div className="error-state">
          <p>{error}</p>
        </div>
      )}

      {!loading && !error && total === 0 && (
        <div data-testid="match-history-empty">
          <EmptyState
            icon="🎮"
            heading="No matches yet"
            subtext="Your cloud match history will appear here once synced."
            variant="no-data"
          />
        </div>
      )}

      {!loading && !error && total > 0 && (
        <>
          <div className="bff-match-history-table-wrapper">
            <table data-testid="match-history-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Format</th>
                  <th>Opponent Deck</th>
                  <th>Result</th>
                </tr>
              </thead>
              <tbody>
                {matches.map((match) => (
                  <tr key={match.id} className={`result-${match.result.toLowerCase()}`}>
                    <td>{formatDate(match.played_at)}</td>
                    <td>{match.format}</td>
                    <td className="ph-no-capture">{match.opponent_deck || '—'}</td>
                    <td>
                      <span className={`result-badge ${match.result.toLowerCase()}`}>
                        {match.result.toUpperCase()}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="bff-match-history-footer">
            <div className="pagination">
              <button
                className="pagination-btn"
                onClick={() => void fetchMatches(offset - PAGE_SIZE)}
                disabled={!hasPrev}
              >
                Previous
              </button>
              <span className="pagination-info">
                Page {currentPage} of {totalPages}
              </span>
              <button
                className="pagination-btn"
                onClick={() => void fetchMatches(offset + PAGE_SIZE)}
                disabled={!hasNext}
              >
                Next
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  );
};

export default BffMatchHistory;
