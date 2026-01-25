import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import DeckHistoryModal from './DeckHistoryModal';

// Note: The API module is mocked globally in src/test/setup.ts
// We import the mocks for test manipulation
import { mockDecks } from '@/test/mocks/apiMock';

describe('DeckHistoryModal', () => {
  const defaultProps = {
    deckId: 'test-deck-1',
    deckName: 'Test Deck',
    isOpen: true,
    onClose: vi.fn(),
    onRestore: vi.fn(),
  };

  const mockPermutations = [
    {
      id: 3,
      deckID: 'test-deck-1',
      versionNumber: 3,
      versionName: 'Final Version',
      matchesPlayed: 10,
      matchesWon: 7,
      matchWinRate: 70.0,
      gamesPlayed: 25,
      gamesWon: 18,
      gameWinRate: 72.0,
      createdAt: '2024-01-20T12:00:00Z',
      isCurrent: true,
      cards: [],
    },
    {
      id: 2,
      deckID: 'test-deck-1',
      versionNumber: 2,
      versionName: null,
      matchesPlayed: 5,
      matchesWon: 3,
      matchWinRate: 60.0,
      gamesPlayed: 12,
      gamesWon: 7,
      gameWinRate: 58.3,
      createdAt: '2024-01-15T12:00:00Z',
      isCurrent: false,
      cards: [],
    },
    {
      id: 1,
      deckID: 'test-deck-1',
      versionNumber: 1,
      versionName: 'Initial Build',
      matchesPlayed: 3,
      matchesWon: 1,
      matchWinRate: 33.3,
      gamesPlayed: 8,
      gamesWon: 3,
      gameWinRate: 37.5,
      createdAt: '2024-01-10T12:00:00Z',
      isCurrent: false,
      cards: [],
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockDecks.getDeckPermutations.mockResolvedValue(mockPermutations);
    mockDecks.getDeckPermutationDiff.mockResolvedValue({
      fromPermutationID: 2,
      toPermutationID: 3,
      addedCards: [],
      removedCards: [],
      changedCards: [],
    });
  });

  it('should render modal when open', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Deck History: Test Deck/)).toBeInTheDocument();
    });
  });

  it('should not render when closed', () => {
    render(<DeckHistoryModal {...defaultProps} isOpen={false} />);

    expect(screen.queryByText(/Deck History/)).not.toBeInTheDocument();
  });

  it('should display loading state initially', async () => {
    // Make the mock hang to show loading state
    mockDecks.getDeckPermutations.mockImplementation(
      () => new Promise(() => {})
    );

    render(<DeckHistoryModal {...defaultProps} />);

    expect(screen.getByText(/Loading deck history/)).toBeInTheDocument();
  });

  it('should display permutation versions after loading', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('v3')).toBeInTheDocument();
      expect(screen.getByText('v2')).toBeInTheDocument();
      expect(screen.getByText('v1')).toBeInTheDocument();
    });
  });

  it('should display "(Current)" badge for current permutation', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/\(Current\)/)).toBeInTheDocument();
    });
  });

  it('should select current permutation by default', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      // The version 3 item (current) should have the 'selected' class
      // Find the version-item that contains the current badge
      const selectedItem = document.querySelector('.version-item.selected');
      expect(selectedItem).toBeInTheDocument();
      expect(selectedItem?.querySelector('.current-badge')).toBeInTheDocument();
    });
  });

  it('should display match statistics for selected permutation', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      // Should show stats for the current (v3) permutation
      expect(screen.getByText('10 played')).toBeInTheDocument();
      expect(screen.getByText('70.0%')).toBeInTheDocument();
    });
  });

  it('should display "No matches" for permutation with 0 matches played', async () => {
    const permutationsWithNoMatches = [
      {
        ...mockPermutations[0],
        matchesPlayed: 0,
        matchesWon: 0,
        matchWinRate: 0,
      },
    ];
    mockDecks.getDeckPermutations.mockResolvedValue(permutationsWithNoMatches);

    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('No matches')).toBeInTheDocument();
    });
  });

  it('should disable restore button for current permutation', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      const restoreButton = screen.getByRole('button', { name: /Restore This Version/i });
      expect(restoreButton).toBeDisabled();
    });
  });

  it('should enable restore button for non-current permutation', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('v2')).toBeInTheDocument();
    });

    // Click on v2 to select it
    const v2Item = screen.getByText('v2').closest('.version-item');
    if (v2Item) {
      fireEvent.click(v2Item);
    }

    await waitFor(() => {
      const restoreButton = screen.getByRole('button', { name: /Restore This Version/i });
      expect(restoreButton).not.toBeDisabled();
    });
  });

  it('should call onClose when close button is clicked', async () => {
    const user = userEvent.setup();
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Deck History/)).toBeInTheDocument();
    });

    const closeButton = screen.getByRole('button', { name: /Close/i });
    await user.click(closeButton);

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('should call onClose when clicking overlay', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Deck History/)).toBeInTheDocument();
    });

    const overlay = document.querySelector('.modal-overlay');
    if (overlay) {
      fireEvent.click(overlay);
    }

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('should not close when clicking modal content', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Deck History/)).toBeInTheDocument();
    });

    const modalContent = document.querySelector('.deck-history-modal');
    if (modalContent) {
      fireEvent.click(modalContent);
    }

    expect(defaultProps.onClose).not.toHaveBeenCalled();
  });

  it('should display empty state when no permutations exist', async () => {
    mockDecks.getDeckPermutations.mockResolvedValue([]);

    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/No version history available/)).toBeInTheDocument();
    });
  });

  it('should display error message on API failure', async () => {
    mockDecks.getDeckPermutations.mockRejectedValue(new Error('API Error'));

    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/API Error/)).toBeInTheDocument();
    });
  });

  it('should allow editing version name', async () => {
    const user = userEvent.setup();
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Final Version')).toBeInTheDocument();
    });

    // Click on the version name to edit
    const versionName = screen.getByText('Final Version');
    await user.click(versionName);

    // Should show input field
    await waitFor(() => {
      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });
  });

  it('should call restoreDeckPermutation when restore is confirmed', async () => {
    // Mock window.confirm to return true
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('v2')).toBeInTheDocument();
    });

    // Select v2
    const v2Item = screen.getByText('v2').closest('.version-item');
    if (v2Item) {
      fireEvent.click(v2Item);
    }

    await waitFor(() => {
      const restoreButton = screen.getByRole('button', { name: /Restore This Version/i });
      expect(restoreButton).not.toBeDisabled();
    });

    const restoreButton = screen.getByRole('button', { name: /Restore This Version/i });
    fireEvent.click(restoreButton);

    await waitFor(() => {
      expect(mockDecks.restoreDeckPermutation).toHaveBeenCalledWith('test-deck-1', 2);
    });
  });

  it('should not restore when confirm is cancelled', async () => {
    // Mock window.confirm to return false
    vi.spyOn(window, 'confirm').mockReturnValue(false);

    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('v2')).toBeInTheDocument();
    });

    // Select v2
    const v2Item = screen.getByText('v2').closest('.version-item');
    if (v2Item) {
      fireEvent.click(v2Item);
    }

    await waitFor(() => {
      const restoreButton = screen.getByRole('button', { name: /Restore This Version/i });
      fireEvent.click(restoreButton);
    });

    expect(mockDecks.restoreDeckPermutation).not.toHaveBeenCalled();
  });

  it('should display win/loss record for permutations with matches', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      // v3: 7W-3L (70.0%)
      expect(screen.getByText(/7W-3L/)).toBeInTheDocument();
    });
  });

  it('should load diff when selecting a different permutation', async () => {
    render(<DeckHistoryModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('v2')).toBeInTheDocument();
    });

    // Select v2
    const v2Item = screen.getByText('v2').closest('.version-item');
    if (v2Item) {
      fireEvent.click(v2Item);
    }

    await waitFor(() => {
      expect(mockDecks.getDeckPermutationDiff).toHaveBeenCalled();
    });
  });
});
