import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as ml from '../mlSuggestions';

// Phase 2 PR #11 — ML routes now hit the BFF via apiClient.
vi.mock('../../apiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

import { get, post, put, del } from '../../apiClient';

describe('mlSuggestions API (BFF routes)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getMLSuggestions', () => {
    it('calls get without active param when activeOnly is true (default)', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await ml.getMLSuggestions('deck-1');

      expect(get).toHaveBeenCalledWith('/decks/deck-1/ml-suggestions');
    });

    it('calls get with active=false param when activeOnly is false', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await ml.getMLSuggestions('deck-1', false);

      expect(get).toHaveBeenCalledWith('/decks/deck-1/ml-suggestions?active=false');
    });
  });

  describe('generateMLSuggestions', () => {
    it('calls post with correct path', async () => {
      vi.mocked(post).mockResolvedValue([]);

      const result = await ml.generateMLSuggestions('deck-2');

      expect(post).toHaveBeenCalledWith('/decks/deck-2/ml-suggestions/generate', {});
      expect(result).toEqual([]);
    });
  });

  describe('dismissMLSuggestion', () => {
    it('calls put with suggestion dismiss path', async () => {
      vi.mocked(put).mockResolvedValue(undefined);

      await ml.dismissMLSuggestion(5);

      expect(put).toHaveBeenCalledWith('/ml-suggestions/5/dismiss', {});
    });
  });

  describe('applyMLSuggestion', () => {
    it('calls put with suggestion apply path', async () => {
      vi.mocked(put).mockResolvedValue(undefined);

      await ml.applyMLSuggestion(12);

      expect(put).toHaveBeenCalledWith('/ml-suggestions/12/apply', {});
    });
  });

  describe('getSynergyReport', () => {
    it('calls get with deck synergy-report path', async () => {
      const mockReport = { deckId: 'deck-1', cardCount: 40, totalPairs: 780, avgSynergyScore: 0.6, synergies: [] };
      vi.mocked(get).mockResolvedValue(mockReport);

      const result = await ml.getSynergyReport('deck-1');

      expect(get).toHaveBeenCalledWith('/decks/deck-1/synergy-report');
      expect(result).toEqual(mockReport);
    });
  });

  describe('getCardSynergies', () => {
    it('calls get with correct path and default params', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await ml.getCardSynergies(12345);

      expect(get).toHaveBeenCalledWith('/cards/12345/synergies?format=Standard&limit=10');
    });

    it('calls get with custom format and limit', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await ml.getCardSynergies(99, 'Pioneer', 25);

      expect(get).toHaveBeenCalledWith('/cards/99/synergies?format=Pioneer&limit=25');
    });
  });

  describe('getCombinationStats', () => {
    it('calls get with card pair params', async () => {
      vi.mocked(get).mockResolvedValue({});

      await ml.getCombinationStats(1, 2, 'Standard');

      expect(get).toHaveBeenCalledWith('/ml/combinations?card1=1&card2=2&format=Standard');
    });

    it('uses Standard as default format', async () => {
      vi.mocked(get).mockResolvedValue({});

      await ml.getCombinationStats(3, 4);

      expect(get).toHaveBeenCalledWith('/ml/combinations?card1=3&card2=4&format=Standard');
    });
  });

  describe('processMatchHistory', () => {
    it('calls post with default 90 days and no format', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'ok', message: 'done' });

      await ml.processMatchHistory();

      expect(post).toHaveBeenCalledWith('/ml/process-history?days=90', {});
    });

    it('calls post with custom days and format', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'ok', message: 'done' });

      await ml.processMatchHistory('Standard', 30);

      expect(post).toHaveBeenCalledWith('/ml/process-history?days=30&format=Standard', {});
    });
  });

  describe('getUserPlayPatterns', () => {
    it('calls get without account_id by default', async () => {
      vi.mocked(get).mockResolvedValue({});

      await ml.getUserPlayPatterns();

      expect(get).toHaveBeenCalledWith('/ml/play-patterns');
    });

    it('calls get with account_id param when provided', async () => {
      vi.mocked(get).mockResolvedValue({});

      await ml.getUserPlayPatterns('user-123');

      expect(get).toHaveBeenCalledWith('/ml/play-patterns?account_id=user-123');
    });
  });

  describe('updateUserPlayPatterns', () => {
    it('calls post without account_id by default', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'ok', message: 'updated' });

      await ml.updateUserPlayPatterns();

      expect(post).toHaveBeenCalledWith('/ml/play-patterns/update', {});
    });

    it('calls post with account_id param when provided', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'ok', message: 'updated' });

      await ml.updateUserPlayPatterns('user-456');

      expect(post).toHaveBeenCalledWith('/ml/play-patterns/update?account_id=user-456', {});
    });
  });

  describe('clearLearnedData', () => {
    it('calls del with ml learned-data path', async () => {
      vi.mocked(del).mockResolvedValue({ status: 'ok', message: 'cleared' });

      const result = await ml.clearLearnedData();

      expect(del).toHaveBeenCalledWith('/ml/learned-data');
      expect(result).toEqual({ status: 'ok', message: 'cleared' });
    });
  });
});

