import { useState } from 'react';
import type {
  ComparisonResult,
  ComparisonGroup,
  StatsFilterRequest,
} from '@/services/api/matches';
import {
  compareFormats,
  compareDecks,
  compareTimePeriods,
} from '@/services/api/matches';
import { reportError } from '@/lib/sentry';
import './MatchComparisonPanel.css';

interface MatchComparisonPanelProps {
  formats?: string[];
  deckIds?: { id: string; name: string }[];
  onClose?: () => void;
}

type ComparisonType = 'formats' | 'decks' | 'time-periods';

interface TimePeriodOption {
  label: string;
  startDate: string;
  endDate: string;
}

function formatWinRate(rate: number | undefined): string {
  if (rate === undefined || rate === null) return '0%';
  return `${(rate * 100).toFixed(1)}%`;
}

function formatDiff(diff: number): string {
  const percentage = (diff * 100).toFixed(1);
  if (diff > 0) return `+${percentage}%`;
  if (diff < 0) return `${percentage}%`;
  return '0%';
}

function getDiffClass(diff: number): string {
  if (diff > 0.05) return 'diff-positive';
  if (diff < -0.05) return 'diff-negative';
  return 'diff-neutral';
}

function getDefaultTimePeriods(): TimePeriodOption[] {
  const now = new Date();
  const today = now.toISOString().split('T')[0];

  // Last 7 days
  const weekAgo = new Date(now);
  weekAgo.setDate(weekAgo.getDate() - 7);
  const weekAgoStr = weekAgo.toISOString().split('T')[0];

  // Last 30 days
  const monthAgo = new Date(now);
  monthAgo.setDate(monthAgo.getDate() - 30);
  const monthAgoStr = monthAgo.toISOString().split('T')[0];

  // Previous 30 days (30-60 days ago)
  const twoMonthsAgo = new Date(now);
  twoMonthsAgo.setDate(twoMonthsAgo.getDate() - 60);
  const twoMonthsAgoStr = twoMonthsAgo.toISOString().split('T')[0];

  return [
    { label: 'Last 7 Days', startDate: weekAgoStr, endDate: today },
    { label: 'Last 30 Days', startDate: monthAgoStr, endDate: today },
    { label: 'Previous 30 Days', startDate: twoMonthsAgoStr, endDate: monthAgoStr },
  ];
}

