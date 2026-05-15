import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as decks from '../decks';

// Mock the apiClient — Phase 2 PR #9 routes decks.* through the BFF.
vi.mock('../../apiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

import { get, post, put, del } from '../../apiClient';

// ---------------------------------------------------------------------------
// Helpers — BFF wire shapes matching decks.go deckWithCardsResponse
// ---------------------------------------------------------------------------

/** Minimal BFF flat deck-detail response (matching decks.go deckWithCardsResponse). */
function makeBffDeckDetail(overrides: Record<string, unknown> = {}) {
  return {
    id: 'deck-123',
    name: 'Test Deck',
    format: 'standard',
    source: 'constructed',
    draftEventId: null,
    matchesPlayed: 0,
    matchesWon: 0,
    gamesPlayed: 0,
    gamesWon: 0,
    winRate: 0,
    isAppCreated: false,
    createdAt: '2025-01-01T00:00:00Z',
    modifiedAt: '2025-01-02T00:00:00Z',
    lastPlayed: null,
    colorIdentity: 'WU',
    description: '',
    cardCount: 2,
    tags: [],
    cards: [
      {
        cardId: 12345,
        quantity: 4,
        board: 'main',
        fromDraftPick: false,
        name: 'Lightning Bolt',
        setCode: 'M21',
        manaCost: '{R}',
        cmc: 1,
        typeLine: 'Instant',
        rarity: 'common',
        imageUri: 'https://example.com/card.jpg',
        colors: ['R'],
      },
    ],
    ...overrides,
  };
}

describe('decks API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getDecks', () => {
    it('should call get with correct path', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await decks.getDecks();

      expect(get).toHaveBeenCalledWith('/decks');
    });

    it('should call get with format query parameter', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await decks.getDecks({ format: 'Standard' });

      expect(get).toHaveBeenCalledWith('/decks?format=Standard');
    });

    it('should call get with source query parameter', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await decks.getDecks({ source: 'draft' });

      expect(get).toHaveBeenCalledWith('/decks?source=draft');
    });
  });

  describe('getDeck', () => {
    it('should call get with correct path', async () => {
      vi.mocked(get).mockResolvedValue(makeBffDeckDetail());

      await decks.getDeck('deck-123');

      expect(get).toHaveBeenCalledWith('/decks/deck-123');
    });

    it('should map flat BFF response to nested DeckWithCards shape', async () => {
      const bffResponse = makeBffDeckDetail({
        id: 'deck-abc',
        name: 'My Deck',
        format: 'limited',
        source: 'draft',
        draftEventId: 'event-xyz',
      });
      vi.mocked(get).mockResolvedValue(bffResponse);

      const result = await decks.getDeck('deck-abc');

      // deck must be a nested object — not undefined (the root bug)
      expect(result.deck).toBeDefined();
      expect(result.deck?.ID).toBe('deck-abc');
      expect(result.deck?.Name).toBe('My Deck');
      expect(result.deck?.Format).toBe('limited');
      expect(result.deck?.Source).toBe('draft');
      expect(result.deck?.DraftEventID).toBe('event-xyz');
    });

    it('should map BFF card fields (camelCase) to models.DeckCard (PascalCase)', async () => {
      const bffResponse = makeBffDeckDetail();
      vi.mocked(get).mockResolvedValue(bffResponse);

      const result = await decks.getDeck('deck-123');

      expect(result.cards).toHaveLength(1);
      const card = result.cards[0];
      // PascalCase field names must be present and correctly mapped
      expect(card.CardID).toBe(12345);
      expect(card.Quantity).toBe(4);
      expect(card.Board).toBe('main');
      expect(card.FromDraftPick).toBe(false);
    });

    it('should return empty cards array when BFF cards is empty', async () => {
      vi.mocked(get).mockResolvedValue(makeBffDeckDetail({ cards: [] }));

      const result = await decks.getDeck('deck-123');

      expect(result.cards).toHaveLength(0);
      expect(result.deck).toBeDefined();
    });

    it('should handle missing optional deck fields gracefully', async () => {
      vi.mocked(get).mockResolvedValue(makeBffDeckDetail({
        draftEventId: null,
        lastPlayed: null,
        description: undefined,
        colorIdentity: undefined,
      }));

      const result = await decks.getDeck('deck-123');

      expect(result.deck).toBeDefined();
      expect(result.deck?.DraftEventID).toBeUndefined();
      expect(result.deck?.LastPlayed).toBeUndefined();
    });
  });

  describe('createDeck', () => {
    it('should call post with correct path and body', async () => {
      vi.mocked(post).mockResolvedValue(makeBffDeckDetail({ id: 'new-deck', name: 'Test Deck' }));

      await decks.createDeck({
        name: 'Test Deck',
        format: 'Standard',
        source: 'constructed',
      });

      expect(post).toHaveBeenCalledWith('/decks', {
        name: 'Test Deck',
        format: 'Standard',
        source: 'constructed',
      });
    });

    it('should include draft_event_id when provided', async () => {
      vi.mocked(post).mockResolvedValue(makeBffDeckDetail({ id: 'new-deck', name: 'Draft Deck' }));

      await decks.createDeck({
        name: 'Draft Deck',
        format: 'Limited',
        source: 'draft',
        draft_event_id: 'draft-event-123',
      });

      expect(post).toHaveBeenCalledWith('/decks', {
        name: 'Draft Deck',
        format: 'Limited',
        source: 'draft',
        draft_event_id: 'draft-event-123',
      });
    });

    it('should return models.Deck with ID mapped from BFF id field', async () => {
      vi.mocked(post).mockResolvedValue(makeBffDeckDetail({ id: 'created-deck-id', name: 'My New Deck' }));

      const result = await decks.createDeck({ name: 'My New Deck', format: 'standard', source: 'manual' });

      // ID must be populated from the BFF "id" camelCase field
      expect(result.ID).toBe('created-deck-id');
      expect(result.Name).toBe('My New Deck');
    });
  });

  describe('updateDeck', () => {
    it('should call put with correct path and body', async () => {
      vi.mocked(put).mockResolvedValue(makeBffDeckDetail({ name: 'Updated Name' }));

      await decks.updateDeck('deck-123', { name: 'Updated Name' });

      expect(put).toHaveBeenCalledWith('/decks/deck-123', { name: 'Updated Name' });
    });

    it('should return DeckWithCards with mapped deck field', async () => {
      vi.mocked(put).mockResolvedValue(makeBffDeckDetail({ id: 'deck-123', name: 'Updated Name' }));

      const result = await decks.updateDeck('deck-123', { name: 'Updated Name' });

      expect(result.deck).toBeDefined();
      expect(result.deck?.ID).toBe('deck-123');
      expect(result.deck?.Name).toBe('Updated Name');
    });
  });

  describe('deleteDeck', () => {
    it('should call del with correct path', async () => {
      vi.mocked(del).mockResolvedValue(undefined);

      await decks.deleteDeck('deck-123');

      expect(del).toHaveBeenCalledWith('/decks/deck-123');
    });
  });

  describe('addCard', () => {
    it('should map frontend field names to backend field names', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'success' });

      await decks.addCard({
        deck_id: 'deck-123',
        arena_id: 12345,
        quantity: 4,
        zone: 'main',
        is_sideboard: false,
      });

      // Verify the backend receives the correct field names
      expect(post).toHaveBeenCalledWith('/decks/deck-123/cards', {
        cardID: 12345, // NOT arena_id
        quantity: 4,
        board: 'main', // NOT zone
        fromDraft: false,
      });
    });

    it('should map arena_id to card_id correctly', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'success' });

      await decks.addCard({
        deck_id: 'deck-456',
        arena_id: 99999,
        quantity: 2,
        zone: 'sideboard',
        is_sideboard: true,
      });

      expect(post).toHaveBeenCalledWith('/decks/deck-456/cards', {
        cardID: 99999,
        quantity: 2,
        board: 'sideboard',
        fromDraft: false,
      });
    });

    it('should pass from_draft flag when adding draft picks', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'success' });

      await decks.addCard({
        deck_id: 'draft-deck-123',
        arena_id: 54321,
        quantity: 1,
        zone: 'main',
        is_sideboard: false,
        from_draft: true,
      });

      expect(post).toHaveBeenCalledWith('/decks/draft-deck-123/cards', {
        cardID: 54321,
        quantity: 1,
        board: 'main',
        fromDraft: true,
      });
    });

    it('should default from_draft to false when not provided', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'success' });

      await decks.addCard({
        deck_id: 'deck-789',
        arena_id: 11111,
        quantity: 3,
        zone: 'main',
        is_sideboard: false,
      });

      expect(post).toHaveBeenCalledWith('/decks/deck-789/cards', {
        cardID: 11111,
        quantity: 3,
        board: 'main',
        fromDraft: false,
      });
    });
  });

  describe('removeCard', () => {
    it('should call del with correct path and query params', async () => {
      vi.mocked(del).mockResolvedValue(undefined);

      await decks.removeCard({
        deck_id: 'deck-123',
        arena_id: 12345,
        zone: 'main',
      });

      expect(del).toHaveBeenCalledWith('/decks/deck-123/cards/12345?zone=main');
    });
  });

  describe('exportDeck', () => {
    it('should call post with correct path and format', async () => {
      vi.mocked(post).mockResolvedValue({ content: 'deck list...' });

      await decks.exportDeck('deck-123', { format: 'mtga' });

      expect(post).toHaveBeenCalledWith('/decks/deck-123/export', { format: 'mtga' });
    });
  });

  describe('importDeck', () => {
    it('should call post with correct path and body', async () => {
      vi.mocked(post).mockResolvedValue({ deck: {}, cards: [] });

      await decks.importDeck({
        content: '4 Lightning Bolt',
        name: 'Burn',
        format: 'Modern',
      });

      expect(post).toHaveBeenCalledWith('/decks/import', {
        content: '4 Lightning Bolt',
        name: 'Burn',
        format: 'Modern',
      });
    });
  });

  describe('suggestDecks', () => {
    it('should call post with correct path and session_id', async () => {
      const mockResponse: decks.SuggestDecksApiResponse = {
        suggestions: [],
        totalCombos: 32,
        viableCombos: 5,
        bestCombo: { colors: ['W', 'U'], name: 'Azorius' },
      };
      vi.mocked(post).mockResolvedValue(mockResponse);

      const result = await decks.suggestDecks({ session_id: 'draft-session-123' });

      expect(post).toHaveBeenCalledWith('/decks/suggest', { session_id: 'draft-session-123' });
      expect(result).toEqual(mockResponse);
    });

    it('should return full response with suggestions, totals, and bestCombo', async () => {
      const mockSuggestion = {
        colorCombo: { colors: ['W', 'U'], name: 'Azorius' },
        spells: [],
        lands: [],
        totalCards: 40,
        score: 0.85,
        viability: 'strong',
      };
      const mockResponse: decks.SuggestDecksApiResponse = {
        suggestions: [mockSuggestion as any],
        totalCombos: 32,
        viableCombos: 14,
        bestCombo: { colors: ['W', 'U'], name: 'Azorius' },
      };
      vi.mocked(post).mockResolvedValue(mockResponse);

      const result = await decks.suggestDecks({ session_id: 'draft-456' });

      expect(result.suggestions).toHaveLength(1);
      expect(result.totalCombos).toBe(32);
      expect(result.viableCombos).toBe(14);
      expect(result.bestCombo?.name).toBe('Azorius');
    });

    it('should handle error response', async () => {
      const mockResponse: decks.SuggestDecksApiResponse = {
        suggestions: [],
        totalCombos: 0,
        viableCombos: 0,
        error: 'No cards in draft pool',
      };
      vi.mocked(post).mockResolvedValue(mockResponse);

      const result = await decks.suggestDecks({ session_id: 'empty-draft' });

      expect(result.error).toBe('No cards in draft pool');
      expect(result.suggestions).toHaveLength(0);
    });
  });

  describe('buildAroundSeed', () => {
    it('should call post with correct path and request', async () => {
      vi.mocked(post).mockResolvedValue({ seedCard: {}, suggestions: [], lands: [] });

      await decks.buildAroundSeed({
        seed_card_id: 12345,
        max_results: 40,
        budget_mode: true,
      });

      expect(post).toHaveBeenCalledWith('/decks/build-around', {
        seed_card_id: 12345,
        max_results: 40,
        budget_mode: true,
      });
    });
  });

  describe('suggestNextCards', () => {
    it('should call post with correct path and request', async () => {
      vi.mocked(post).mockResolvedValue({ suggestions: [], deckAnalysis: {}, slotsRemaining: 36 });

      await decks.suggestNextCards({
        seed_card_id: 12345,
        deck_card_ids: [111, 222, 333],
        max_results: 15,
      });

      expect(post).toHaveBeenCalledWith('/decks/build-around/suggest-next', {
        seed_card_id: 12345,
        deck_card_ids: [111, 222, 333],
        max_results: 15,
      });
    });
  });

  describe('getDeckStatistics', () => {
    it('should call get with correct path', async () => {
      vi.mocked(get).mockResolvedValue({ totalMainboard: 60 });

      await decks.getDeckStatistics('deck-123');

      expect(get).toHaveBeenCalledWith('/decks/deck-123/statistics');
    });
  });

  describe('validateDraftDeck', () => {
    it('should call get and return valid status', async () => {
      vi.mocked(get).mockResolvedValue({ valid: true });

      const result = await decks.validateDraftDeck('deck-123');

      expect(get).toHaveBeenCalledWith('/decks/deck-123/validate-draft');
      expect(result).toBe(true);
    });

    it('should return false when deck is invalid', async () => {
      vi.mocked(get).mockResolvedValue({ valid: false });

      const result = await decks.validateDraftDeck('deck-123');

      expect(result).toBe(false);
    });
  });

  describe('cloneDeck', () => {
    it('should call post with correct path and name', async () => {
      vi.mocked(post).mockResolvedValue(makeBffDeckDetail({ id: 'cloned-deck', name: 'Cloned Deck' }));

      await decks.cloneDeck('deck-123', 'Cloned Deck');

      expect(post).toHaveBeenCalledWith('/decks/deck-123/clone', { name: 'Cloned Deck' });
    });

    it('should return models.Deck with ID mapped from BFF id field', async () => {
      vi.mocked(post).mockResolvedValue(makeBffDeckDetail({ id: 'cloned-deck-id', name: 'Cloned Deck' }));

      const result = await decks.cloneDeck('deck-123', 'Cloned Deck');

      expect(result.ID).toBe('cloned-deck-id');
      expect(result.Name).toBe('Cloned Deck');
    });
  });

  describe('addTag', () => {
    it('should call post with correct path and tag', async () => {
      vi.mocked(post).mockResolvedValue(undefined);

      await decks.addTag('deck-123', 'aggro');

      expect(post).toHaveBeenCalledWith('/decks/deck-123/tags', { tag: 'aggro' });
    });
  });

  describe('removeTag', () => {
    it('should call del with correct path', async () => {
      vi.mocked(del).mockResolvedValue(undefined);

      await decks.removeTag('deck-123', 'aggro');

      expect(del).toHaveBeenCalledWith('/decks/deck-123/tags/aggro');
    });

    it('should encode special characters in tag', async () => {
      vi.mocked(del).mockResolvedValue(undefined);

      await decks.removeTag('deck-123', 'multi word tag');

      expect(del).toHaveBeenCalledWith('/decks/deck-123/tags/multi%20word%20tag');
    });
  });
});
