import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as cards from '../cards';

// Mock the apiClient — Phase 2 PR #8 routes cards.* through the BFF.
vi.mock('../../apiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn(),
  getRaw: vi.fn(),
}));

import { get, post, getRaw } from '../../apiClient';

describe('cards API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('searchCards', () => {
    it('should call get with query parameters', async () => {
      const mockCards = [{ name: 'Lightning Bolt' }];
      vi.mocked(get).mockResolvedValue(mockCards);

      const result = await cards.searchCards({ query: 'lightning' });

      expect(get).toHaveBeenCalledWith('/cards?q=lightning');
      expect(result).toEqual(mockCards);
    });

    it('should include set_code and limit in query parameters', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await cards.searchCards({
        query: 'bolt',
        set_code: 'MKM',
        limit: 20,
      });

      expect(get).toHaveBeenCalledWith('/cards?q=bolt&set=MKM&limit=20');
    });
  });

  describe('getCardByArenaId', () => {
    it('should call get with correct path (no /arena/ prefix)', async () => {
      const mockCard = { name: 'Opt', arena_id: 12345 };
      vi.mocked(get).mockResolvedValue(mockCard);

      const result = await cards.getCardByArenaId(12345);

      // Backend route is /cards/{cardID}, NOT /cards/arena/{cardID}
      expect(get).toHaveBeenCalledWith('/cards/12345');
      expect(result).toEqual(mockCard);
    });

    it('should correctly format different arena IDs', async () => {
      vi.mocked(get).mockResolvedValue({ name: 'Test Card' });

      await cards.getCardByArenaId(97326);

      expect(get).toHaveBeenCalledWith('/cards/97326');
    });
  });

  describe('getAllSetInfo', () => {
    it('should call get with correct path', async () => {
      const mockSets = [{ code: 'MKM', name: 'Murders at Karlov Manor' }];
      vi.mocked(get).mockResolvedValue(mockSets);

      const result = await cards.getAllSetInfo();

      expect(get).toHaveBeenCalledWith('/cards/sets');
      expect(result).toEqual(mockSets);
    });
  });

  describe('getSetCards', () => {
    it('should call get with correct path', async () => {
      const mockCards = [{ name: 'Card 1' }, { name: 'Card 2' }];
      vi.mocked(get).mockResolvedValue(mockCards);

      const result = await cards.getSetCards('MKM');

      expect(get).toHaveBeenCalledWith('/cards/sets/MKM/cards');
      expect(result).toEqual(mockCards);
    });
  });

  describe('getCardRatings', () => {
    it('should call get with correct path', async () => {
      const mockRatings = [{ name: 'Card 1', ever_drawn_win_rate: 0.55 }];
      vi.mocked(get).mockResolvedValue(mockRatings);

      const result = await cards.getCardRatings('MKM', 'PremierDraft');

      expect(get).toHaveBeenCalledWith('/cards/ratings/MKM/PremierDraft');
      expect(result).toEqual(mockRatings);
    });
  });

  describe('getCollectionQuantities', () => {
    it('should call post with arena IDs', async () => {
      const mockQuantities = { 12345: 4, 67890: 2 };
      vi.mocked(post).mockResolvedValue(mockQuantities);

      const result = await cards.getCollectionQuantities([12345, 67890]);

      expect(post).toHaveBeenCalledWith('/cards/collection-quantities', {
        arenaIDs: [12345, 67890],
      });
      expect(result).toEqual(mockQuantities);
    });
  });

  describe('getColorRatings', () => {
    it('should call get with correct path', async () => {
      const mockRatings = [{ color: 'W', win_rate: 0.52 }];
      vi.mocked(get).mockResolvedValue(mockRatings);

      const result = await cards.getColorRatings('MKM', 'PremierDraft');

      // Backend route is /cards/ratings/{setCode}/colors (format param not used in path)
      expect(get).toHaveBeenCalledWith('/cards/ratings/MKM/colors');
      expect(result).toEqual(mockRatings);
    });
  });

  describe('searchCardsWithCollection', () => {
    it('should call post with search parameters', async () => {
      const mockCards = [{ name: 'Card 1', quantity: 4 }];
      vi.mocked(post).mockResolvedValue(mockCards);

      const result = await cards.searchCardsWithCollection('lightning', ['MKM'], 50);

      expect(post).toHaveBeenCalledWith('/cards/search-with-collection', {
        query: 'lightning',
        setCodes: ['MKM'],
        limit: 50,
      });
      expect(result).toEqual(mockCards);
    });

    it('should handle missing optional parameters', async () => {
      vi.mocked(post).mockResolvedValue([]);

      await cards.searchCardsWithCollection('bolt');

      expect(post).toHaveBeenCalledWith('/cards/search-with-collection', {
        query: 'bolt',
        setCodes: undefined,
        limit: undefined,
      });
    });
  });

  describe('getCardRatingsWithDegradedFlag', () => {
    it('returns cacheDegraded=false when header is absent', async () => {
      const mockRatings = [{ name: 'Card 1', ever_drawn_win_rate: 0.55 }];
      const mockHeaders = new Headers(); // no X-Cache-Degraded header
      vi.mocked(getRaw).mockResolvedValue({ data: mockRatings, headers: mockHeaders });

      const result = await cards.getCardRatingsWithDegradedFlag('MKM', 'PremierDraft');

      expect(getRaw).toHaveBeenCalledWith('/cards/ratings/MKM/PremierDraft');
      expect(result.ratings).toEqual(mockRatings);
      expect(result.cacheDegraded).toBe(false);
    });

    it('returns cacheDegraded=true when X-Cache-Degraded: true header is present', async () => {
      const mockRatings = [{ name: 'Card 1', ever_drawn_win_rate: 0.55 }];
      const mockHeaders = new Headers({ 'x-cache-degraded': 'true' });
      vi.mocked(getRaw).mockResolvedValue({ data: mockRatings, headers: mockHeaders });

      const result = await cards.getCardRatingsWithDegradedFlag('MKM', 'PremierDraft');

      expect(result.cacheDegraded).toBe(true);
      expect(result.ratings).toEqual(mockRatings);
    });

    it('returns cacheDegraded=false when X-Cache-Degraded has a non-true value', async () => {
      const mockHeaders = new Headers({ 'x-cache-degraded': 'false' });
      vi.mocked(getRaw).mockResolvedValue({ data: [], headers: mockHeaders });

      const result = await cards.getCardRatingsWithDegradedFlag('OTJ', 'PremierDraft');

      expect(result.cacheDegraded).toBe(false);
    });

    it('returns cacheAgeHours when X-Cache-Age-Hours header is present', async () => {
      const mockHeaders = new Headers({ 'x-cache-degraded': 'true', 'x-cache-age-hours': '4' });
      vi.mocked(getRaw).mockResolvedValue({ data: [], headers: mockHeaders });

      const result = await cards.getCardRatingsWithDegradedFlag('MKM', 'PremierDraft');

      expect(result.cacheAgeHours).toBe(4);
    });

    it('returns cacheAgeHours as float when X-Cache-Age-Hours is a decimal', async () => {
      const mockHeaders = new Headers({ 'x-cache-age-hours': '2.5' });
      vi.mocked(getRaw).mockResolvedValue({ data: [], headers: mockHeaders });

      const result = await cards.getCardRatingsWithDegradedFlag('MKM', 'PremierDraft');

      expect(result.cacheAgeHours).toBe(2.5);
    });

    it('returns cacheAgeHours=undefined when X-Cache-Age-Hours header is absent', async () => {
      const mockHeaders = new Headers();
      vi.mocked(getRaw).mockResolvedValue({ data: [], headers: mockHeaders });

      const result = await cards.getCardRatingsWithDegradedFlag('MKM', 'PremierDraft');

      expect(result.cacheAgeHours).toBeUndefined();
    });

    it('returns cacheAgeHours=undefined when X-Cache-Age-Hours is non-numeric', async () => {
      const mockHeaders = new Headers({ 'x-cache-age-hours': 'not-a-number' });
      vi.mocked(getRaw).mockResolvedValue({ data: [], headers: mockHeaders });

      const result = await cards.getCardRatingsWithDegradedFlag('MKM', 'PremierDraft');

      expect(result.cacheAgeHours).toBeUndefined();
    });
  });
});