export default function MatchComparisonPanel({
  formats = [],
  deckIds = [],
  onClose,
}: MatchComparisonPanelProps) {
  const [comparisonType, setComparisonType] = useState<ComparisonType>('formats');
  const [selectedFormats, setSelectedFormats] = useState<string[]>([]);
  const [selectedDecks, setSelectedDecks] = useState<string[]>([]);
  const [selectedPeriods, setSelectedPeriods] = useState<TimePeriodOption[]>([]);
  const [result, setResult] = useState<ComparisonResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const timePeriodOptions = getDefaultTimePeriods();

  const handleCompare = async () => {
    setLoading(true);
    setError(null);
    try {
      let comparisonResult: ComparisonResult;
      const baseFilter: StatsFilterRequest = {};

      switch (comparisonType) {
        case 'formats':
          if (selectedFormats.length < 2) {
            setError('Please select at least 2 formats to compare');
            setLoading(false);
            return;
          }
          comparisonResult = await compareFormats({
            formats: selectedFormats,
            baseFilter: baseFilter,
          });
          break;
        case 'decks':
          if (selectedDecks.length < 2) {
            setError('Please select at least 2 decks to compare');
            setLoading(false);
            return;
          }
          comparisonResult = await compareDecks({
            deckIDs: selectedDecks,
            baseFilter: baseFilter,
          });
          break;
        case 'time-periods':
          if (selectedPeriods.length < 2) {
            setError('Please select at least 2 time periods to compare');
            setLoading(false);
            return;
          }
          comparisonResult = await compareTimePeriods({
            periods: selectedPeriods.map((p) => ({
              label: p.label,
              startDate: p.startDate,
              endDate: p.endDate,
            })),
            baseFilter: baseFilter,
          });
          break;
        default:
          throw new Error('Invalid comparison type');
      }

      setResult(comparisonResult);
    } catch (err) {
      reportError(err, { component: 'MatchComparisonPanel', action: 'compare_matches' });
      setError(err instanceof Error ? err.message : 'Failed to compare matches');
    } finally {
      setLoading(false);
    }
  };

  const toggleFormat = (format: string) => {
    setSelectedFormats((prev) =>
      prev.includes(format) ? prev.filter((f) => f !== format) : [...prev, format]
    );
  };

  const toggleDeck = (deckId: string) => {
    setSelectedDecks((prev) =>
      prev.includes(deckId) ? prev.filter((d) => d !== deckId) : [...prev, deckId]
    );
  };

  const togglePeriod = (period: TimePeriodOption) => {
    setSelectedPeriods((prev) => {
      const exists = prev.some((p) => p.label === period.label);
      if (exists) {
        return prev.filter((p) => p.label !== period.label);
      }
      return [...prev, period];
    });
  };

  const renderSelector = () => {
    switch (comparisonType) {
      case 'formats':
        return (
          <div className="comparison-selector" data-testid="format-selector">
            <h4>Select Formats to Compare</h4>
            <div className="selector-options">
              {formats.length > 0 ? (
                formats.map((format) => (
                  <label key={format} className="selector-option" data-testid={`format-option-${format}`}>
                    <input
                      type="checkbox"
                      checked={selectedFormats.includes(format)}
                      onChange={() => toggleFormat(format)}
                      data-testid={`format-checkbox-${format}`}
                    />
                    <span>{format}</span>
                  </label>
                ))
              ) : (
                <p className="no-options" data-testid="no-formats-message">No formats available. Play some matches first.</p>
              )}
            </div>
          </div>
        );
      case 'decks':
        return (
          <div className="comparison-selector" data-testid="deck-selector">
            <h4>Select Decks to Compare</h4>
            <div className="selector-options">
              {deckIds.length > 0 ? (
                deckIds.map((deck) => (
                  <label key={deck.id} className="selector-option" data-testid={`deck-option-${deck.id}`}>
                    <input
                      type="checkbox"
                      checked={selectedDecks.includes(deck.id)}
                      onChange={() => toggleDeck(deck.id)}
                      data-testid={`deck-checkbox-${deck.id}`}
                    />
                    <span>{deck.name}</span>
                  </label>
                ))
              ) : (
                <p className="no-options" data-testid="no-decks-message">No decks available. Create some decks first.</p>
              )}
            </div>
          </div>
        );
      case 'time-periods':
        return (
          <div className="comparison-selector" data-testid="time-period-selector">
            <h4>Select Time Periods to Compare</h4>
            <div className="selector-options">
              {timePeriodOptions.map((period) => (
                <label key={period.label} className="selector-option" data-testid={`period-option-${period.label.replace(/\s+/g, '-')}`}>
                  <input
                    type="checkbox"
                    checked={selectedPeriods.some((p) => p.label === period.label)}
                    onChange={() => togglePeriod(period)}
                    data-testid={`period-checkbox-${period.label.replace(/\s+/g, '-')}`}
                  />
                  <span>{period.label}</span>
                </label>
              ))}
            </div>
          </div>
        );
      default:
        return null;
    }
  };

  // Helper to get display label - looks up deck name if comparing decks
  const getDisplayLabel = (label: string): string => {
    if (comparisonType === 'decks') {
      const deck = deckIds.find((d) => d.id === label);
      return deck?.name || label;
    }
    return label;
  };

  const renderGroupRow = (group: ComparisonGroup, isBest: boolean, isWorst: boolean) => {
    const stats = group.Statistics;
    const displayLabel = getDisplayLabel(group.Label);
    return (
      <tr
        key={group.Label}
        className={`${isBest ? 'best-group' : ''} ${isWorst ? 'worst-group' : ''}`}
      >
        <td className="group-label">
          {displayLabel}
          {isBest && <span className="badge badge-best">Best</span>}
          {isWorst && <span className="badge badge-worst">Worst</span>}
        </td>
        <td className="stat-value">{group.MatchCount}</td>
        <td className="stat-value win-rate">{formatWinRate(stats?.WinRate)}</td>
        <td className="stat-value">{formatWinRate(stats?.GameWinRate)}</td>
        <td className="stat-value">{stats?.TotalGames || 0}</td>
      </tr>
    );
  };

  const renderInsight = () => {
    if (!result?.BestGroup || !result?.WorstGroup) return null;

    const best = result.BestGroup;
    const worst = result.WorstGroup;
    const bestName = getDisplayLabel(best.Label);
    const worstName = getDisplayLabel(worst.Label);
    const bestWinRate = best.Statistics?.WinRate || 0;
    const worstWinRate = worst.Statistics?.WinRate || 0;
    const bestGameWinRate = best.Statistics?.GameWinRate || 0;
    const worstGameWinRate = worst.Statistics?.GameWinRate || 0;
    const winRateDiff = (bestWinRate - worstWinRate) * 100;
    const bestMatches = best.MatchCount || 0;
    const worstMatches = worst.MatchCount || 0;

    const insights: React.ReactNode[] = [];

    // Primary comparison insight
    if (comparisonType === 'decks') {
      insights.push(
        <>
          <strong>{bestName}</strong> outperforms <strong>{worstName}</strong> by{' '}
          <span className="highlight">{winRateDiff.toFixed(1)} percentage points</span> in match win rate.
        </>
      );

      // Sample size warning
      if (bestMatches < 10 || worstMatches < 10) {
        const lowSampleDeck = bestMatches < worstMatches ? bestName : worstName;
        const lowCount = Math.min(bestMatches, worstMatches);
        insights.push(
          <>
            <em>Note:</em> {lowSampleDeck} has only {lowCount} matches — results may change with more games.
          </>
        );
      }

      // Game win rate vs match win rate analysis
      const bestGameMatchDiff = (bestGameWinRate - bestWinRate) * 100;
      const worstGameMatchDiff = (worstGameWinRate - worstWinRate) * 100;

      if (Math.abs(bestGameMatchDiff) > 5) {
        if (bestGameMatchDiff > 5) {
          insights.push(
            <>
              {bestName}&apos;s game win rate is higher than match win rate, suggesting you&apos;re winning games but losing close matches — consider sideboard adjustments.
            </>
          );
        } else if (bestGameMatchDiff < -5) {
          insights.push(
            <>
              {bestName} converts games to match wins efficiently — you&apos;re strong in best-of-three scenarios.
            </>
          );
        }
      }

      if (Math.abs(worstGameMatchDiff) > 5 && worstMatches >= 10) {
        if (worstGameMatchDiff > 5) {
          insights.push(
            <>
              {worstName} wins individual games but struggles to close out matches — sideboard strategy may need work.
            </>
          );
        }
      }

      // Recommendation based on data
      if (winRateDiff > 20 && bestMatches >= 10) {
        insights.push(
          <>
            With a {winRateDiff.toFixed(0)}%+ advantage, <strong>{bestName}</strong> appears to be significantly stronger in your hands. Consider what makes it work for you.
          </>
        );
      } else if (winRateDiff > 10 && worstMatches >= 15) {
        insights.push(
          <>
            <strong>{worstName}</strong> might benefit from more practice or meta adjustments — the sample size suggests this isn&apos;t just variance.
          </>
        );
      }
    } else if (comparisonType === 'formats') {
      insights.push(
        <>
          You perform best in <strong>{bestName}</strong> ({formatWinRate(bestWinRate)}) compared to <strong>{worstName}</strong> ({formatWinRate(worstWinRate)}).
        </>
      );

      if (winRateDiff > 15) {
        insights.push(
          <>
            Consider focusing on {bestName} for climbing — your {winRateDiff.toFixed(0)}% edge is significant.
          </>
        );
      }
    } else if (comparisonType === 'time-periods') {
      insights.push(
        <>
          Your win rate was highest during <strong>{bestName}</strong> ({formatWinRate(bestWinRate)}).
        </>
      );

      if (bestWinRate > worstWinRate) {
        insights.push(
          <>
            Performance improved by {winRateDiff.toFixed(1)} percentage points compared to {worstName}.
          </>
        );
      } else {
        insights.push(
          <>
            Recent performance in {worstName} shows room for improvement.
          </>
        );
      }
    }

    return (
      <>
        {insights.map((insight, index) => (
          <p key={index}>{insight}</p>
        ))}
      </>
    );
  };

  const renderResults = () => {
    if (!result) return null;

    return (
      <div className="comparison-results" data-testid="comparison-results">
        <div className="results-header" data-testid="results-header">
          <h4>Comparison Results</h4>
          <div className="results-summary" data-testid="results-summary">
            <span data-testid="total-matches">Total Matches: {result.TotalMatches}</span>
            <span className={getDiffClass(result.WinRateDiff)} data-testid="win-rate-spread">
              Win Rate Spread: {formatDiff(result.WinRateDiff)}
            </span>
          </div>
        </div>

        <table className="comparison-table" data-testid="comparison-table">
          <thead>
            <tr>
              <th>Group</th>
              <th>Matches</th>
              <th>Win Rate</th>
              <th>Game Win Rate</th>
              <th>Games</th>
            </tr>
          </thead>
          <tbody>
            {result.Groups.map((group) =>
              renderGroupRow(
                group,
                result.BestGroup?.Label === group.Label,
                result.WorstGroup?.Label === group.Label
              )
            )}
          </tbody>
        </table>

        {result.BestGroup && result.WorstGroup && result.BestGroup.Label !== result.WorstGroup.Label && (
          <div className="comparison-insight" data-testid="comparison-insight">
            {renderInsight()}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="match-comparison-panel" data-testid="match-comparison-panel">
      <div className="panel-header" data-testid="comparison-header">
        <h3>Match Comparison</h3>
        {onClose && (
          <button className="close-button" onClick={onClose} data-testid="comparison-close-button">
            &times;
          </button>
        )}
      </div>

      <div className="comparison-type-selector" data-testid="comparison-type-selector">
        <button
          className={`type-button ${comparisonType === 'formats' ? 'active' : ''}`}
          onClick={() => setComparisonType('formats')}
          data-testid="compare-formats-button"
        >
          Compare Formats
        </button>
        <button
          className={`type-button ${comparisonType === 'decks' ? 'active' : ''}`}
          onClick={() => setComparisonType('decks')}
          data-testid="compare-decks-button"
        >
          Compare Decks
        </button>
        <button
          className={`type-button ${comparisonType === 'time-periods' ? 'active' : ''}`}
          onClick={() => setComparisonType('time-periods')}
          data-testid="compare-time-periods-button"
        >
          Compare Time Periods
        </button>
      </div>

      {renderSelector()}

      {error && <div className="comparison-error" data-testid="comparison-error">{error}</div>}

      <div className="comparison-actions" data-testid="comparison-actions">
        <button className="compare-button" onClick={handleCompare} disabled={loading} data-testid="compare-button">
          {loading ? 'Comparing...' : 'Compare'}
        </button>
      </div>

      {renderResults()}
    </div>
  );
}
