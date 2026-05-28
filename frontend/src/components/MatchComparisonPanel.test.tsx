import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import MatchComparisonPanel from './MatchComparisonPanel';
import type { ComparisonResult, StatsFilter } from '@/services/api/matches';
import * as matchesApi from '@/services/api/matches';
import * as Sentry from '@sentry/react';

vi.mock('@/services/api/matches', () => ({
  compareFormats: vi.fn(),
  compareDecks: vi.fn(),
  compareTimePeriods: vi.fn(),
}));

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

const mockCompareFormats = vi.mocked(matchesApi.compareFormats);
const mockCompareDecks = vi.mocked(matchesApi.compareDecks);
const mockCompareTimePeriods = vi.mocked(matchesApi.compareTimePeriods);

// Mock filter with required fields - cast as StatsFilter for test purposes
const mockFilter = { Formats: [], EventNames: [] } as unknown as StatsFilter;

const mockComparisonResult: ComparisonResult = {
  Groups: [
    {
      Label: 'Standard',
      Filter: mockFilter,
      Statistics: {
        TotalMatches: 50,
        MatchesWon: 30,
        MatchesLost: 20,
        TotalGames: 100,
        GamesWon: 60,
        GamesLost: 40,
        WinRate: 0.6,
        GameWinRate: 0.6,
      },
      MatchCount: 50,
    },
    {
      Label: 'Historic',
      Filter: mockFilter,
      Statistics: {
        TotalMatches: 30,
        MatchesWon: 15,
        MatchesLost: 15,
        TotalGames: 60,
        GamesWon: 30,
        GamesLost: 30,
        WinRate: 0.5,
        GameWinRate: 0.5,
      },
      MatchCount: 30,
    },
  ],
  BestGroup: {
    Label: 'Standard',
    Filter: mockFilter,
    Statistics: {
      TotalMatches: 50,
      MatchesWon: 30,
      MatchesLost: 20,
      TotalGames: 100,
      GamesWon: 60,
      GamesLost: 40,
      WinRate: 0.6,
      GameWinRate: 0.6,
    },
    MatchCount: 50,
  },
  WorstGroup: {
    Label: 'Historic',
    Filter: mockFilter,
    Statistics: {
      TotalMatches: 30,
      MatchesWon: 15,
      MatchesLost: 15,
      TotalGames: 60,
      GamesWon: 30,
      GamesLost: 30,
      WinRate: 0.5,
      GameWinRate: 0.5,
    },
    MatchCount: 30,
  },
  WinRateDiff: 0.1,
  TotalMatches: 80,
  ComparisonDate: '2025-01-09T12:00:00Z',
};

