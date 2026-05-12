import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as notes from '../notes';

// Mock apiClient — Phase 2 PR #7 routes notes.* through the BFF.
vi.mock('../../apiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

import { get, post, put, del } from '../../apiClient';

describe('notes API (daemon routes)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // -------------------- Deck Notes --------------------

  describe('getDeckNotes', () => {
    it('calls get without category param when not provided', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await notes.getDeckNotes('deck-1');

      expect(get).toHaveBeenCalledWith('/decks/deck-1/notes');
    });

    it('calls get with category param when provided', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await notes.getDeckNotes('deck-1', 'general');

      expect(get).toHaveBeenCalledWith('/decks/deck-1/notes?category=general');
    });
  });

  describe('getDeckNote', () => {
    it('calls get with deck and note id in path', async () => {
      const mockNote = { id: 42, deckId: 'deck-1', content: 'Test', category: 'general', createdAt: '', updatedAt: '' };
      vi.mocked(get).mockResolvedValue(mockNote);

      const result = await notes.getDeckNote('deck-1', 42);

      expect(get).toHaveBeenCalledWith('/decks/deck-1/notes/42');
      expect(result).toEqual(mockNote);
    });
  });

  describe('createDeckNote', () => {
    it('calls post with deck id and request body', async () => {
      const req = { content: 'New note', category: 'matchup' as notes.NoteCategory };
      const mockNote = { id: 1, deckId: 'deck-1', content: 'New note', category: 'matchup', createdAt: '', updatedAt: '' };
      vi.mocked(post).mockResolvedValue(mockNote);

      const result = await notes.createDeckNote('deck-1', req);

      expect(post).toHaveBeenCalledWith('/decks/deck-1/notes', req);
      expect(result).toEqual(mockNote);
    });
  });

  describe('updateDeckNote', () => {
    it('calls put with correct path and body', async () => {
      const req = { content: 'Updated note' };
      const mockNote = { id: 5, deckId: 'deck-1', content: 'Updated note', category: 'general', createdAt: '', updatedAt: '' };
      vi.mocked(put).mockResolvedValue(mockNote);

      const result = await notes.updateDeckNote('deck-1', 5, req);

      expect(put).toHaveBeenCalledWith('/decks/deck-1/notes/5', req);
      expect(result).toEqual(mockNote);
    });
  });

  describe('deleteDeckNote', () => {
    it('calls del with correct path', async () => {
      vi.mocked(del).mockResolvedValue(undefined);

      await notes.deleteDeckNote('deck-1', 7);

      expect(del).toHaveBeenCalledWith('/decks/deck-1/notes/7');
    });
  });

  // -------------------- Match Notes --------------------

  describe('getMatchNotes', () => {
    it('calls get with correct match notes path', async () => {
      const mockMatchNotes = { matchId: 'match-1', notes: 'Good game', rating: 4 };
      vi.mocked(get).mockResolvedValue(mockMatchNotes);

      const result = await notes.getMatchNotes('match-1');

      expect(get).toHaveBeenCalledWith('/matches/match-1/notes');
      expect(result).toEqual(mockMatchNotes);
    });
  });

  describe('updateMatchNotes', () => {
    it('calls put with match id and request body', async () => {
      const req = { notes: 'Revised notes', rating: 3 };
      const mockMatchNotes = { matchId: 'match-1', notes: 'Revised notes', rating: 3 };
      vi.mocked(put).mockResolvedValue(mockMatchNotes);

      const result = await notes.updateMatchNotes('match-1', req);

      expect(put).toHaveBeenCalledWith('/matches/match-1/notes', req);
      expect(result).toEqual(mockMatchNotes);
    });
  });

  // -------------------- Suggestions --------------------

  describe('getDeckSuggestions', () => {
    it('calls get without active param when activeOnly is true (default)', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await notes.getDeckSuggestions('deck-1');

      expect(get).toHaveBeenCalledWith('/decks/deck-1/suggestions');
    });

    it('calls get with active=false param when activeOnly is false', async () => {
      vi.mocked(get).mockResolvedValue([]);

      await notes.getDeckSuggestions('deck-1', false);

      expect(get).toHaveBeenCalledWith('/decks/deck-1/suggestions?active=false');
    });
  });

  describe('generateSuggestions', () => {
    it('calls post with no min_games param when default (5)', async () => {
      vi.mocked(post).mockResolvedValue([]);

      await notes.generateSuggestions('deck-1');

      expect(post).toHaveBeenCalledWith('/decks/deck-1/suggestions/generate', {});
    });

    it('calls post with custom min_games param', async () => {
      vi.mocked(post).mockResolvedValue([]);

      await notes.generateSuggestions('deck-1', 10);

      expect(post).toHaveBeenCalledWith('/decks/deck-1/suggestions/generate?min_games=10', {});
    });
  });

  describe('dismissSuggestion', () => {
    it('calls put with suggestion id path', async () => {
      vi.mocked(put).mockResolvedValue(undefined);

      await notes.dismissSuggestion(99);

      expect(put).toHaveBeenCalledWith('/suggestions/99/dismiss', {});
    });
  });
});

describe('notes utility functions', () => {
  describe('parseEvidence', () => {
    it('parses valid JSON object', () => {
      const result = notes.parseEvidence<{ totalGames: number }>('{"totalGames":10}');
      expect(result).toEqual({ totalGames: 10 });
    });
    it('returns null for undefined', () => {
      expect(notes.parseEvidence(undefined)).toBeNull();
    });
    it('returns null for invalid JSON', () => {
      expect(notes.parseEvidence('not-json')).toBeNull();
    });
  });

  describe('getSuggestionTypeLabel', () => {
    it('returns correct label for each type', () => {
      expect(notes.getSuggestionTypeLabel('curve')).toBe('Mana Curve');
      expect(notes.getSuggestionTypeLabel('removal')).toBe('Removal');
      expect(notes.getSuggestionTypeLabel('mana')).toBe('Mana Base');
      expect(notes.getSuggestionTypeLabel('sequencing')).toBe('Play Sequencing');
      expect(notes.getSuggestionTypeLabel('sideboard')).toBe('Sideboard');
    });
  });

  describe('getPriorityLabel', () => {
    it('returns correct label for each priority', () => {
      expect(notes.getPriorityLabel('low')).toBe('Low Priority');
      expect(notes.getPriorityLabel('medium')).toBe('Medium Priority');
      expect(notes.getPriorityLabel('high')).toBe('High Priority');
    });
  });

  describe('getPriorityColor', () => {
    it('returns correct color class for each priority', () => {
      expect(notes.getPriorityColor('low')).toBe('text-blue-400');
      expect(notes.getPriorityColor('medium')).toBe('text-yellow-400');
      expect(notes.getPriorityColor('high')).toBe('text-red-400');
    });
  });
});
