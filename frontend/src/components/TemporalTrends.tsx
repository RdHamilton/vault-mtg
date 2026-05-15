import React, { useState, useEffect, useCallback } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  Area,
  ComposedChart,
} from 'recharts';
import { apiAdapter } from '@/services/adapter';
import type { TrendAnalysisResponse, TrendEntry, LearningCurveResponse } from '@/services/api/drafts';
import './TemporalTrends.css';

interface TemporalTrendsProps {
  setCode?: string;
  periodType?: 'week' | 'month';
  numPeriods?: number;
  showLearningCurve?: boolean;
  autoRefresh?: boolean;
}

interface ChartDataPoint {
  label: string;
  winRate: number;
  draftsCount: number;
  matchesPlayed: number;
  avgDraftGrade?: number;
}

interface LearningChartDataPoint {
  draftNumber: number;
  winRate: number;
  cumulative: number;
}

const TemporalTrends: React.FC<TemporalTrendsProps> = ({
  setCode,
  periodType = 'week',
  numPeriods = 12,
  showLearningCurve = false,
  autoRefresh = false,
}) => {
  const [trendData, setTrendData] = useState<TrendAnalysisResponse | null>(null);
  const [learningData, setLearningData] = useState<LearningCurveResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedPeriodType, setSelectedPeriodType] = useState<'week' | 'month'>(periodType);

  const fetchTrends = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const trends = await apiAdapter.drafts.getTemporalTrends({
        period_type: selectedPeriodType,
        num_periods: numPeriods,
        set_code: setCode,
      });
      setTrendData(trends);

      // Fetch learning curve if set code is provided
      if (showLearningCurve && setCode) {
        const curve = await apiAdapter.drafts.getLearningCurve(setCode);
        setLearningData(curve);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load trend data');
    } finally {
      setLoading(false);
    }
  }, [selectedPeriodType, numPeriods, setCode, showLearningCurve]);

  useEffect(() => {
    fetchTrends();
  }, [fetchTrends]);

  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(fetchTrends, 30000);
    return () => clearInterval(interval);
  }, [autoRefresh, fetchTrends]);

  const formatChartData = (trends: TrendEntry[]): ChartDataPoint[] => {
    return trends.map((trend) => ({
      label: formatDateRange(trend.periodStart, trend.periodEnd),
      winRate: Math.round(trend.winRate * 100),
      draftsCount: trend.draftsCount,
      matchesPlayed: trend.matchesPlayed,
      avgDraftGrade: trend.avgDraftGrade,
    }));
  };

  const formatDateRange = (start: string, end: string): string => {
    const startDate = new Date(start);
    const endDate = new Date(end);
    return `${startDate.getMonth() + 1}/${startDate.getDate()}-${endDate.getMonth() + 1}/${endDate.getDate()}`;
  };

  const formatLearningData = (curve: LearningCurveResponse): LearningChartDataPoint[] => {
    return curve.periods.map((period) => ({
      draftNumber: period.draftNumber,
      winRate: Math.round(period.winRate * 100),
      cumulative: Math.round(period.cumulative * 100),
    }));
  };

  const getDirectionIcon = (direction: string): string => {
    switch (direction) {
      case 'improving':
        return '+';
      case 'declining':
        return '-';
      default:
        return '~';
    }
  };

  const getDirectionColor = (direction: string): string => {
    switch (direction) {
      case 'improving':
        return 'var(--color-success)';
      case 'declining':
        return 'var(--color-error)';
      default:
        return 'var(--color-text-secondary)';
    }
  };

  if (loading) {
    return (
      <div className="temporal-trends temporal-trends--loading" data-testid="temporal-trends-loading">
        <div className="temporal-trends__spinner" />
        <span>Loading trend data...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="temporal-trends temporal-trends--error" data-testid="temporal-trends-error">
        <span>Error: {error}</span>
        <button onClick={fetchTrends} data-testid="temporal-trends-retry-button">Retry</button>
      </div>
    );
  }

  if (!trendData || trendData.trends.length === 0) {
    return (
      <div className="temporal-trends temporal-trends--empty" data-testid="temporal-trends-empty">
        <span>No trend data available yet. Complete some drafts to see your performance over time.</span>
      </div>
    );
  }

  const chartData = formatChartData(trendData.trends);
  const learningChartData = learningData ? formatLearningData(learningData) : [];

  return (
    <div className="temporal-trends" data-testid="temporal-trends">
      <div className="temporal-trends__header">
        <h3>Draft Performance Trends</h3>
        <div className="temporal-trends__controls">
          <select
            value={selectedPeriodType}
            onChange={(e) => setSelectedPeriodType(e.target.value as 'week' | 'month')}
            className="temporal-trends__select"
            data-testid="temporal-trends-period-select"
          >
            <option value="week">Weekly</option>
            <option value="month">Monthly</option>
          </select>
          <button onClick={fetchTrends} className="temporal-trends__refresh" data-testid="temporal-trends-refresh-button">
            Refresh
          </button>
        </div>
      </div>

      <div className="temporal-trends__summary" data-testid="temporal-trends-summary">
        <div className="temporal-trends__stat">
          <span className="temporal-trends__stat-value">{trendData.summary.totalDrafts}</span>
          <span className="temporal-trends__stat-label">Total Drafts</span>
        </div>
        <div className="temporal-trends__stat">
          <span className="temporal-trends__stat-value">
            {Math.round(trendData.summary.overallWinRate * 100)}%
          </span>
          <span className="temporal-trends__stat-label">Overall Win Rate</span>
        </div>
        <div className="temporal-trends__stat">
          <span
            className="temporal-trends__stat-value"
            style={{ color: getDirectionColor(trendData.direction) }}
          >
            {getDirectionIcon(trendData.direction)}{' '}
            {Math.round(Math.abs(trendData.summary.winRateImprovement) * 100)}%
          </span>
          <span className="temporal-trends__stat-label">
            {trendData.direction === 'improving' ? 'Improvement' : trendData.direction === 'declining' ? 'Decline' : 'Stable'}
          </span>
        </div>
        <div className="temporal-trends__stat">
          <span className="temporal-trends__stat-value">
            {Math.round(trendData.summary.bestPeriodWinRate * 100)}%
          </span>
          <span className="temporal-trends__stat-label">Best Period</span>
        </div>
      </div>

      <div className="temporal-trends__chart">
        <h4>Win Rate Over Time</h4>
        <ResponsiveContainer width="100%" height={250}>
          <ComposedChart data={chartData}>
            <CartesianGrid strokeDasharray="3 3" stroke="#444" />
            <XAxis dataKey="label" stroke="#aaa" tick={{ fontSize: 12 }} />
            <YAxis
              yAxisId={0}
              stroke="#aaa"
              domain={[0, 100]}
              tickFormatter={(value) => `${value}%`}
            />
            <YAxis
              yAxisId={1}
              orientation="right"
              stroke="#82ca9d"
              tick={{ fontSize: 11 }}
              allowDecimals={false}
            />
            <Tooltip
              contentStyle={{ backgroundColor: '#2d2d2d', border: '1px solid #444' }}
              formatter={(value: number, name: string) => {
                if (name === 'winRate') return [`${value}%`, 'Win Rate'];
                if (name === 'draftsCount') return [value, 'Drafts'];
                return [value, name];
              }}
            />
            <Legend />
            <Area
              type="monotone"
              dataKey="winRate"
              fill="#4a9eff33"
              stroke="#4a9eff"
              name="Win Rate"
              yAxisId={0}
            />
            <Line
              type="monotone"
              dataKey="draftsCount"
              stroke="#82ca9d"
              strokeDasharray="5 5"
              name="Drafts"
              yAxisId={1}
              dot={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {showLearningCurve && learningChartData.length > 0 && (
        <div className="temporal-trends__chart">
          <h4>
            Learning Curve - {setCode}
            {learningData?.isMastered && (
              <span className="temporal-trends__mastered">Mastered!</span>
            )}
          </h4>
          <ResponsiveContainer width="100%" height={200}>
            <LineChart data={learningChartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#444" />
              <XAxis
                dataKey="draftNumber"
                stroke="#aaa"
                tick={{ fontSize: 12 }}
                label={{ value: 'Draft #', position: 'insideBottom', offset: -5 }}
              />
              <YAxis
                stroke="#aaa"
                domain={[0, 100]}
                tickFormatter={(value) => `${value}%`}
              />
              <Tooltip
                contentStyle={{ backgroundColor: '#2d2d2d', border: '1px solid #444' }}
                formatter={(value: number, name: string) => {
                  if (name === 'winRate') return [`${value}%`, 'Draft Win Rate'];
                  if (name === 'cumulative') return [`${value}%`, 'Cumulative'];
                  return [value, name];
                }}
              />
              <Legend />
              <Line
                type="monotone"
                dataKey="winRate"
                stroke="#ff7f50"
                name="Draft Win Rate"
                dot={{ r: 3 }}
              />
              <Line
                type="monotone"
                dataKey="cumulative"
                stroke="#4a9eff"
                name="Cumulative"
                strokeWidth={2}
              />
            </LineChart>
          </ResponsiveContainer>
          {learningData && (
            <div className="temporal-trends__learning-summary">
              <span>
                Improvement: {Math.round(learningData.improvement * 100)}%
                {learningData.improvement > 0 ? ' better' : learningData.improvement < 0 ? ' worse' : ''}
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default TemporalTrends;
