import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as opponents from '../opponents';

// Mock apiClient — Phase 2 PR #6 routes opponents.* through the BFF.
vi.mock('../../apiClient', () => ({
  get: vi.fn(),
}));

import { get } from '../../apiClient';

describe('opponents API (daemon routes)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getOpponentAnalysis', () => {
    it('calls get with correct match path', async () => {
      const mockAnalysis = { profile: null, observedCards: [], expectedCards: [], strategicInsights: [], matchupStats: null, metaArchetype: null };
      vi.mocked(get).mockResolvedValue(mockAnalysis);

      const result = await opponents.getOpponentAnalysis('match-abc');

      expect(get).toHaveBeenCalledWith('/matches/match-abc/opponent-analysis');
      expect(result).toEqual(mockAnalysis);
    });
  });

  describe('listOpponentDecks', () => {
    it('calls get with no query params when none provided', async () => {
      vi.mocked(get).mockResolvedValue({ profiles: [], total: 0 });

      await opponents.listOpponentDecks();

      expect(get).toHaveBeenCalledWith('/opponents/decks');
    });

    it('calls get with archetype and format params', async () => {
      vi.mocked(get).mockResolvedValue({ profiles: [], total: 0 });

      await opponents.listOpponentDecks({ archetype: 'Aggro', format: 'Standard' });

      expect(get).toHaveBeenCalledWith('/opponents/decks?archetype=Aggro&format=Standard');
    });

    it('calls get with minConfidence and limit params', async () => {
      vi.mocked(get).mockResolvedValue({ profiles: [], total: 0 });

      await opponents.listOpponentDecks({ minConfidence: 0.7, limit: 10 });

      expect(get).toHaveBeenCalledWith('/opponents/decks?min_confidence=0.7&limit=10');
    });
  });

  describe('getMatchupStats', () => {
    it('calls get without format param when not provided', async () => {
      vi.mocked(get).mockResolvedValue({ matchups: [], total: 0 });

      await opponents.getMatchupStats();

      expect(get).toHaveBeenCalledWith('/analytics/matchups');
    });

    it('calls get with encoded format param', async () => {
      vi.mocked(get).mockResolvedValue({ matchups: [], total: 0 });

      await opponents.getMatchupStats('Standard');

      expect(get).toHaveBeenCalledWith('/analytics/matchups?format=Standard');
    });
  });

  describe('getOpponentHistory', () => {
    it('calls get without format when not provided', async () => {
      vi.mocked(get).mockResolvedValue({ totalOpponents: 0, uniqueArchetypes: 0, mostCommonArchetype: '', mostCommonCount: 0, archetypeBreakdown: [], colorIdentityStats: [] });

      await opponents.getOpponentHistory();

      expect(get).toHaveBeenCalledWith('/analytics/opponent-history');
    });

    it('calls get with format param', async () => {
      vi.mocked(get).mockResolvedValue({ totalOpponents: 5, uniqueArchetypes: 3, mostCommonArchetype: 'Aggro', mostCommonCount: 2, archetypeBreakdown: [], colorIdentityStats: [] });

      await opponents.getOpponentHistory('Standard');

      expect(get).toHaveBeenCalledWith('/analytics/opponent-history?format=Standard');
    });
  });

  describe('getExpectedCards', () => {
    it('calls get with archetype path and no format', async () => {
      vi.mocked(get).mockResolvedValue({ archetype: 'Aggro', format: '', expectedCards: [], total: 0 });

      await opponents.getExpectedCards('Aggro');

      expect(get).toHaveBeenCalledWith('/archetypes/Aggro/expected-cards');
    });

    it('calls get with archetype and format', async () => {
      vi.mocked(get).mockResolvedValue({ archetype: 'Control', format: 'Standard', expectedCards: [], total: 0 });

      await opponents.getExpectedCards('Control', 'Standard');

      expect(get).toHaveBeenCalledWith('/archetypes/Control/expected-cards?format=Standard');
    });
  });
});

describe('opponents utility functions', () => {
  describe('parseCardIds', () => {
    it('parses valid JSON array', () => {
      expect(opponents.parseCardIds('[1,2,3]')).toEqual([1, 2, 3]);
    });
    it('returns empty array for null', () => {
      expect(opponents.parseCardIds(null)).toEqual([]);
    });
    it('returns empty array for invalid JSON', () => {
      expect(opponents.parseCardIds('not-json')).toEqual([]);
    });
  });

  describe('getDeckStyleDisplayName', () => {
    it('maps known styles', () => {
      expect(opponents.getDeckStyleDisplayName('aggro')).toBe('Aggro');
      expect(opponents.getDeckStyleDisplayName('control')).toBe('Control');
      expect(opponents.getDeckStyleDisplayName('midrange')).toBe('Midrange');
      expect(opponents.getDeckStyleDisplayName('combo')).toBe('Combo');
      expect(opponents.getDeckStyleDisplayName('tempo')).toBe('Tempo');
    });
    it('returns Unknown for null', () => {
      expect(opponents.getDeckStyleDisplayName(null)).toBe('Unknown');
    });
    it('returns original for unknown style', () => {
      expect(opponents.getDeckStyleDisplayName('jank')).toBe('jank');
    });
  });

  describe('getPriorityColorClass', () => {
    it('returns correct class for each priority', () => {
      expect(opponents.getPriorityColorClass('high')).toBe('text-red-400');
      expect(opponents.getPriorityColorClass('medium')).toBe('text-yellow-400');
      expect(opponents.getPriorityColorClass('low')).toBe('text-blue-400');
    });
    it('returns gray for unknown priority', () => {
      expect(opponents.getPriorityColorClass('unknown' as 'high')).toBe('text-gray-400');
    });
  });

  describe('formatConfidence', () => {
    it('formats fraction as percentage', () => {
      expect(opponents.formatConfidence(0.85)).toBe('85%');
      expect(opponents.formatConfidence(1.0)).toBe('100%');
      expect(opponents.formatConfidence(0)).toBe('0%');
    });
  });

  describe('getConfidenceColorClass', () => {
    it('returns green for >= 0.7', () => {
      expect(opponents.getConfidenceColorClass(0.7)).toBe('text-green-400');
    });
    it('returns yellow for >= 0.5 and < 0.7', () => {
      expect(opponents.getConfidenceColorClass(0.5)).toBe('text-yellow-400');
    });
    it('returns gray for < 0.5', () => {
      expect(opponents.getConfidenceColorClass(0.3)).toBe('text-gray-400');
    });
  });
});
