import { useState, useEffect, useRef, useCallback } from 'react';
import { useAuth } from '@clerk/react';
import { getDraftHistory } from '@/services/api/bffDraftHistory';
import type { DraftHistoryItem } from '@/services/api/bffDraftHistory';
import LoadingSpinner from '../components/LoadingSpinner';
import EmptyState from '../components/EmptyState';
import './BffDraftHistory.css';

const PAGE_SIZE = 20;

const BffDraftHistory = () => {
  const { getToken, isSignedIn } = useAuth();
  // Stable ref so useCallback / useEffect deps don't re-fire on every render
  const getTokenRef = useRef(getToken);
  useEffect(() => { getTokenRef.current = getToken; });

  const [drafts, setDrafts] = useState<DraftHistoryItem[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchDrafts = useCallback(
    async (nextOffset: number) => {
      if (!isSignedIn) return;
      setLoading(true);
      setError(null);
      try {
        const token = await getTokenRef.current();
        if (!token) throw new Error('No auth token');
        const data = await getDraftHistory(token, { limit: PAGE_SIZE, offset: nextOffset });
        setDrafts(data.drafts);
        setTotal(data.total);
        setOffset(nextOffset);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load draft history');
      } finally {
        setLoading(false);
      }
    },
    [isSignedIn]
  );

  useEffect(() => {
    void fetchDrafts(0);
  }, [fetchDrafts]);

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
    <div className="page-container">
      <div className="bff-draft-history-header">
        <h1 className="page-title">Draft History</h1>
      </div>

      {loading && <LoadingSpinner message="Loading drafts..." />}

      {!loading && error && (
        <div className="error-state">
          <p>{error}</p>
        </div>
      )}

      {!loading && !error && total === 0 && (
        <div data-testid="draft-history-empty">
          <EmptyState
            icon="🃏"
            heading="No drafts yet"
            subtext="Your cloud draft history will appear here once synced."
            variant="no-data"
          />
        </div>
      )}

      {!loading && !error && total > 0 && (
        <>
          <div className="bff-draft-history-table-wrapper">
            <table data-testid="draft-history-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Set</th>
                  <th>Wins</th>
                  <th>Losses</th>
                </tr>
              </thead>
              <tbody>
                {drafts.map((draft) => (
                  <tr key={draft.id}>
                    <td>{formatDate(draft.drafted_at)}</td>
                    <td>{draft.set_code}</td>
                    <td>{draft.wins}</td>
                    <td>{draft.losses}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="bff-draft-history-footer">
            <div className="pagination">
              <button
                className="pagination-btn"
                onClick={() => void fetchDrafts(offset - PAGE_SIZE)}
                disabled={!hasPrev}
              >
                Previous
              </button>
              <span className="pagination-info">
                Page {currentPage} of {totalPages}
              </span>
              <button
                className="pagination-btn"
                onClick={() => void fetchDrafts(offset + PAGE_SIZE)}
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

export default BffDraftHistory;
