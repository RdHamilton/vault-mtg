import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import GamePlayTimelinePanel from './GamePlayTimelinePanel';
import type { PlayTimelineEntry, GamePlay, GameStateSnapshot } from '@/services/api/gameplays';
import * as Sentry from '@sentry/react';

// Mock the gameplays API module
vi.mock('@/services/api/gameplays', () => ({
  getMatchTimeline: vi.fn(),
}));

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

import * as gameplays from '@/services/api/gameplays';

const mockGetMatchTimeline = vi.mocked(gameplays.getMatchTimeline);

const mockSnapshot: GameStateSnapshot = {
  id: 1,
  game_id: 1,
  match_id: 'match-123',
  turn_number: 1,
  active_player: 'player',
  player_life: 20,
  opponent_life: 18,
  player_cards_in_hand: 5,
  opponent_cards_in_hand: 6,
  player_lands_in_play: 2,
  opponent_lands_in_play: 1,
  timestamp: '2025-01-09T12:00:00Z',
};

const mockPlayerPlay: GamePlay = {
  id: 1,
  game_id: 1,
  match_id: 'match-123',
  turn_number: 1,
  phase: 'Main1',
  player_type: 'player',
  action_type: 'play_card',
  card_id: 123,
  card_name: 'Lightning Bolt',
  zone_from: 'hand',
  zone_to: 'stack',
  timestamp: '2025-01-09T12:00:01Z',
  sequence_number: 1,
};

const mockOpponentPlay: GamePlay = {
  id: 2,
  game_id: 1,
  match_id: 'match-123',
  turn_number: 1,
  phase: 'Main1',
  player_type: 'opponent',
  action_type: 'land_drop',
  card_id: 456,
  card_name: 'Forest',
  zone_from: 'hand',
  zone_to: 'battlefield',
  timestamp: '2025-01-09T12:00:02Z',
  sequence_number: 2,
};

const mockTimeline: PlayTimelineEntry[] = [
  {
    turn: 1,
    active_player: 'player',
    player_plays: [mockPlayerPlay],
    opponent_plays: [mockOpponentPlay],
    snapshot: mockSnapshot,
  },
  {
    turn: 2,
    active_player: 'opponent',
    player_plays: [],
    opponent_plays: [
      {
        ...mockOpponentPlay,
        id: 3,
        turn_number: 2,
        action_type: 'attack',
        card_name: 'Grizzly Bears',
        sequence_number: 3,
      },
    ],
    snapshot: {
      ...mockSnapshot,
      id: 2,
      turn_number: 2,
      active_player: 'opponent',
      player_life: 18,
      opponent_life: 18,
    },
  },
];

