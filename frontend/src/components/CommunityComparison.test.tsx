import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { render } from '../test/utils/testUtils';
import CommunityComparison from './CommunityComparison';
import { mockDrafts } from '@/test/mocks/apiMock';
import { analytics } from '@/types/models';

function createMockComparison(
  overrides: Partial<analytics.CommunityComparisonResponse> = {}
): analytics.CommunityComparisonResponse {
  return new analytics.CommunityComparisonResponse({
    setCode: 'DSK',
    draftFormat: 'PremierDraft',
    userWinRate: 0.58,
    communityAvgWinRate: 0.52,
    winRateDelta: 0.06,
    percentileRank: 68,
    sampleSize: 30,
    rank: 'Above Average',
    archetypeComparison: [
      {
        colorCombination: 'WG',
        archetypeName: 'Selesnya',
        userWinRate: 0.65,
        communityWinRate: 0.54,
        winRateDelta: 0.11,
        userMatchesPlayed: 12,
        percentileRank: 80,
        isAboveCommunity: true,
      },
      {
        colorCombination: 'UB',
        archetypeName: 'Dimir',
        userWinRate: 0.48,
        communityWinRate: 0.52,
        winRateDelta: -0.04,
        userMatchesPlayed: 8,
        percentileRank: 38,
        isAboveCommunity: false,
      },
    ],
    ...overrides,
  });
}

describe('CommunityComparison Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading and Error States', () => {
    it('should display loading state while fetching comparison', () => {
      mockDrafts.getCommunityComparison.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<CommunityComparison setCode="DSK" />);

      expect(screen.getByText('Loading community comparison...')).toBeInTheDocument();
    });

    it('should display error message when fetching fails', async () => {
      mockDrafts.getCommunityComparison.mockRejectedValue(new Error('Failed to load comparison'));

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText(/Error: Failed to load comparison/i)).toBeInTheDocument();
      });
    });

    it('should display empty state when no data is available', async () => {
      const emptyComparison = createMockComparison({ sampleSize: 0 });
      mockDrafts.getCommunityComparison.mockResolvedValue(emptyComparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(
          screen.getByText(/No draft data available for DSK/i)
        ).toBeInTheDocument();
      });
    });
  });

  describe('Main Comparison Display', () => {
    it('should display user win rate', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Your Win Rate')).toBeInTheDocument();
        expect(screen.getByText('58%')).toBeInTheDocument();
      });
    });

    it('should display community average win rate', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Community Avg')).toBeInTheDocument();
        // Use getAllByText since archetype comparison may also show 52%
        const elements = screen.getAllByText('52%');
        expect(elements.length).toBeGreaterThanOrEqual(1);
      });
    });

    it('should display win rate difference', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Difference')).toBeInTheDocument();
        expect(screen.getByText('+6%')).toBeInTheDocument();
      });
    });

    it('should display sample size', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Matches')).toBeInTheDocument();
        expect(screen.getByText('30')).toBeInTheDocument();
      });
    });

    it('should display rank label', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Above Average')).toBeInTheDocument();
      });
    });

    it('should display percentile rank', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('68th percentile')).toBeInTheDocument();
      });
    });
  });

  describe('Rank Display', () => {
    it('should display Top 5% rank with elite styling', async () => {
      const comparison = createMockComparison({ rank: 'Top 5%', percentileRank: 96 });
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Top 5%')).toBeInTheDocument();
      });
    });

    it('should display Needs Improvement rank for low percentile', async () => {
      const comparison = createMockComparison({
        rank: 'Needs Improvement',
        percentileRank: 15,
        userWinRate: 0.45,
        winRateDelta: -0.07,
      });
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Needs Improvement')).toBeInTheDocument();
      });
    });
  });

  describe('Archetype Comparison', () => {
    it('should display archetype comparison section', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Archetype Performance vs Community')).toBeInTheDocument();
      });
    });

    it('should display archetype entries with color indicators', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('Selesnya')).toBeInTheDocument();
        expect(screen.getByText('Dimir')).toBeInTheDocument();
      });
    });

    it('should display archetype win rates and deltas', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        // Selesnya: 65% vs 54% (+11%)
        expect(screen.getByText('65%')).toBeInTheDocument();
        expect(screen.getByText('(+11%)')).toBeInTheDocument();
        // Dimir: 48% vs 52% (-4%)
        expect(screen.getByText('48%')).toBeInTheDocument();
        expect(screen.getByText('(-4%)')).toBeInTheDocument();
      });
    });

    it('should display match counts for archetypes', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(screen.getByText('12 matches')).toBeInTheDocument();
        expect(screen.getByText('8 matches')).toBeInTheDocument();
      });
    });
  });

  describe('Set Code Handling', () => {
    it('should pass set code to API request', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="FDN" />);

      await waitFor(() => {
        expect(mockDrafts.getCommunityComparison).toHaveBeenCalledWith(
          expect.objectContaining({ set_code: 'FDN' })
        );
      });
    });

    it('should display set code in header', async () => {
      const comparison = createMockComparison({ setCode: 'FDN' });
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="FDN" />);

      await waitFor(() => {
        expect(screen.getByText('FDN')).toBeInTheDocument();
      });
    });
  });

  describe('Draft Format Handling', () => {
    it('should pass draft format to API request', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" draftFormat="QuickDraft" />);

      await waitFor(() => {
        expect(mockDrafts.getCommunityComparison).toHaveBeenCalledWith(
          expect.objectContaining({ draft_format: 'QuickDraft' })
        );
      });
    });

    it('should use PremierDraft as default format', async () => {
      const comparison = createMockComparison();
      mockDrafts.getCommunityComparison.mockResolvedValue(comparison);

      render(<CommunityComparison setCode="DSK" />);

      await waitFor(() => {
        expect(mockDrafts.getCommunityComparison).toHaveBeenCalledWith(
          expect.objectContaining({ draft_format: 'PremierDraft' })
        );
      });
    });
  });
});
