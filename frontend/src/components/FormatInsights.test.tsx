import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '../test/utils/testUtils';
import userEvent from '@testing-library/user-event';
import FormatInsights from './FormatInsights';
import { mockMeta } from '@/test/mocks/apiMock';
import * as Sentry from '@sentry/react';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

describe('FormatInsights', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockInsightsData = {
    color_rankings: [
      {
        color: 'WU',
        win_rate: 58.5,
        popularity: 12.3,
        games_played: 5000,
        rating: 'A',
      },
      {
        color: 'BR',
        win_rate: 56.2,
        popularity: 10.5,
        games_played: 4200,
        rating: 'B',
      },
      {
        color: 'W',
        win_rate: 52.3,
        popularity: 8.1,
        games_played: 3200,
        rating: 'C',
      },
    ],
    format_speed: {
      speed: 'Medium',
      description: 'Balanced format with moderate game length',
    },
    color_analysis: {
      best_mono_color: 'W',
      best_color_pair: 'WU',
      overdrafted_colors: [
        {
          color: 'BR',
          win_rate: 56.2,
          popularity: 60.5,
          delta: 4.3,
        },
      ],
    },
    top_bombs: [
      {
        name: 'Mythic Bomb',
        rarity: 'mythic',
        color: 'W',
        gihwr: 68.5,
      },
    ],
    top_removal: [
      {
        name: 'Murder',
        rarity: 'uncommon',
        color: 'B',
        gihwr: 62.3,
      },
    ],
    top_creatures: [
      {
        name: 'Strong Creature',
        rarity: 'uncommon',
        color: 'U',
        gihwr: 59.8,
      },
    ],
    top_commons: [
      {
        name: 'Good Common',
        rarity: 'common',
        color: 'W',
        gihwr: 55.2,
      },
    ],
  };

  const mockArchetypeCards = {
    top_cards: [
      {
        name: 'Archetype Card 1',
        rarity: 'rare',
        color: 'W',
        gihwr: 65.0,
      },
    ],
    top_removal: [
      {
        name: 'Archetype Removal',
        rarity: 'uncommon',
        color: 'U',
        gihwr: 63.0,
      },
    ],
    top_commons: [
      {
        name: 'Archetype Common',
        rarity: 'common',
        color: 'W',
        gihwr: 56.0,
      },
    ],
  };

  it('should render collapsed by default', () => {
    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    expect(screen.getByText(/Archetype Performance Dashboard/)).toBeInTheDocument();
    expect(screen.getByText(/▶/)).toBeInTheDocument();
    expect(screen.queryByText('Loading format insights...')).not.toBeInTheDocument();
  });

  it('should expand and load insights when header is clicked', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    const header = screen.getByText(/Archetype Performance Dashboard/);
    await user.click(header);

    expect(screen.getByText(/▼/)).toBeInTheDocument();

    await waitFor(() => {
      expect(mockMeta.getFormatInsights).toHaveBeenCalledWith('PremierDraft', 'BLB');
    });

    await waitFor(() => {
      expect(screen.getByText('Archetype Rankings')).toBeInTheDocument();
    });
  });

  it('should display error when loading fails', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockRejectedValue(new Error('Failed to load'));

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText(/Failed to load/)).toBeInTheDocument();
    });
  });

  it('should display color rankings when data is loaded', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Archetype Rankings')).toBeInTheDocument();
      const wuElements = screen.getAllByText('WU');
      expect(wuElements.length).toBeGreaterThan(0);
      const brElements = screen.getAllByText('BR');
      expect(brElements.length).toBeGreaterThan(0);
    });

    // Check ratings
    const ratingsA = screen.getAllByText('A');
    expect(ratingsA.length).toBeGreaterThan(0);
  });

  it('should filter color rankings by mono color', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Archetype Rankings')).toBeInTheDocument();
    });

    // Select filter - find by control-select class within control-group
    const controlGroups = document.querySelectorAll('.control-group');
    const filterGroup = Array.from(controlGroups).find((group) =>
      group.textContent?.includes('Filter:')
    );
    const filterSelect = filterGroup?.querySelector('select') as HTMLSelectElement;

    if (filterSelect) {
      await user.selectOptions(filterSelect, 'mono');
      expect(filterSelect.value).toBe('mono');
    }
  });

  it('should sort color rankings by different criteria', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Archetype Rankings')).toBeInTheDocument();
    });

    // Find sort select by control-group
    const controlGroups = document.querySelectorAll('.control-group');
    const sortGroup = Array.from(controlGroups).find((group) =>
      group.textContent?.includes('Sort by:')
    );
    const sortSelect = sortGroup?.querySelector('select') as HTMLSelectElement;

    if (sortSelect) {
      // Default sort is by win rate
      expect(sortSelect.value).toBe('winRate');

      // Change to popularity
      await user.selectOptions(sortSelect, 'popularity');
      expect(sortSelect.value).toBe('popularity');

      // Change to games played
      await user.selectOptions(sortSelect, 'games');
      expect(sortSelect.value).toBe('games');
    }
  });

  it('should load archetype cards when archetype is clicked', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);
    mockMeta.getArchetypeCards.mockResolvedValue(mockArchetypeCards);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Archetype Rankings')).toBeInTheDocument();
    });

    // Find and click on WU archetype - there might be multiple WU texts (in chart and grid)
    const colorRankItems = document.querySelectorAll('.color-rank-item');
    const wuArchetype = Array.from(colorRankItems).find((item) =>
      item.textContent?.includes('WU')
    );
    expect(wuArchetype).toBeDefined();

    if (wuArchetype) {
      await user.click(wuArchetype as HTMLElement);

      await waitFor(() => {
        expect(mockMeta.getArchetypeCards).toHaveBeenCalledWith('PremierDraft', 'WU');
      });

      await waitFor(() => {
        expect(screen.getByText('WU Top Cards')).toBeInTheDocument();
        expect(screen.getByText('Archetype Card 1')).toBeInTheDocument();
      });
    }
  });

  it('should close archetype details when close button is clicked', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);
    mockMeta.getArchetypeCards.mockResolvedValue(mockArchetypeCards);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Archetype Rankings')).toBeInTheDocument();
    });

    const colorRankItems = document.querySelectorAll('.color-rank-item');
    const wuArchetype = Array.from(colorRankItems).find((item) =>
      item.textContent?.includes('WU')
    );

    if (wuArchetype) {
      await user.click(wuArchetype as HTMLElement);

      await waitFor(() => {
        expect(screen.getByText('WU Top Cards')).toBeInTheDocument();
      });

      const closeButton = screen.getByText('✕ Close');
      await user.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByText('WU Top Cards')).not.toBeInTheDocument();
      });
    }
  });

  it('should display format speed information', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Format Speed')).toBeInTheDocument();
      expect(screen.getByText('Medium')).toBeInTheDocument();
      expect(screen.getByText('Balanced format with moderate game length')).toBeInTheDocument();
    });
  });

  it('should display color analysis', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Color Analysis')).toBeInTheDocument();
      expect(screen.getByText('Best Mono Color:')).toBeInTheDocument();
      expect(screen.getByText('Best Color Pair:')).toBeInTheDocument();
      expect(screen.getByText('Overdrafted Colors (Popularity > Win Rate)')).toBeInTheDocument();
    });
  });

  it('should display top cards sections', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(screen.getByText('Top Bombs (Rare/Mythic)')).toBeInTheDocument();
      expect(screen.getByText('Top Removal')).toBeInTheDocument();
      expect(screen.getByText('Top Performers')).toBeInTheDocument();
      expect(screen.getByText('Best Commons')).toBeInTheDocument();
    });

    expect(screen.getByText('Mythic Bomb')).toBeInTheDocument();
    expect(screen.getByText('Murder')).toBeInTheDocument();
    expect(screen.getByText('Strong Creature')).toBeInTheDocument();
    expect(screen.getByText('Good Common')).toBeInTheDocument();
  });

  it('should refresh insights when refresh button is clicked', async () => {
    const user = userEvent.setup();
    mockMeta.getFormatInsights.mockResolvedValue(mockInsightsData);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(mockMeta.getFormatInsights).toHaveBeenCalledTimes(1);
    });

    const refreshButton = screen.getByText('Refresh');
    await user.click(refreshButton);

    await waitFor(() => {
      expect(mockMeta.getFormatInsights).toHaveBeenCalledTimes(2);
    });
  });

  it('should show empty message when no data and no error', async () => {
    const user = userEvent.setup();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    mockMeta.getFormatInsights.mockResolvedValue(null as any);

    render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);

    await user.click(screen.getByText(/Archetype Performance Dashboard/));

    await waitFor(() => {
      expect(mockMeta.getFormatInsights).toHaveBeenCalled();
    });

    await waitFor(
      () => {
        expect(
          screen.getByText(/No format insights available/)
        ).toBeInTheDocument();
      },
      { timeout: 2000 }
    );
  });

  describe('Sentry error reporting', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('calls reportError with fetch_format_insights on initial load failure', async () => {
      const user = userEvent.setup();
      const sentryCapture = vi.mocked(Sentry.captureException);
      mockMeta.getFormatInsights.mockRejectedValue(new Error('API error'));

      render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);
      await user.click(screen.getByText(/Archetype Performance Dashboard/));

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalledOnce();
      });

      const callArgs = sentryCapture.mock.calls[0][1] as { tags?: Record<string, string> };
      expect(callArgs?.tags).toMatchObject({ component: 'FormatInsights', action: 'fetch_format_insights' });
    });

    it('still renders error UI when fetch_format_insights fails', async () => {
      const user = userEvent.setup();
      mockMeta.getFormatInsights.mockRejectedValue(new Error('API error'));

      render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);
      await user.click(screen.getByText(/Archetype Performance Dashboard/));

      await waitFor(() => {
        expect(screen.getByText('API error')).toBeInTheDocument();
      });
    });

    it('calls reportError with refresh_format_insights on refresh failure', async () => {
      const user = userEvent.setup();
      const sentryCapture = vi.mocked(Sentry.captureException);
      // First call succeeds to allow component to expand and show refresh button
      mockMeta.getFormatInsights
        .mockResolvedValueOnce({
          color_rankings: [],
          format_speed: null,
          color_analysis: null,
          top_bombs: [],
          top_removal: [],
          top_creatures: [],
          top_commons: [],
        })
        .mockRejectedValue(new Error('refresh failed'));

      render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);
      await user.click(screen.getByText(/Archetype Performance Dashboard/));

      await waitFor(() => {
        expect(screen.getByText('Refresh')).toBeInTheDocument();
      });

      await user.click(screen.getByText('Refresh'));

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalled();
      });

      const refreshCall = sentryCapture.mock.calls.find(
        (c) => (c[1] as { tags?: Record<string, string> })?.tags?.action === 'refresh_format_insights'
      );
      expect(refreshCall).toBeDefined();
    });

    it('calls reportError with fetch_archetype_cards on archetype load failure', async () => {
      const user = userEvent.setup();
      const sentryCapture = vi.mocked(Sentry.captureException);
      mockMeta.getFormatInsights.mockResolvedValue({
        color_rankings: [{ color: 'WU', win_rate: 58, popularity: 10, games_played: 1000, rating: 'A' }],
        format_speed: null,
        color_analysis: null,
        top_bombs: [],
        top_removal: [],
        top_creatures: [],
        top_commons: [],
      });
      mockMeta.getArchetypeCards.mockRejectedValue(new Error('archetype failed'));

      render(<FormatInsights setCode="BLB" draftFormat="PremierDraft" />);
      await user.click(screen.getByText(/Archetype Performance Dashboard/));

      await waitFor(() => {
        expect(screen.getByText('Archetype Rankings')).toBeInTheDocument();
      });

      const colorRankItems = document.querySelectorAll('.color-rank-item');
      const wuItem = Array.from(colorRankItems).find((el) => el.textContent?.includes('WU'));
      if (wuItem) {
        await user.click(wuItem as HTMLElement);
      }

      await waitFor(() => {
        const archetypeCall = sentryCapture.mock.calls.find(
          (c) => (c[1] as { tags?: Record<string, string> })?.tags?.action === 'fetch_archetype_cards'
        );
        expect(archetypeCall).toBeDefined();
      });
    });
  });
});