describe('mlSuggestions utility functions', () => {
  describe('parseReasons', () => {
    it('parses valid JSON array of reasons', () => {
      const json = '[{"type":"synergy","description":"Good pair","impact":0.1,"confidence":0.8}]';
      const result = ml.parseReasons(json);
      expect(result).toHaveLength(1);
      expect(result[0].type).toBe('synergy');
    });
    it('returns empty array for undefined', () => {
      expect(ml.parseReasons(undefined)).toEqual([]);
    });
    it('returns empty array for invalid JSON', () => {
      expect(ml.parseReasons('bad-json')).toEqual([]);
    });
  });

  describe('parseColorPreferences', () => {
    it('parses valid JSON color map', () => {
      expect(ml.parseColorPreferences('{"W":0.8,"U":0.5}')).toEqual({ W: 0.8, U: 0.5 });
    });
    it('returns empty object for undefined', () => {
      expect(ml.parseColorPreferences(undefined)).toEqual({});
    });
    it('returns empty object for invalid JSON', () => {
      expect(ml.parseColorPreferences('not-json')).toEqual({});
    });
  });

  describe('getMLSuggestionTypeLabel', () => {
    it('returns correct labels', () => {
      expect(ml.getMLSuggestionTypeLabel('add')).toBe('Add Card');
      expect(ml.getMLSuggestionTypeLabel('remove')).toBe('Remove Card');
      expect(ml.getMLSuggestionTypeLabel('swap')).toBe('Swap Cards');
    });
  });

  describe('formatConfidence', () => {
    it('formats fraction as percentage', () => {
      expect(ml.formatConfidence(0.75)).toBe('75%');
      expect(ml.formatConfidence(1.0)).toBe('100%');
      expect(ml.formatConfidence(0)).toBe('0%');
    });
  });

  describe('formatWinRateChange', () => {
    it('adds + sign for positive values', () => {
      expect(ml.formatWinRateChange(3.5)).toBe('+3.5%');
    });
    it('no sign for negative values', () => {
      expect(ml.formatWinRateChange(-2.1)).toBe('-2.1%');
    });
    it('adds + sign for zero', () => {
      expect(ml.formatWinRateChange(0)).toBe('+0.0%');
    });
  });

  describe('getConfidenceLevel', () => {
    it('returns high for >= 0.7', () => {
      expect(ml.getConfidenceLevel(0.7)).toBe('high');
      expect(ml.getConfidenceLevel(0.9)).toBe('high');
    });
    it('returns medium for >= 0.4 and < 0.7', () => {
      expect(ml.getConfidenceLevel(0.4)).toBe('medium');
      expect(ml.getConfidenceLevel(0.6)).toBe('medium');
    });
    it('returns low for < 0.4', () => {
      expect(ml.getConfidenceLevel(0.3)).toBe('low');
      expect(ml.getConfidenceLevel(0)).toBe('low');
    });
  });

  describe('getConfidenceColor', () => {
    it('returns green for high confidence', () => {
      expect(ml.getConfidenceColor(0.8)).toBe('text-green-400');
    });
    it('returns blue for medium confidence', () => {
      expect(ml.getConfidenceColor(0.5)).toBe('text-blue-400');
    });
    it('returns yellow for low confidence', () => {
      expect(ml.getConfidenceColor(0.2)).toBe('text-yellow-400');
    });
  });
});
