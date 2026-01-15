import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { render } from '../test/utils/testUtils';
import TemporalTrends from './TemporalTrends';
import { mockDrafts } from '@/test/mocks/apiMock';
import { analytics } from '@/types/models';

function createMockTrendAnalysis(
  overrides: Partial<analytics.TrendAnalysisResponse> = {}
): analytics.TrendAnalysisResponse {
  return new analytics.TrendAnalysisResponse({
    periodType: 'weekly',
    setCode: undefined,
    direction: 'improving',
    trends: [
      {
        periodStart: '2025-01-01',
        periodEnd: '2025-01-07',
        draftsCount: 5,
        matchesPlayed: 15,
        matchesWon: 8,
        winRate: 0.53,
        avgDraftGrade: 3.5,
      },
      {
        periodStart: '2025-01-08',
        periodEnd: '2025-01-14',
        draftsCount: 7,
        matchesPlayed: 21,
        matchesWon: 13,
        winRate: 0.62,
        avgDraftGrade: 3.8,
      },
    ],
    summary: {
      totalDrafts: 12,
      totalMatches: 36,
      totalWins: 21,
      overallWinRate: 0.583,
      bestPeriodWinRate: 0.62,
      worstPeriodWinRate: 0.53,
      winRateImprovement: 0.09,
    },
    ...overrides,
  });
}

function createMockLearningCurve(
  overrides: Partial<analytics.LearningCurveResponse> = {}
): analytics.LearningCurveResponse {
  return new analytics.LearningCurveResponse({
    setCode: 'DSK',
    improvement: 0.15,
    isMastered: true,
    periods: [
      { draftNumber: 1, winRate: 0.4, cumulative: 0.4 },
      { draftNumber: 2, winRate: 0.5, cumulative: 0.45 },
      { draftNumber: 3, winRate: 0.6, cumulative: 0.5 },
      { draftNumber: 4, winRate: 0.55, cumulative: 0.51 },
      { draftNumber: 5, winRate: 0.7, cumulative: 0.55 },
    ],
    ...overrides,
  });
}

describe('TemporalTrends Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading and Error States', () => {
    it('should display loading state while fetching trends', () => {
      mockDrafts.getTemporalTrends.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<TemporalTrends />);

      expect(screen.getByText('Loading trend data...')).toBeInTheDocument();
    });

    it('should display error message when fetching fails', async () => {
      mockDrafts.getTemporalTrends.mockRejectedValue(new Error('Failed to load trend data'));

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText(/Error: Failed to load trend data/i)).toBeInTheDocument();
      });
    });

    it('should display empty state when no trends are available', async () => {
      const emptyTrends = createMockTrendAnalysis({ trends: [] });
      mockDrafts.getTemporalTrends.mockResolvedValue(emptyTrends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(
          screen.getByText(/No trend data available yet/i)
        ).toBeInTheDocument();
      });
    });
  });

  describe('Summary Statistics Display', () => {
    it('should display total drafts count', async () => {
      const trends = createMockTrendAnalysis();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Total Drafts')).toBeInTheDocument();
        expect(screen.getByText('12')).toBeInTheDocument();
      });
    });

    it('should display overall win rate', async () => {
      const trends = createMockTrendAnalysis();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Overall Win Rate')).toBeInTheDocument();
        expect(screen.getByText('58%')).toBeInTheDocument();
      });
    });

    it('should display improvement indicator for improving trend', async () => {
      const trends = createMockTrendAnalysis({ direction: 'improving' });
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Improvement')).toBeInTheDocument();
      });
    });

    it('should display decline indicator for declining trend', async () => {
      const trends = createMockTrendAnalysis({
        direction: 'declining',
        summary: {
          totalDrafts: 12,
          totalMatches: 36,
          totalWins: 21,
          overallWinRate: 0.583,
          bestPeriodWinRate: 0.62,
          worstPeriodWinRate: 0.53,
          winRateImprovement: -0.09,
        },
      });
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Decline')).toBeInTheDocument();
      });
    });

    it('should display best period win rate', async () => {
      const trends = createMockTrendAnalysis();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Best Period')).toBeInTheDocument();
        expect(screen.getByText('62%')).toBeInTheDocument();
      });
    });
  });

  describe('Chart Display', () => {
    it('should display Win Rate Over Time chart title', async () => {
      const trends = createMockTrendAnalysis();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Win Rate Over Time')).toBeInTheDocument();
      });
    });
  });

  describe('Period Type Selection', () => {
    it('should allow switching between weekly and monthly views', async () => {
      const trends = createMockTrendAnalysis();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Draft Performance Trends')).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox');
      fireEvent.change(select, { target: { value: 'monthly' } });

      // Should trigger a re-fetch with monthly period type
      await waitFor(() => {
        expect(mockDrafts.getTemporalTrends).toHaveBeenCalledWith(
          expect.objectContaining({ period_type: 'monthly' })
        );
      });
    });
  });

  describe('Learning Curve Display', () => {
    it('should display learning curve when showLearningCurve is true and setCode is provided', async () => {
      const trends = createMockTrendAnalysis();
      const learningCurve = createMockLearningCurve();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);
      mockDrafts.getLearningCurve.mockResolvedValue(learningCurve);

      render(<TemporalTrends setCode="DSK" showLearningCurve={true} />);

      await waitFor(() => {
        expect(screen.getByText(/Learning Curve - DSK/i)).toBeInTheDocument();
      });
    });

    it('should display mastered badge when format is mastered', async () => {
      const trends = createMockTrendAnalysis();
      const learningCurve = createMockLearningCurve({ isMastered: true });
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);
      mockDrafts.getLearningCurve.mockResolvedValue(learningCurve);

      render(<TemporalTrends setCode="DSK" showLearningCurve={true} />);

      await waitFor(() => {
        expect(screen.getByText('Mastered!')).toBeInTheDocument();
      });
    });

    it('should not fetch learning curve when showLearningCurve is false', async () => {
      const trends = createMockTrendAnalysis();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends setCode="DSK" showLearningCurve={false} />);

      await waitFor(() => {
        expect(screen.getByText('Draft Performance Trends')).toBeInTheDocument();
      });

      expect(mockDrafts.getLearningCurve).not.toHaveBeenCalled();
    });
  });

  describe('Refresh Functionality', () => {
    it('should allow manual refresh of trend data', async () => {
      const trends = createMockTrendAnalysis();
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends />);

      await waitFor(() => {
        expect(screen.getByText('Draft Performance Trends')).toBeInTheDocument();
      });

      // Clear mock calls after initial load
      mockDrafts.getTemporalTrends.mockClear();

      // Click refresh button
      const refreshButton = screen.getByText('Refresh');
      fireEvent.click(refreshButton);

      await waitFor(() => {
        expect(mockDrafts.getTemporalTrends).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('Set Code Filtering', () => {
    it('should pass set code to API request when provided', async () => {
      const trends = createMockTrendAnalysis({ setCode: 'DSK' });
      mockDrafts.getTemporalTrends.mockResolvedValue(trends);

      render(<TemporalTrends setCode="DSK" />);

      await waitFor(() => {
        expect(mockDrafts.getTemporalTrends).toHaveBeenCalledWith(
          expect.objectContaining({ set_code: 'DSK' })
        );
      });
    });
  });
});
