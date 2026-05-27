import { useState, useEffect, useMemo, useRef } from 'react';
import { EventsOn } from '@/services/websocketClient';
import { matches as matchesApi } from '@/services/api';
import { models } from '@/types/models';
import { getDisplayFormat, getDisplayEventName } from '@/utils/formatNormalization';
import LoadingSpinner from '../components/LoadingSpinner';
import Tooltip from '../components/Tooltip';
import EmptyState from '../components/EmptyState';
import DaemonEmptyState from '../components/DaemonEmptyState';
import ErrorState from '../components/ErrorState';
import MatchDetailsModal from '../components/MatchDetailsModal';
import MatchNotesModal from '../components/MatchNotesModal';
import MatchComparisonPanel from '../components/MatchComparisonPanel';
import { useAppContext } from '../context/AppContext';
import { useDaemonStatus } from '../hooks/useDaemonStatus';
import { trackEvent } from '@/services/analytics';
import './MatchHistory.css';

type SortField = 'Timestamp' | 'Result' | 'Format' | 'EventName' | 'DeckName';
type SortDirection = 'asc' | 'desc';

const MatchHistory = () => {
  const { filters, updateFilters } = useAppContext();
  const { dateRange, customStartDate, customEndDate, cardFormat, queueType, result } = filters.matchHistory;

  // Daemon connectivity — drives the no-daemon empty state
  const { daemonConnected, daemonChecked } = useDaemonStatus();

  // Analytics: fire funnel_daemon_installed once when daemon first connects
  const daemonInstalledFiredRef = useRef(false);
  useEffect(() => {
    if (daemonChecked && daemonConnected && !daemonInstalledFiredRef.current) {
      daemonInstalledFiredRef.current = true;
      trackEvent({ name: 'funnel_daemon_installed', properties: { source: 'match_history' } });
    }
  }, [daemonChecked, daemonConnected]);

  // Analytics: fire funnel_first_game_played once when the first match record is seen
  const firstGamePlayedFiredRef = useRef(false);

  const [matchList, setMatchList] = useState<models.Match[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Pagination
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);

  // Sorting
  const [sortField, setSortField] = useState<SortField>('Timestamp');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');

  // Match details modal
  const [selectedMatch, setSelectedMatch] = useState<models.Match | null>(null);

  // Match notes modal
  const [notesMatchId, setNotesMatchId] = useState<string | null>(null);

  // Match comparison panel
  const [showComparisonPanel, setShowComparisonPanel] = useState(false);

  useEffect(() => {
    const loadMatches = async () => {
    try {
      setLoading(true);
      setError(null);

      // Build filter
      const filter = new models.StatsFilter();

      // Date range
      if (dateRange === 'custom') {
        // Use custom date range if provided
        if (customStartDate) {
          const start = new Date(customStartDate + 'T00:00:00');
          filter.StartDate = start;
        }
        if (customEndDate) {
          // Add 1 day to end date to make it inclusive
          // (e.g., end date "2024-11-14" becomes "2024-11-15T00:00:00")
          const end = new Date(customEndDate + 'T00:00:00');
          end.setDate(end.getDate() + 1);
          filter.EndDate = end;
        }
      } else if (dateRange !== 'all') {
        const now = new Date();
        const start = new Date();

        switch (dateRange) {
          case '7days':
            start.setDate(now.getDate() - 7);
            break;
          case '30days':
            start.setDate(now.getDate() - 30);
            break;
          case '90days':
            start.setDate(now.getDate() - 90);
            break;
        }

        // Set start time to beginning of day
        start.setHours(0, 0, 0, 0);
        // Add 1 day to end date to make it inclusive (beginning of next day)
        const end = new Date(now);
        end.setDate(end.getDate() + 1);
        end.setHours(0, 0, 0, 0);

        filter.StartDate = start;
        filter.EndDate = end;
      }

      // Card format filter (deck format)
      if (cardFormat !== 'all') {
        filter.DeckFormat = cardFormat;
      }

      // Queue type filter (ladder/play)
      if (queueType !== 'all') {
        filter.Format = queueType;
      }

      // Result filter
      if (result !== 'all') {
        filter.Result = result;
      }

      const matchData = await matchesApi.getMatches(matchesApi.statsFilterToRequest(filter));
      const matches = matchData || [];
      setMatchList(matches);
      // Fire first-game-played funnel event when the user has at least one match
      if (matches.length > 0 && !firstGamePlayedFiredRef.current) {
        firstGamePlayedFiredRef.current = true;
        const firstMatch = matches[0];
        trackEvent({
          name: 'funnel_first_game_played',
          properties: {
            format: firstMatch.Format ?? undefined,
            source: 'match_history',
          },
        });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load matches');
      console.error('Error loading matches:', err);
    } finally {
      setLoading(false);
    }
  };

    loadMatches();
  }, [dateRange, customStartDate, customEndDate, cardFormat, queueType, result]);

  // Listen for real-time updates
  useEffect(() => {
    const loadMatches = async () => {
      try {
        setLoading(true);
        setError(null);

        // Build filter
        const filter = new models.StatsFilter();

        // Date range
        if (dateRange === 'custom') {
          // Use custom date range if provided
          if (customStartDate) {
            const start = new Date(customStartDate + 'T00:00:00');
            filter.StartDate = start;
          }
          if (customEndDate) {
            // Add 1 day to end date to make it inclusive
            // (e.g., end date "2024-11-14" becomes "2024-11-15T00:00:00")
            const end = new Date(customEndDate + 'T00:00:00');
            end.setDate(end.getDate() + 1);
            filter.EndDate = end;
          }
        } else if (dateRange !== 'all') {
          const now = new Date();
          const start = new Date();

          switch (dateRange) {
            case '7days':
              start.setDate(now.getDate() - 7);
              break;
            case '30days':
              start.setDate(now.getDate() - 30);
              break;
            case '90days':
              start.setDate(now.getDate() - 90);
              break;
          }

          // Set start time to beginning of day
          start.setHours(0, 0, 0, 0);
          // Add 1 day to end date to make it inclusive (beginning of next day)
          const end = new Date(now);
          end.setDate(end.getDate() + 1);
          end.setHours(0, 0, 0, 0);

          filter.StartDate = start;
          filter.EndDate = end;
        }

        // Card format filter (deck format)
        if (cardFormat !== 'all') {
          filter.DeckFormat = cardFormat;
        }

        // Queue type filter (ladder/play)
        if (queueType !== 'all') {
          filter.Format = queueType;
        }

        // Result filter
        if (result !== 'all') {
          filter.Result = result;
        }

        const matchData = await matchesApi.getMatches(matchesApi.statsFilterToRequest(filter));
        const matches = matchData || [];
        setMatchList(matches);
        if (matches.length > 0 && !firstGamePlayedFiredRef.current) {
          firstGamePlayedFiredRef.current = true;
          const firstMatch = matches[0];
          trackEvent({
            name: 'funnel_first_game_played',
            properties: {
              format: firstMatch.Format ?? undefined,
              source: 'match_history',
            },
          });
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load matches');
        console.error('Error loading matches:', err);
      } finally {
        setLoading(false);
      }
    };

    const unsubscribe = EventsOn('stats:updated', () => {
      console.log('Stats updated event received - reloading match history');
      loadMatches();
    });

    return () => {
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, [dateRange, customStartDate, customEndDate, cardFormat, queueType, result]);

  const formatTimestamp = (timestamp: unknown) => {
    return new Date(String(timestamp)).toLocaleString();
  };

  const formatScore = (wins: number, losses: number) => {
    return `${wins}-${losses}`;
  };


  const handleSort = (field: SortField) => {
    if (sortField === field) {
      // Toggle direction
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      // New field, default to descending
      setSortField(field);
      setSortDirection('desc');
    }
    setPage(1); // Reset to first page when sorting changes
  };


  // Sort and paginate matches
  const sortedMatches = [...matchList].sort((a, b) => {
    let aVal: string | number = String(a[sortField] ?? '');
    let bVal: string | number = String(b[sortField] ?? '');

    // Handle timestamp
    if (sortField === 'Timestamp') {
      aVal = new Date(aVal).getTime();
      bVal = new Date(bVal).getTime();
    }

    // Handle nulls/undefined (already converted to empty string)
    if (aVal === '') return 1;
    if (bVal === '') return -1;

    // String comparison (lowercase for case-insensitive sort)
    if (typeof aVal === 'string' && typeof bVal === 'string') {
      aVal = aVal.toLowerCase();
      bVal = bVal.toLowerCase();
    }

    if (sortDirection === 'asc') {
      return aVal > bVal ? 1 : aVal < bVal ? -1 : 0;
    } else {
      return aVal < bVal ? 1 : aVal > bVal ? -1 : 0;
    }
  });

  const totalPages = Math.ceil(sortedMatches.length / pageSize);
  const paginatedMatches = sortedMatches.slice((page - 1) * pageSize, page * pageSize);

  // Extract unique formats and decks from matches for comparison panel
  const uniqueFormats = useMemo(() => {
    const formats = new Set<string>();
    matchList.forEach(match => {
      const format = getDisplayFormat(match);
      if (format && format !== 'Constructed') {
        formats.add(format);
      }
    });
    return Array.from(formats).sort();
  }, [matchList]);

  const uniqueDecks = useMemo(() => {
    const decks = new Map<string, string>(); // id -> name
    matchList.forEach(match => {
      if (match.DeckID && match.DeckName) {
        decks.set(match.DeckID, match.DeckName);
      }
    });
    return Array.from(decks.entries())
      .map(([id, name]) => ({ id, name }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [matchList]);

  const getSortIcon = (field: SortField) => {
    if (sortField !== field) return '⇅';
    return sortDirection === 'asc' ? '↑' : '↓';
  };

  // Get today's date in YYYY-MM-DD format for max date constraint
  const getTodayDateString = () => {
    const today = new Date();
    return today.toISOString().split('T')[0];
  };

  // Get min date for end date (must be >= start date)
  const getMinEndDate = () => {
    return customStartDate || undefined;
  };

  return (
    <div className="page-container">
      {/* Header Section - Fixed */}
      <div className="match-history-header">
        <h1 className="page-title">Match History</h1>

        {/* Filters */}
        <div className="filter-row">
          <div className="filter-group">
            <label className="filter-label">Date Range</label>
            <select
              value={dateRange}
              onChange={(e) => {
                updateFilters('matchHistory', { dateRange: e.target.value });
                trackEvent({ name: 'feature_match_history_filtered', properties: { filter_type: 'date_range', filter_value: e.target.value } });
              }}
            >
              <option value="7days">Last 7 Days</option>
              <option value="30days">Last 30 Days</option>
              <option value="90days">Last 90 Days</option>
              <option value="all">All Time</option>
              <option value="custom">Custom Range</option>
            </select>
          </div>

          {dateRange === 'custom' && (
            <>
              <div className="filter-group">
                <label className="filter-label">Start Date</label>
                <input
                  type="date"
                  value={customStartDate}
                  max={getTodayDateString()}
                  onChange={(e) => updateFilters('matchHistory', { customStartDate: e.target.value })}
                />
              </div>

              <div className="filter-group">
                <label className="filter-label">End Date</label>
                <input
                  type="date"
                  value={customEndDate}
                  min={getMinEndDate()}
                  max={getTodayDateString()}
                  onChange={(e) => updateFilters('matchHistory', { customEndDate: e.target.value })}
                />
              </div>
            </>
          )}

          <div className="filter-group">
            <label className="filter-label">Card Format</label>
            <select
              value={cardFormat}
              onChange={(e) => {
                updateFilters('matchHistory', { cardFormat: e.target.value });
                trackEvent({ name: 'feature_match_history_filtered', properties: { filter_type: 'format', filter_value: e.target.value } });
              }}
            >
              <option value="all">All Card Formats</option>
              <option value="Standard">Standard</option>
              <option value="Historic">Historic</option>
              <option value="Alchemy">Alchemy</option>
              <option value="Explorer">Explorer</option>
              <option value="HistoricBrawl">Historic Brawl</option>
              <option value="Brawl">Brawl</option>
            </select>
          </div>

          <div className="filter-group">
            <label className="filter-label">Queue Type</label>
            <select
              value={queueType}
              onChange={(e) => {
                updateFilters('matchHistory', { queueType: e.target.value });
                trackEvent({ name: 'feature_match_history_filtered', properties: { filter_type: 'format', filter_value: e.target.value } });
              }}
            >
              <option value="all">All Queues</option>
              <option value="Ladder">Ranked</option>
              <option value="Play">Play Queue</option>
            </select>
          </div>

          <div className="filter-group">
            <label className="filter-label">Result</label>
            <select
              value={result}
              onChange={(e) => {
                updateFilters('matchHistory', { result: e.target.value });
                trackEvent({ name: 'feature_match_history_filtered', properties: { filter_type: 'result', filter_value: e.target.value } });
              }}
            >
              <option value="all">All Results</option>
              <option value="win">Wins Only</option>
              <option value="loss">Losses Only</option>
            </select>
          </div>

          {/* Filtered Record Summary */}
          {!loading && matchList.length > 0 && (
            <div className="filter-group record-summary">
              <label className="filter-label">Record</label>
              <span className="record-value">
                {(() => {
                  const wins = matchList.filter(m => m.Result.toLowerCase() === 'win').length;
                  const losses = matchList.filter(m => m.Result.toLowerCase() === 'loss').length;
                  const total = wins + losses;
                  const winRate = total > 0 ? ((wins / total) * 100).toFixed(1) : '0.0';
                  return `${wins}-${losses} (${winRate}%)`;
                })()}
              </span>
            </div>
          )}

          {/* Compare Button */}
          {!loading && matchList.length > 0 && (
            <Tooltip content="Compare performance across formats, decks, or time periods">
              <button
                className="compare-btn"
                onClick={() => setShowComparisonPanel(true)}
              >
                Compare
              </button>
            </Tooltip>
          )}
        </div>

        {!loading && !error && matchList.length > 0 && (
          <div className="match-count">
            Showing {paginatedMatches.length} of {matchList.length} match{matchList.length !== 1 ? 'es' : ''}
            {totalPages > 1 && ` (Page ${page} of ${totalPages})`}
          </div>
        )}
      </div>

      {/* Content - Loading/Error/Empty States */}
      {loading && <LoadingSpinner message="Loading matches..." />}

      {/* Daemon not connected — show first-run empty state */}
      {!loading && daemonChecked && !daemonConnected && (
        <DaemonEmptyState
          page="match_history"
          heading="Daemon not connected"
          subtext="Match History requires the VaultMTG daemon to be running. Download and start the daemon to see your match history."
        />
      )}

      {error && daemonConnected && (
        <ErrorState
          message="Failed to load matches"
          error={error}
          helpText="Check that the database is accessible and try refreshing the page."
        />
      )}

      {!loading && !error && daemonConnected && matchList.length === 0 && (
        dateRange === 'all' && cardFormat === 'all' && queueType === 'all' && result === 'all' ? (
          <EmptyState
            icon="🎮"
            heading="No matches yet"
            subtext="Start playing MTG Arena to begin tracking your match history!"
            variant="no-data"
          />
        ) : (
          <EmptyState
            icon="🔍"
            heading="No matches found"
            subtext="Try adjusting your filters to see more results."
            variant="no-data"
          />
        )
      )}

      {/* Table Container - Scrollable */}
      {!loading && !error && daemonConnected && matchList.length > 0 && (
        <>
          <div className="match-history-table-container">
            <table>
            <thead>
              <tr>
                <th onClick={() => handleSort('Timestamp')} style={{ cursor: 'pointer' }}>
                  <Tooltip content="Click to sort by match time">
                    <span>Time {getSortIcon('Timestamp')}</span>
                  </Tooltip>
                </th>
                <th onClick={() => handleSort('Result')} style={{ cursor: 'pointer' }}>
                  <Tooltip content="Click to sort by win/loss">
                    <span>Result {getSortIcon('Result')}</span>
                  </Tooltip>
                </th>
                <th onClick={() => handleSort('Format')} style={{ cursor: 'pointer' }}>
                  <Tooltip content="Click to sort by game format">
                    <span>Format {getSortIcon('Format')}</span>
                  </Tooltip>
                </th>
                <th onClick={() => handleSort('EventName')} style={{ cursor: 'pointer' }}>
                  <Tooltip content="Click to sort by event name">
                    <span>Event {getSortIcon('EventName')}</span>
                  </Tooltip>
                </th>
                <th>
                  <Tooltip content="Match score (Your wins - Opponent wins)">
                    <span>Score</span>
                  </Tooltip>
                </th>
                <th>
                  <Tooltip content="Opponent player name">
                    <span>Opponent</span>
                  </Tooltip>
                </th>
                <th onClick={() => handleSort('DeckName')} style={{ cursor: 'pointer' }}>
                  <Tooltip content="Click to sort by deck name">
                    <span>Deck {getSortIcon('DeckName')}</span>
                  </Tooltip>
                </th>
                <th>
                  <Tooltip content="Add notes about this match">
                    <span>Notes</span>
                  </Tooltip>
                </th>
              </tr>
            </thead>
            <tbody>
              {paginatedMatches.map((match) => (
                <tr
                  key={match.ID}
                  className={`result-${match.Result.toLowerCase()} clickable-row`}
                  onClick={() => {
                    setSelectedMatch(match);
                    trackEvent({
                      name: 'feature_match_details_opened',
                      properties: {
                        match_result: match.Result.toLowerCase() as 'win' | 'loss' | 'draw',
                        format: match.Format ?? '',
                      },
                    });
                  }}
                  title="Click to view match details"
                >
                  <td>{formatTimestamp(match.Timestamp)}</td>
                  <td>
                    <span className={`result-badge ${match.Result.toLowerCase()}`}>
                      {match.Result.toUpperCase()}
                    </span>
                  </td>
                  <td>{getDisplayFormat(match)}</td>
                  <td>{getDisplayEventName(match)}</td>
                  <td>{formatScore(match.PlayerWins, match.OpponentWins)}</td>
                  <td>{match.OpponentName || '—'}</td>
                  <td className="deck-name-cell" title={match.DeckName || 'Unknown'}>
                    {match.DeckName || '—'}
                  </td>
                  <td>
                    <button
                      className="notes-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        setNotesMatchId(match.ID);
                      }}
                      title="Add/view notes for this match"
                    >
                      📝
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
            </table>
          </div>

          {/* Footer Section - Fixed Pagination */}
          {totalPages > 1 && (
            <div className="match-history-footer">
              <div className="pagination">
              <button
                onClick={() => setPage(1)}
                disabled={page === 1}
                className="pagination-btn"
              >
                First
              </button>
              <button
                onClick={() => setPage(page - 1)}
                disabled={page === 1}
                className="pagination-btn"
              >
                Previous
              </button>
              <span className="pagination-info">
                Page {page} of {totalPages}
              </span>
              <button
                onClick={() => setPage(page + 1)}
                disabled={page === totalPages}
                className="pagination-btn"
              >
                Next
              </button>
              <button
                onClick={() => setPage(totalPages)}
                disabled={page === totalPages}
                className="pagination-btn"
              >
                Last
              </button>
              </div>
            </div>
          )}
        </>
      )}

      {/* Match Details Modal */}
      {selectedMatch && (
        <MatchDetailsModal
          match={selectedMatch}
          onClose={() => setSelectedMatch(null)}
        />
      )}

      {/* Match Notes Modal */}
      <MatchNotesModal
        matchId={notesMatchId || ''}
        isOpen={!!notesMatchId}
        onClose={() => setNotesMatchId(null)}
      />

      {/* Match Comparison Panel */}
      {showComparisonPanel && (
        <div
          className="comparison-panel-overlay"
          onClick={() => setShowComparisonPanel(false)}
          data-testid="comparison-panel-overlay"
        >
          <div
            className="comparison-panel-container"
            onClick={(e) => e.stopPropagation()}
            data-testid="comparison-panel-container"
          >
            <MatchComparisonPanel
              formats={uniqueFormats}
              deckIds={uniqueDecks}
              onClose={() => setShowComparisonPanel(false)}
            />
          </div>
        </div>
      )}
    </div>
  );
};

export default MatchHistory;
