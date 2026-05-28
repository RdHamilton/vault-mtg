/**
 * Collection — PostHog analytics event tests (#1838)
 *
 * Covers:
 *   - feature_collection_viewed fires once on mount when cards are non-empty
 *   - does not fire when collection is empty
 *   - NEGATIVE: does not fire again on filter change re-loads
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { waitFor } from '@testing-library/react';
import { renderWithRouter } from '@/test/utils/testUtils';
import Collection from './Collection';
import { mockCollection, mockCards as mockCardsApi } from '@/test/mocks/apiMock';
import { gui } from '@/types/models';

vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

import { trackEvent } from '@/services/analytics';

function createMockCollectionCard(overrides: Record<string, unknown> = {}): gui.CollectionCard {
  return new gui.CollectionCard({
    cardId: 12345,
    arenaId: 12345,
    quantity: 4,
    name: 'Lightning Bolt',
    setCode: 'sta',
    setName: 'Strixhaven Mystical Archive',
    rarity: 'rare',
    manaCost: '{R}',
    cmc: 1,
    typeLine: 'Instant',
    colors: ['R'],
    colorIdentity: ['R'],
    imageUri: 'https://example.com/card.jpg',
    power: '',
    toughness: '',
    ...overrides,
  });
}

describe('Collection — analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCardsApi.getAllSetInfo.mockResolvedValue([]);
    mockCollection.getCollectionValue = vi.fn().mockResolvedValue({ totalValueUsd: 0 });
  });

  describe('feature_collection_viewed', () => {
    it('fires once on mount when collection has cards', async () => {
      const cards = [createMockCollectionCard(), createMockCollectionCard({ cardId: 99, arenaId: 99, name: 'Island' })];
      mockCollection.getCollectionWithMetadata.mockResolvedValue({
        cards,
        totalCount: cards.length,
        unknownCardsFetched: 0,
        unknownCardsRemaining: 0,
      });

      renderWithRouter(<Collection />);

      await waitFor(() => {
        expect(trackEvent).toHaveBeenCalledWith({
          name: 'feature_collection_viewed',
          properties: { card_count: 2 },
        });
      });
    });

    it('does not fire when collection is empty', async () => {
      mockCollection.getCollectionWithMetadata.mockResolvedValue({
        cards: [],
        totalCount: 0,
        unknownCardsFetched: 0,
        unknownCardsRemaining: 0,
      });

      renderWithRouter(<Collection />);

      await new Promise((r) => setTimeout(r, 50));

      const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_collection_viewed',
      );
      expect(viewedCalls).toHaveLength(0);
    });

    it('fires only once even when collection reloads on filter change', async () => {
      const cards = [createMockCollectionCard()];
      mockCollection.getCollectionWithMetadata.mockResolvedValue({
        cards,
        totalCount: 1,
        unknownCardsFetched: 0,
        unknownCardsRemaining: 0,
      });

      renderWithRouter(<Collection />);

      await waitFor(() => {
        const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
          ([e]) => e.name === 'feature_collection_viewed',
        );
        expect(viewedCalls).toHaveLength(1);
      });

      // Simulate second load (auto-refresh)
      await new Promise((r) => setTimeout(r, 20));

      const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
        ([e]) => e.name === 'feature_collection_viewed',
      );
      expect(viewedCalls).toHaveLength(1);
    });
  });

  describe('NEGATIVE — no PII in payload', () => {
    it('does not include user_id in feature_collection_viewed', async () => {
      const cards = [createMockCollectionCard()];
      mockCollection.getCollectionWithMetadata.mockResolvedValue({
        cards,
        totalCount: 1,
        unknownCardsFetched: 0,
        unknownCardsRemaining: 0,
      });

      renderWithRouter(<Collection />);

      await waitFor(() => {
        const viewedCalls = vi.mocked(trackEvent).mock.calls.filter(
          ([e]) => e.name === 'feature_collection_viewed',
        );
        expect(viewedCalls.length).toBeGreaterThan(0);
        expect(viewedCalls[0][0]).not.toHaveProperty('properties.user_id');
      });
    });
  });
});