describe('GamePlayTimelinePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetMatchTimeline.mockResolvedValue(mockTimeline);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('collapsed state', () => {
    it('renders header when collapsed', () => {
      render(
        <GamePlayTimelinePanel matchId="match-123" isExpanded={false} onToggle={() => {}} />
      );

      expect(screen.getByRole('button', { name: /Game Timeline/i })).toBeInTheDocument();
    });

    it('does not load data when collapsed', () => {
      render(
        <GamePlayTimelinePanel matchId="match-123" isExpanded={false} onToggle={() => {}} />
      );

      expect(mockGetMatchTimeline).not.toHaveBeenCalled();
    });

    it('calls onToggle when header is clicked', () => {
      const onToggle = vi.fn();
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={false} onToggle={onToggle} />);

      fireEvent.click(screen.getByRole('button', { name: /Game Timeline/i }));

      expect(onToggle).toHaveBeenCalled();
    });
  });

  describe('expanded state', () => {
    it('loads timeline data when expanded', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(mockGetMatchTimeline).toHaveBeenCalledWith('match-123');
      });
    });

    it('shows loading state while fetching', async () => {
      // Delay the resolution to see loading state
      mockGetMatchTimeline.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve(mockTimeline), 100))
      );

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      expect(screen.getByText(/Loading timeline/i)).toBeInTheDocument();
    });

    it('displays turn navigation buttons', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Turn 1' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Turn 2' })).toBeInTheDocument();
      });
    });

    it('auto-selects first turn on load', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Turn 1')).toBeInTheDocument();
      });
    });

    it('shows turn details including snapshot', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        // Check life totals
        expect(screen.getByText('You: 20')).toBeInTheDocument();
        expect(screen.getByText('Opp: 18')).toBeInTheDocument();
      });
    });

    it('shows player plays', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Lightning Bolt')).toBeInTheDocument();
      });
    });

    it('shows opponent plays', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Forest')).toBeInTheDocument();
      });
    });

    it('changes turn when button is clicked', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Turn 2' })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Turn 2' }));

      await waitFor(() => {
        expect(screen.getByText('Turn 2')).toBeInTheDocument();
        expect(screen.getByText('Grizzly Bears')).toBeInTheDocument();
      });
    });

    it('shows active player for each turn', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Active: You')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Turn 2' }));

      await waitFor(() => {
        expect(screen.getByText('Active: Opponent')).toBeInTheDocument();
      });
    });
  });

  describe('empty state', () => {
    it('shows empty state when no timeline data', async () => {
      mockGetMatchTimeline.mockResolvedValue([]);

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText(/No game play data available/i)).toBeInTheDocument();
      });
    });
  });

  describe('error state', () => {
    it('shows error message on fetch failure', async () => {
      mockGetMatchTimeline.mockRejectedValue(new Error('Network error'));

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Network error')).toBeInTheDocument();
      });
    });
  });

  describe('action type formatting', () => {
    it('formats play_card as Played', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Played')).toBeInTheDocument();
      });
    });

    it('formats land_drop as Land', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Land')).toBeInTheDocument();
      });
    });

    it('formats attack action', async () => {
      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      // Navigate to turn 2 which has attack action
      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Turn 2' })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Turn 2' }));

      await waitFor(() => {
        expect(screen.getByText('Attack')).toBeInTheDocument();
      });
    });

    it('formats life_change with damage amount', async () => {
      const lifeChangePlay: GamePlay = {
        id: 10,
        game_id: 1,
        match_id: 'match-123',
        turn_number: 3,
        phase: 'Combat',
        player_type: 'opponent',
        action_type: 'life_change',
        timestamp: '2025-01-09T12:00:05Z',
        sequence_number: 10,
        life_from: 18,
        life_to: 15,
      };

      const timelineWithLifeChange: PlayTimelineEntry[] = [
        {
          turn: 3,
          active_player: 'player',
          player_plays: [],
          opponent_plays: [lifeChangePlay],
          snapshot: mockSnapshot,
        },
      ];

      mockGetMatchTimeline.mockResolvedValue(timelineWithLifeChange);

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Life Change')).toBeInTheDocument();
        expect(screen.getByText(/18 → 15/)).toBeInTheDocument();
        expect(screen.getByText(/\(-3\)/)).toBeInTheDocument();
      });
    });

    it('formats life_change with heal amount', async () => {
      const lifeHealPlay: GamePlay = {
        id: 11,
        game_id: 1,
        match_id: 'match-123',
        turn_number: 4,
        phase: 'Main1',
        player_type: 'player',
        action_type: 'life_change',
        timestamp: '2025-01-09T12:00:06Z',
        sequence_number: 11,
        life_from: 15,
        life_to: 18,
      };

      const timelineWithHeal: PlayTimelineEntry[] = [
        {
          turn: 4,
          active_player: 'player',
          player_plays: [lifeHealPlay],
          opponent_plays: [],
          snapshot: mockSnapshot,
        },
      ];

      mockGetMatchTimeline.mockResolvedValue(timelineWithHeal);

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Life Change')).toBeInTheDocument();
        expect(screen.getByText(/15 → 18/)).toBeInTheDocument();
        expect(screen.getByText(/\(\+3\)/)).toBeInTheDocument();
      });
    });

    it('formats cast_spell action', async () => {
      const castSpellPlay: GamePlay = {
        id: 12,
        game_id: 1,
        match_id: 'match-123',
        turn_number: 5,
        phase: 'Main1',
        player_type: 'player',
        action_type: 'cast_spell',
        card_id: 789,
        card_name: 'Counterspell',
        zone_from: 'hand',
        zone_to: 'stack',
        timestamp: '2025-01-09T12:00:07Z',
        sequence_number: 12,
      };

      const timelineWithCast: PlayTimelineEntry[] = [
        {
          turn: 5,
          active_player: 'player',
          player_plays: [castSpellPlay],
          opponent_plays: [],
          snapshot: mockSnapshot,
        },
      ];

      mockGetMatchTimeline.mockResolvedValue(timelineWithCast);

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('Cast')).toBeInTheDocument();
        expect(screen.getByText('Counterspell')).toBeInTheDocument();
      });
    });
  });

  describe('unmount behavior', () => {
    it('does not update state after unmount', async () => {
      let resolveTimeline: (value: PlayTimelineEntry[]) => void;
      const slowTimelinePromise = new Promise<PlayTimelineEntry[]>((resolve) => {
        resolveTimeline = resolve;
      });
      mockGetMatchTimeline.mockReturnValue(slowTimelinePromise);

      const { unmount } = render(
        <GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />
      );

      // Unmount before API resolves
      unmount();

      // Resolve the API - should not cause state update
      resolveTimeline!(mockTimeline);

      // No warnings or errors should occur
    });
  });

  describe('Sentry error reporting', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('calls reportError with load_game_timeline when getMatchTimeline throws', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      mockGetMatchTimeline.mockRejectedValue(new Error('timeline error'));

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalledOnce();
      });

      const callArgs = sentryCapture.mock.calls[0][1] as { tags?: Record<string, string> };
      expect(callArgs?.tags).toMatchObject({ component: 'GamePlayTimelinePanel', action: 'load_game_timeline' });
    });

    it('still renders error UI when timeline load fails', async () => {
      mockGetMatchTimeline.mockRejectedValue(new Error('timeline error'));

      render(<GamePlayTimelinePanel matchId="match-123" isExpanded={true} onToggle={() => {}} />);

      await waitFor(() => {
        expect(screen.getByText('timeline error')).toBeInTheDocument();
      });
    });
  });
});
