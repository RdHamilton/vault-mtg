import React, { useState, useEffect, useCallback } from 'react';
import { apiAdapter } from '@/services/adapter';
import type { CommunityComparisonResponse, ArchetypeComparisonEntry } from '@/services/api/drafts';
import './CommunityComparison.css';

interface CommunityComparisonProps {
  setCode: string;
  draftFormat?: string;
  autoRefresh?: boolean;
}

const CommunityComparison: React.FC<CommunityComparisonProps> = ({
  setCode,
  draftFormat = 'PremierDraft',
  autoRefresh = false,
}) => {
  const [comparison, setComparison] = useState<CommunityComparisonResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchComparison = useCallback(async () => {
    if (!setCode) return;

    try {
      setLoading(true);
      setError(null);

      const data = await apiAdapter.drafts.getCommunityComparison({
        set_code: setCode,
        draft_format: draftFormat,
      });
      setComparison(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load community comparison');
    } finally {
      setLoading(false);
    }
  }, [setCode, draftFormat]);

  useEffect(() => {
    fetchComparison();
  }, [fetchComparison]);

  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(fetchComparison, 60000);
    return () => clearInterval(interval);
  }, [autoRefresh, fetchComparison]);

  const getRankClass = (rank: string): string => {
    switch (rank) {
      case 'Top 5%':
      case 'Top 10%':
        return 'community-comparison__rank--elite';
      case 'Top 20%':
        return 'community-comparison__rank--high';
      case 'Above Average':
        return 'community-comparison__rank--above';
      case 'Average':
        return 'community-comparison__rank--average';
      case 'Below Average':
        return 'community-comparison__rank--below';
      default:
        return 'community-comparison__rank--low';
    }
  };

  const formatPercent = (value: number): string => {
    return `${Math.round(value * 100)}%`;
  };

  const formatDelta = (value: number): string => {
    const percent = Math.round(value * 100);
    return percent >= 0 ? `+${percent}%` : `${percent}%`;
  };

  if (loading) {
    return (
      <div className="community-comparison community-comparison--loading">
        <div className="community-comparison__spinner" />
        <span>Loading community comparison...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="community-comparison community-comparison--error">
        <span>Error: {error}</span>
        <button onClick={fetchComparison}>Retry</button>
      </div>
    );
  }

  if (!comparison || comparison.sampleSize === 0) {
    return (
      <div className="community-comparison community-comparison--empty">
        <span>No draft data available for {setCode}. Complete some drafts to see your comparison.</span>
      </div>
    );
  }

  return (
    <div className="community-comparison">
      <div className="community-comparison__header">
        <h3>Community Comparison</h3>
        <span className="community-comparison__set">{comparison.setCode}</span>
      </div>

      <div className="community-comparison__main">
        <div className={`community-comparison__rank ${getRankClass(comparison.rank)}`}>
          <span className="community-comparison__rank-label">{comparison.rank}</span>
          <span className="community-comparison__percentile">
            {Math.round(comparison.percentileRank)}th percentile
          </span>
        </div>

        <div className="community-comparison__stats">
          <div className="community-comparison__stat">
            <span className="community-comparison__stat-label">Your Win Rate</span>
            <span className="community-comparison__stat-value community-comparison__stat-value--primary">
              {formatPercent(comparison.userWinRate)}
            </span>
          </div>
          <div className="community-comparison__stat">
            <span className="community-comparison__stat-label">Community Avg</span>
            <span className="community-comparison__stat-value">
              {formatPercent(comparison.communityAvgWinRate)}
            </span>
          </div>
          <div className="community-comparison__stat">
            <span className="community-comparison__stat-label">Difference</span>
            <span
              className={`community-comparison__stat-value ${
                comparison.winRateDelta >= 0
                  ? 'community-comparison__stat-value--positive'
                  : 'community-comparison__stat-value--negative'
              }`}
            >
              {formatDelta(comparison.winRateDelta)}
            </span>
          </div>
          <div className="community-comparison__stat">
            <span className="community-comparison__stat-label">Matches</span>
            <span className="community-comparison__stat-value">{comparison.sampleSize}</span>
          </div>
        </div>
      </div>

      {comparison.archetypeComparison && comparison.archetypeComparison.length > 0 && (
        <div className="community-comparison__archetypes">
          <h4>Archetype Performance vs Community</h4>
          <div className="community-comparison__archetype-list">
            {comparison.archetypeComparison
              .sort((a, b) => b.userMatchesPlayed - a.userMatchesPlayed)
              .slice(0, 6)
              .map((arch) => (
                <ArchetypeRow key={arch.colorCombination} archetype={arch} />
              ))}
          </div>
        </div>
      )}
    </div>
  );
};

interface ArchetypeRowProps {
  archetype: ArchetypeComparisonEntry;
}

const ArchetypeRow: React.FC<ArchetypeRowProps> = ({ archetype }) => {
  const formatPercent = (value: number): string => {
    return `${Math.round(value * 100)}%`;
  };

  const formatDelta = (value: number): string => {
    const percent = Math.round(value * 100);
    return percent >= 0 ? `+${percent}%` : `${percent}%`;
  };

  return (
    <div className="community-comparison__archetype-row">
      <div className="community-comparison__archetype-colors">
        {archetype.colorCombination.split('').map((color, idx) => (
          <span
            key={idx}
            className={`community-comparison__color community-comparison__color--${color.toLowerCase()}`}
          >
            {color}
          </span>
        ))}
      </div>
      <div className="community-comparison__archetype-name">
        {archetype.archetypeName || archetype.colorCombination}
      </div>
      <div className="community-comparison__archetype-stats">
        <span className="community-comparison__archetype-user">
          {formatPercent(archetype.userWinRate)}
        </span>
        <span className="community-comparison__archetype-vs">vs</span>
        <span className="community-comparison__archetype-community">
          {formatPercent(archetype.communityWinRate)}
        </span>
        <span
          className={`community-comparison__archetype-delta ${
            archetype.isAboveCommunity
              ? 'community-comparison__archetype-delta--positive'
              : 'community-comparison__archetype-delta--negative'
          }`}
        >
          ({formatDelta(archetype.winRateDelta)})
        </span>
      </div>
      <div className="community-comparison__archetype-matches">
        {archetype.userMatchesPlayed} matches
      </div>
    </div>
  );
};

export default CommunityComparison;
