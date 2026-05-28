import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import SuggestDecksModal from './SuggestDecksModal';
import * as Sentry from '@sentry/react';

// Mock the REST API service
vi.mock('@/services/api', () => ({
  decks: {
    suggestDecks: vi.fn(),
    applySuggestedDeck: vi.fn(),
    getSuggestedDeckExportContent: vi.fn(),
  },
  SuggestDecksApiResponse: {},
}));

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

import { decks } from '@/services/api';
import type { SuggestDecksApiResponse } from '@/services/api/decks';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('SuggestDecksModal', () => {
  it('should not render when isOpen is false', () => {
    render(
      <SuggestDecksModal
        isOpen={false}
        onClose={() => {}}
        draftEventID="test-draft-id"
        currentDeckID="test-deck-id"
        deckName="Test Deck"
        onDeckApplied={() => {}}
      />
    );

    expect(screen.queryByText('Suggested Decks')).not.toBeInTheDocument();
  });

  it('should render modal content when open', async () => {
    render(
      <SuggestDecksModal
        isOpen={true}
        onClose={() => {}}
        draftEventID="test-draft-id"
        currentDeckID="test-deck-id"
        deckName="Test Deck"
        onDeckApplied={() => {}}
      />
    );

    // Modal should be visible with content area
    expect(document.querySelector('.suggest-decks-content')).toBeInTheDocument();
  });

  it('should call onClose when close button is clicked', async () => {
    const onClose = vi.fn();

    render(
      <SuggestDecksModal
        isOpen={true}
        onClose={onClose}
        draftEventID="test-draft-id"
        currentDeckID="test-deck-id"
        deckName="Test Deck"
        onDeckApplied={() => {}}
      />
    );

    const closeButton = screen.getByRole('button', { name: /×/ });
    fireEvent.click(closeButton);

    expect(onClose).toHaveBeenCalled();
  });

  it('should call onClose when clicking overlay', async () => {
    const onClose = vi.fn();

    render(
      <SuggestDecksModal
        isOpen={true}
        onClose={onClose}
        draftEventID="test-draft-id"
        currentDeckID="test-deck-id"
        deckName="Test Deck"
        onDeckApplied={() => {}}
      />
    );

    // Click the overlay (background)
    const overlay = document.querySelector('.suggest-decks-overlay');
    if (overlay) {
      fireEvent.click(overlay);
    }

    expect(onClose).toHaveBeenCalled();
  });

  it('should render modal header when open', () => {
    render(
      <SuggestDecksModal
        isOpen={true}
        onClose={() => {}}
        draftEventID="test-draft-id"
        currentDeckID="test-deck-id"
        deckName="Test Deck"
        onDeckApplied={() => {}}
      />
    );

    expect(screen.getByText('Suggested Decks')).toBeInTheDocument();
  });

  it('should display viable combinations count from API response', async () => {
    const mockResponse = {
      suggestions: [{
        colorCombo: { colors: ['W', 'U'], name: 'Azorius' },
        spells: [],
        lands: [],
        totalCards: 40,
        score: 0.85,
        viability: 'strong',
      }],
      totalCombos: 32,
      viableCombos: 14,
      bestCombo: { colors: ['W', 'U'], name: 'Azorius' },
    } as unknown as SuggestDecksApiResponse;
    vi.mocked(decks.suggestDecks).mockResolvedValue(mockResponse);

    render(
      <SuggestDecksModal
        isOpen={true}
        onClose={() => {}}
        draftEventID="test-draft-id"
        currentDeckID="test-deck-id"
        deckName="Test Deck"
        onDeckApplied={() => {}}
      />
    );

    // Wait for the API call to complete and UI to update
    await waitFor(() => {
      expect(screen.getByText('14')).toBeInTheDocument();
    });

    // Should show the count from the API response
    // The text is split across multiple elements like "Found <strong>14</strong> viable color combinations out of 32 possible."
    expect(screen.getByText(/viable color combinations/)).toBeInTheDocument();
    expect(screen.getByText(/out of/)).toBeInTheDocument();
  });

  it('should display error message when API returns error', async () => {
    const mockResponse: SuggestDecksApiResponse = {
      suggestions: [],
      totalCombos: 0,
      viableCombos: 0,
      error: 'No cards in draft pool',
    };
    vi.mocked(decks.suggestDecks).mockResolvedValue(mockResponse);

    render(
      <SuggestDecksModal
        isOpen={true}
        onClose={() => {}}
        draftEventID="test-draft-id"
        currentDeckID="test-deck-id"
        deckName="Test Deck"
        onDeckApplied={() => {}}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('No cards in draft pool')).toBeInTheDocument();
    });
  });

  describe('Sentry error reporting', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('calls reportError with load_suggestions when suggestDecks throws', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      vi.mocked(decks.suggestDecks).mockRejectedValue(new Error('network error'));

      render(
        <SuggestDecksModal
          isOpen={true}
          onClose={() => {}}
          draftEventID="test-draft-id"
          currentDeckID="test-deck-id"
          deckName="Test Deck"
          onDeckApplied={() => {}}
        />
      );

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalledOnce();
      });

      const callArgs = sentryCapture.mock.calls[0][1] as { tags?: Record<string, string> };
      expect(callArgs?.tags).toMatchObject({ component: 'SuggestDecksModal', action: 'load_suggestions' });
    });

    it('calls reportError with apply_deck when applySuggestedDeck throws', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      const mockResponse = {
        suggestions: [{
          colorCombo: { colors: ['W', 'U'], name: 'Azorius' },
          spells: [],
          lands: [],
          totalCards: 40,
          score: 0.85,
          viability: 'strong',
        }],
        totalCombos: 32,
        viableCombos: 1,
      } as unknown as SuggestDecksApiResponse;
      vi.mocked(decks.suggestDecks).mockResolvedValue(mockResponse);
      vi.mocked(decks.applySuggestedDeck).mockRejectedValue(new Error('apply failed'));

      render(
        <SuggestDecksModal
          isOpen={true}
          onClose={() => {}}
          draftEventID="test-draft-id"
          currentDeckID="test-deck-id"
          deckName="Test Deck"
          onDeckApplied={() => {}}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Azorius')).toBeInTheDocument();
      });

      const useBtn = screen.getByRole('button', { name: /use this deck/i });
      fireEvent.click(useBtn);

      await waitFor(() => {
        const applyCall = sentryCapture.mock.calls.find(
          (c) => (c[1] as { tags?: Record<string, string> })?.tags?.action === 'apply_deck'
        );
        expect(applyCall).toBeDefined();
      });
    });
  });
});
