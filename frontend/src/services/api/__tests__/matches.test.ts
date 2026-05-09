import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as matches from '../matches';

// Mock the daemonClient (matches routes go to the local daemon)
vi.mock('../../daemonClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
}));

import { get, post } from '../../daemonClient';

describe('matches API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('statsFilterToRequest', () => {
    it('should convert StatsFilter to StatsFilterRequest', () => {
      const filter = {
        AccountID: 123,
        StartDate: new Date('2024-01-01'),
        EndDate: new Date('2024-01-31'),
        Format: 'standard',
        Formats: ['standard', 'historic'],
        DeckFormat: 'standard',
        DeckID: 'deck-123',
        EventName: 'Ranked',
        EventNames: ['Ranked', 'Premier'],
        OpponentName: 'opponent',
        OpponentID: 'opp-123',
        Result: 'win',
        RankClass: 'Gold',
        RankMinClass: 'Silver',
        RankMaxClass: 'Platinum',
        ResultReason: 'concede',
      } as unknown as matches.StatsFilter;

      const result = matches.statsFilterToRequest(filter);

      expect(result).toEqual({
        accountID: 123,
        startDate: '2024-01-01',
        endDate: '2024-01-31',
        format: 'standard',
        formats: ['standard', 'historic'],
        deckFormat: 'standard',
        deckID: 'deck-123',
        eventName: 'Ranked',
        eventNames: ['Ranked', 'Premier'],
        opponentName: 'opponent',
        opponentID: 'opp-123',
        result: 'win',
        rankClass: 'Gold',
        rankMinClass: 'Silver',
        rankMaxClass: 'Platinum',
        resultReason: 'concede',
      });
    });

    it('should handle empty filter', () => {
      const filter = {} as unknown as matches.StatsFilter;
      const result = matches.statsFilterToRequest(filter);

      expect(result).toEqual({
        accountID: undefined,
        startDate: undefined,
        endDate: undefined,
        format: undefined,
        formats: undefined,
        deckFormat: undefined,
        deckID: undefined,
        eventName: undefined,
        eventNames: undefined,
        opponentName: undefined,
        opponentID: undefined,
        result: undefined,
        rankClass: undefined,
        rankMinClass: undefined,
        rankMaxClass: undefined,
        resultReason: undefined,
      });
    });

    it('should handle ISO string dates', () => {
      const filter = {
        StartDate: '2024-01-01T00:00:00.000Z' as unknown as Date,
        EndDate: '2024-01-31T23:59:59.999Z' as unknown as Date,
      } as unknown as matches.StatsFilter;

      const result = matches.statsFilterToRequest(filter);

      expect(result.startDate).toBe('2024-01-01');
      expect(result.endDate).toBe('2024-01-31');
    });
  });

  describe('getMatches', () => {
    it('should call post with correct path and filter', async () => {
      const mockMatches = [{ id: '1', result: 'win' }];
      vi.mocked(post).mockResolvedValue(mockMatches);

      const result = await matches.getMatches({ format: 'standard' });

      expect(post).toHaveBeenCalledWith('/matches', { format: 'standard' });
      expect(result).toEqual(mockMatches);
    });
  });

  describe('getMatch', () => {
    it('should call get with correct path', async () => {
      const mockMatch = { id: 'match-123', result: 'win' };
      vi.mocked(get).mockResolvedValue(mockMatch);

      const result = await matches.getMatch('match-123');

      expect(get).toHaveBeenCalledWith('/matches/match-123');
      expect(result).toEqual(mockMatch);
    });
  });

  describe('getMatchGames', () => {
    it('should call get with correct path', async () => {
      const mockGames = [{ id: 'game-1' }];
      vi.mocked(get).mockResolvedValue(mockGames);

      const result = await matches.getMatchGames('match-123');

      expect(get).toHaveBeenCalledWith('/matches/match-123/games');
      expect(result).toEqual(mockGames);
    });
  });

  describe('getStats', () => {
    it('should call post with correct path and filter', async () => {
      const mockStats = { wins: 10, losses: 5 };
      vi.mocked(post).mockResolvedValue(mockStats);

      const result = await matches.getStats({ format: 'historic' });

      expect(post).toHaveBeenCalledWith('/matches/stats', { format: 'historic' });
      expect(result).toEqual(mockStats);
    });
  });

  describe('getFormats', () => {
    it('should call get with correct path', async () => {
      const mockFormats = ['standard', 'historic'];
      vi.mocked(get).mockResolvedValue(mockFormats);

      const result = await matches.getFormats();

      expect(get).toHaveBeenCalledWith('/matches/formats');
      expect(result).toEqual(mockFormats);
    });
  });

  describe('getPerformanceMetrics', () => {
    it('should call post with correct path and filter', async () => {
      const mockMetrics = { winRate: 0.6 };
      vi.mocked(post).mockResolvedValue(mockMetrics);

      const result = await matches.getPerformanceMetrics({ format: 'standard' });

      expect(post).toHaveBeenCalledWith('/matches/performance', { format: 'standard' });
      expect(result).toEqual(mockMetrics);
    });
  });

  describe('getRankProgression', () => {
    it('should call get with correct path', async () => {
      const mockProgression = { currentRank: 'Gold' };
      vi.mocked(get).mockResolvedValue(mockProgression);

      const result = await matches.getRankProgression('standard');

      expect(get).toHaveBeenCalledWith('/matches/rank-progression/standard');
      expect(result).toEqual(mockProgression);
    });

    it('should encode format with special characters', async () => {
      vi.mocked(get).mockResolvedValue({});

      await matches.getRankProgression('format/with/slashes');

      expect(get).toHaveBeenCalledWith('/matches/rank-progression/format%2Fwith%2Fslashes');
    });
  });

  describe('exportMatches', () => {
    it('should call get with correct format parameter', async () => {
      vi.mocked(get).mockResolvedValue({ data: 'exported' });

      await matches.exportMatches('json');

      expect(get).toHaveBeenCalledWith('/matches/export?format=json');
    });

    it('should handle csv format', async () => {
      vi.mocked(get).mockResolvedValue('csv,data');

      await matches.exportMatches('csv');

      expect(get).toHaveBeenCalledWith('/matches/export?format=csv');
    });
  });
});