describe('MatchComparisonPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCompareFormats.mockResolvedValue(mockComparisonResult);
    mockCompareDecks.mockResolvedValue(mockComparisonResult);
    mockCompareTimePeriods.mockResolvedValue(mockComparisonResult);
  });

  describe('rendering', () => {
    it('renders the panel header', () => {
      render(<MatchComparisonPanel />);
      expect(screen.getByText('Match Comparison')).toBeInTheDocument();
    });

    it('renders comparison type buttons', () => {
      render(<MatchComparisonPanel />);
      expect(screen.getByText('Compare Formats')).toBeInTheDocument();
      expect(screen.getByText('Compare Decks')).toBeInTheDocument();
      expect(screen.getByText('Compare Time Periods')).toBeInTheDocument();
    });

    it('renders formats selector by default', () => {
      render(<MatchComparisonPanel formats={['Standard', 'Historic']} />);
      expect(screen.getByText('Select Formats to Compare')).toBeInTheDocument();
    });

    it('renders format options when formats provided', () => {
      render(<MatchComparisonPanel formats={['Standard', 'Historic']} />);
      expect(screen.getByText('Standard')).toBeInTheDocument();
      expect(screen.getByText('Historic')).toBeInTheDocument();
    });

    it('renders empty message when no formats', () => {
      render(<MatchComparisonPanel formats={[]} />);
      expect(screen.getByText('No formats available. Play some matches first.')).toBeInTheDocument();
    });

    it('renders close button when onClose provided', () => {
      const onClose = vi.fn();
      render(<MatchComparisonPanel onClose={onClose} />);
      expect(screen.getByText('×')).toBeInTheDocument();
    });
  });

  describe('comparison type selection', () => {
    it('switches to decks selector', () => {
      render(
        <MatchComparisonPanel
          deckIds={[
            { id: 'deck-1', name: 'Izzet Phoenix' },
            { id: 'deck-2', name: 'Mono White' },
          ]}
        />
      );
      fireEvent.click(screen.getByText('Compare Decks'));
      expect(screen.getByText('Select Decks to Compare')).toBeInTheDocument();
    });

    it('switches to time periods selector', () => {
      render(<MatchComparisonPanel />);
      fireEvent.click(screen.getByText('Compare Time Periods'));
      expect(screen.getByText('Select Time Periods to Compare')).toBeInTheDocument();
    });

    it('renders deck options when decks provided', () => {
      render(
        <MatchComparisonPanel
          deckIds={[
            { id: 'deck-1', name: 'Izzet Phoenix' },
            { id: 'deck-2', name: 'Mono White' },
          ]}
        />
      );
      fireEvent.click(screen.getByText('Compare Decks'));
      expect(screen.getByText('Izzet Phoenix')).toBeInTheDocument();
      expect(screen.getByText('Mono White')).toBeInTheDocument();
    });

    it('renders time period options', () => {
      render(<MatchComparisonPanel />);
      fireEvent.click(screen.getByText('Compare Time Periods'));
      expect(screen.getByText('Last 7 Days')).toBeInTheDocument();
      expect(screen.getByText('Last 30 Days')).toBeInTheDocument();
    });
  });

  describe('selection behavior', () => {
    it('toggles format selection', () => {
      render(<MatchComparisonPanel formats={['Standard', 'Historic']} />);
      const checkboxes = screen.getAllByRole('checkbox');
      const standardCheckbox = checkboxes[0];

      fireEvent.click(standardCheckbox);
      expect(standardCheckbox).toBeChecked();

      fireEvent.click(standardCheckbox);
      expect(standardCheckbox).not.toBeChecked();
    });
  });

  describe('comparison execution', () => {
    it('shows error when less than 2 formats selected', async () => {
      render(<MatchComparisonPanel formats={['Standard', 'Historic']} />);

      // Select only one format
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]);

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        expect(screen.getByText('Please select at least 2 formats to compare')).toBeInTheDocument();
      });
    });

    it('calls compareFormats API with selected formats', async () => {
      render(<MatchComparisonPanel formats={['Standard', 'Historic', 'Explorer']} />);

      // Select formats
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]); // Standard
      fireEvent.click(checkboxes[1]); // Historic

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        expect(mockCompareFormats).toHaveBeenCalledWith({
          formats: ['Standard', 'Historic'],
          baseFilter: {},
        });
      });
    });

    it('displays comparison results', async () => {
      render(<MatchComparisonPanel formats={['Standard', 'Historic']} />);

      // Select formats
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]); // Standard
      fireEvent.click(checkboxes[1]); // Historic

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        expect(screen.getByText('Comparison Results')).toBeInTheDocument();
        expect(screen.getByText('Total Matches: 80')).toBeInTheDocument();
        // Win rates appear twice (WinRate and GameWinRate columns)
        expect(screen.getAllByText('60.0%').length).toBeGreaterThan(0);
        expect(screen.getAllByText('50.0%').length).toBeGreaterThan(0);
      });
    });

    it('shows best and worst badges', async () => {
      render(<MatchComparisonPanel formats={['Standard', 'Historic']} />);

      // Select formats
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]); // Standard
      fireEvent.click(checkboxes[1]); // Historic

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        expect(screen.getByText('Best')).toBeInTheDocument();
        expect(screen.getByText('Worst')).toBeInTheDocument();
      });
    });

    it('handles API errors gracefully', async () => {
      mockCompareFormats.mockRejectedValue(new Error('Network error'));

      render(<MatchComparisonPanel formats={['Standard', 'Historic']} />);

      // Select formats
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]); // Standard
      fireEvent.click(checkboxes[1]); // Historic

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        expect(screen.getByText('Network error')).toBeInTheDocument();
      });
    });
  });

  describe('close behavior', () => {
    it('calls onClose when close button clicked', () => {
      const onClose = vi.fn();
      render(<MatchComparisonPanel onClose={onClose} />);

      fireEvent.click(screen.getByText('×'));

      expect(onClose).toHaveBeenCalled();
    });
  });

  describe('deck comparison display', () => {
    const deckComparisonResult: ComparisonResult = {
      Groups: [
        {
          Label: 'deck-uuid-1', // API returns UUID
          Filter: mockFilter,
          Statistics: {
            TotalMatches: 50,
            MatchesWon: 30,
            MatchesLost: 20,
            TotalGames: 100,
            GamesWon: 60,
            GamesLost: 40,
            WinRate: 0.6,
            GameWinRate: 0.6,
          },
          MatchCount: 50,
        },
        {
          Label: 'deck-uuid-2', // API returns UUID
          Filter: mockFilter,
          Statistics: {
            TotalMatches: 30,
            MatchesWon: 12,
            MatchesLost: 18,
            TotalGames: 60,
            GamesWon: 24,
            GamesLost: 36,
            WinRate: 0.4,
            GameWinRate: 0.4,
          },
          MatchCount: 30,
        },
      ],
      BestGroup: {
        Label: 'deck-uuid-1',
        Filter: mockFilter,
        Statistics: {
          TotalMatches: 50,
          MatchesWon: 30,
          MatchesLost: 20,
          TotalGames: 100,
          GamesWon: 60,
          GamesLost: 40,
          WinRate: 0.6,
          GameWinRate: 0.6,
        },
        MatchCount: 50,
      },
      WorstGroup: {
        Label: 'deck-uuid-2',
        Filter: mockFilter,
        Statistics: {
          TotalMatches: 30,
          MatchesWon: 12,
          MatchesLost: 18,
          TotalGames: 60,
          GamesWon: 24,
          GamesLost: 36,
          WinRate: 0.4,
          GameWinRate: 0.4,
        },
        MatchCount: 30,
      },
      WinRateDiff: 0.2,
      TotalMatches: 80,
      ComparisonDate: '2025-01-09T12:00:00Z',
    };

    it('displays deck names instead of UUIDs in comparison results', async () => {
      mockCompareDecks.mockResolvedValue(deckComparisonResult);

      render(
        <MatchComparisonPanel
          deckIds={[
            { id: 'deck-uuid-1', name: 'Izzet Phoenix' },
            { id: 'deck-uuid-2', name: 'Mono White Aggro' },
          ]}
        />
      );

      // Switch to decks comparison
      fireEvent.click(screen.getByText('Compare Decks'));

      // Select decks
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]); // Izzet Phoenix
      fireEvent.click(checkboxes[1]); // Mono White Aggro

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        // Should show deck names, not UUIDs
        expect(screen.getByText('Izzet Phoenix')).toBeInTheDocument();
        expect(screen.getByText('Mono White Aggro')).toBeInTheDocument();
        // Should NOT show UUIDs
        expect(screen.queryByText('deck-uuid-1')).not.toBeInTheDocument();
        expect(screen.queryByText('deck-uuid-2')).not.toBeInTheDocument();
      });
    });

    it('displays insightful comparison text for deck comparison', async () => {
      mockCompareDecks.mockResolvedValue(deckComparisonResult);

      render(
        <MatchComparisonPanel
          deckIds={[
            { id: 'deck-uuid-1', name: 'Izzet Phoenix' },
            { id: 'deck-uuid-2', name: 'Mono White Aggro' },
          ]}
        />
      );

      // Switch to decks comparison
      fireEvent.click(screen.getByText('Compare Decks'));

      // Select decks
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]);
      fireEvent.click(checkboxes[1]);

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        // Should show deck-specific insight with comparison
        expect(screen.getByText(/outperforms/)).toBeInTheDocument();
        expect(screen.getByText(/percentage points/)).toBeInTheDocument();
        // Deck names appear multiple times (in selector, table, and insight)
        // so we use getAllByText and verify they exist
        expect(screen.getAllByText(/Izzet Phoenix/).length).toBeGreaterThan(0);
        expect(screen.getAllByText(/Mono White Aggro/).length).toBeGreaterThan(0);
      });
    });

    it('renders insights as safe JSX without raw HTML tags', async () => {
      mockCompareDecks.mockResolvedValue(deckComparisonResult);

      const { container } = render(
        <MatchComparisonPanel
          deckIds={[
            { id: 'deck-uuid-1', name: 'Izzet Phoenix' },
            { id: 'deck-uuid-2', name: 'Mono White Aggro' },
          ]}
        />
      );

      // Switch to decks comparison
      fireEvent.click(screen.getByText('Compare Decks'));

      // Select decks
      const checkboxes = screen.getAllByRole('checkbox');
      fireEvent.click(checkboxes[0]);
      fireEvent.click(checkboxes[1]);

      // Click compare
      fireEvent.click(screen.getByText('Compare'));

      await waitFor(() => {
        // Verify insights section exists
        expect(screen.getByText(/outperforms/)).toBeInTheDocument();

        // Verify no raw HTML tags in the insights section
        // This tests that we're using JSX properly instead of dangerouslySetInnerHTML
        const insightsSection = container.querySelector('.comparison-insight');
        expect(insightsSection).not.toBeNull();
        if (insightsSection) {
          const htmlContent = insightsSection.innerHTML;
          // Should not contain escaped HTML entities that would indicate raw HTML strings
          expect(htmlContent).not.toContain('&lt;strong&gt;');
          expect(htmlContent).not.toContain('&lt;span&gt;');
        }

        // Verify strong and span elements are rendered properly as DOM elements
        const strongElements = container.querySelectorAll('.comparison-insight strong');
        expect(strongElements.length).toBeGreaterThan(0);

        const highlightSpans = container.querySelectorAll('.comparison-insight .highlight');
        expect(highlightSpans.length).toBeGreaterThan(0);
      });
    });
  });

  describe('Sentry error reporting', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('calls reportError with compare_matches when compareFormats throws', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      mockCompareFormats.mockRejectedValue(new Error('comparison failed'));

      render(
        <MatchComparisonPanel
          formats={['Standard', 'Historic']}
          deckIds={[]}
        />
      );

      // Select 2 formats
      fireEvent.click(screen.getByTestId('format-checkbox-Standard'));
      fireEvent.click(screen.getByTestId('format-checkbox-Historic'));
      fireEvent.click(screen.getByTestId('compare-button'));

      await waitFor(() => {
        const compCall = sentryCapture.mock.calls.find(
          (c) => (c[1] as { tags?: Record<string, string> })?.tags?.action === 'compare_matches'
        );
        expect(compCall).toBeDefined();
        const callArgs = compCall![1] as { tags?: Record<string, string> };
        expect(callArgs?.tags).toMatchObject({ component: 'MatchComparisonPanel', action: 'compare_matches' });
      });
    });
  });
});
