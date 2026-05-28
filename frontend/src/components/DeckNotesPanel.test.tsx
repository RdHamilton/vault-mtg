import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { mockNotes } from '@/test/mocks/apiMock';
import type { DeckNote } from '@/services/api/notes';
import * as Sentry from '@sentry/react';
import DeckNotesPanel from './DeckNotesPanel';

// Mock the API module
vi.mock('@/services/api', () => ({
  notes: mockNotes,
}));

// Mock @sentry/react so we can spy on captureException
vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

// Helper to create mock notes
function createMockNote(overrides: Partial<DeckNote> = {}): DeckNote {
  return {
    id: 1,
    deckId: 'deck-1',
    content: 'Test note content',
    category: 'general',
    createdAt: '2024-01-15T10:00:00Z',
    updatedAt: '2024-01-15T10:00:00Z',
    ...overrides,
  };
}

describe('DeckNotesPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading State', () => {
    it('should show loading spinner while fetching notes', async () => {
      let resolvePromise: (value: DeckNote[]) => void;
      const loadingPromise = new Promise<DeckNote[]>((resolve) => {
        resolvePromise = resolve;
      });
      mockNotes.getDeckNotes.mockReturnValue(loadingPromise);

      render(<DeckNotesPanel deckId="deck-1" />);

      expect(screen.getByText('Loading notes...')).toBeInTheDocument();

      resolvePromise!([createMockNote()]);
      await waitFor(() => {
        expect(screen.queryByText('Loading notes...')).not.toBeInTheDocument();
      });
    });
  });

  describe('Empty State', () => {
    it('should show empty state when no notes exist', async () => {
      mockNotes.getDeckNotes.mockResolvedValue([]);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('No notes yet.')).toBeInTheDocument();
      });
    });
  });

  describe('Notes List', () => {
    it('should display notes when loaded', async () => {
      const notes = [
        createMockNote({ id: 1, content: 'First note' }),
        createMockNote({ id: 2, content: 'Second note' }),
      ];
      mockNotes.getDeckNotes.mockResolvedValue(notes);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('First note')).toBeInTheDocument();
        expect(screen.getByText('Second note')).toBeInTheDocument();
      });
    });

    it('should display note category badge', async () => {
      const notes = [createMockNote({ category: 'matchup' })];
      mockNotes.getDeckNotes.mockResolvedValue(notes);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('matchup')).toBeInTheDocument();
      });
    });
  });

  describe('Add Note', () => {
    it('should show add note form when button clicked', async () => {
      mockNotes.getDeckNotes.mockResolvedValue([]);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('+ Add Note')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('+ Add Note'));

      expect(screen.getByPlaceholderText('Write your note here...')).toBeInTheDocument();
      expect(screen.getByText('Save Note')).toBeInTheDocument();
    });

    it('should create note when form submitted', async () => {
      mockNotes.getDeckNotes.mockResolvedValue([]);
      mockNotes.createDeckNote.mockResolvedValue(createMockNote({ content: 'New note' }));

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        fireEvent.click(screen.getByText('+ Add Note'));
      });

      const textarea = screen.getByPlaceholderText('Write your note here...');
      fireEvent.change(textarea, { target: { value: 'New note content' } });

      fireEvent.click(screen.getByText('Save Note'));

      await waitFor(() => {
        expect(mockNotes.createDeckNote).toHaveBeenCalledWith('deck-1', {
          content: 'New note content',
          category: 'general',
        });
      });
    });

    it('should cancel add note form', async () => {
      mockNotes.getDeckNotes.mockResolvedValue([]);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        fireEvent.click(screen.getByText('+ Add Note'));
      });

      expect(screen.getByText('Cancel')).toBeInTheDocument();
      fireEvent.click(screen.getByText('Cancel'));

      expect(screen.queryByPlaceholderText('Write your note here...')).not.toBeInTheDocument();
    });
  });

  describe('Edit Note', () => {
    it('should show edit form when edit button clicked', async () => {
      const notes = [createMockNote({ content: 'Original content' })];
      mockNotes.getDeckNotes.mockResolvedValue(notes);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('Original content')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle('Edit note'));

      const textarea = screen.getByDisplayValue('Original content');
      expect(textarea).toBeInTheDocument();
    });

    it('should update note when edit saved', async () => {
      const notes = [createMockNote({ id: 1, content: 'Original' })];
      mockNotes.getDeckNotes.mockResolvedValue(notes);
      mockNotes.updateDeckNote.mockResolvedValue(createMockNote({ content: 'Updated' }));

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        fireEvent.click(screen.getByTitle('Edit note'));
      });

      const textarea = screen.getByDisplayValue('Original');
      fireEvent.change(textarea, { target: { value: 'Updated content' } });

      fireEvent.click(screen.getByText('Save'));

      await waitFor(() => {
        expect(mockNotes.updateDeckNote).toHaveBeenCalledWith('deck-1', 1, {
          content: 'Updated content',
          category: 'general',
        });
      });
    });
  });

  describe('Delete Note', () => {
    it('should delete note when delete button clicked', async () => {
      const notes = [createMockNote({ id: 1, content: 'To delete' })];
      mockNotes.getDeckNotes.mockResolvedValue(notes);
      mockNotes.deleteDeckNote.mockResolvedValue(undefined);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('To delete')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle('Delete note'));

      await waitFor(() => {
        expect(mockNotes.deleteDeckNote).toHaveBeenCalledWith('deck-1', 1);
      });
    });
  });

  describe('Category Filter', () => {
    it('should filter notes by category', async () => {
      mockNotes.getDeckNotes.mockResolvedValue([]);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByRole('combobox')).toBeInTheDocument();
      });

      fireEvent.change(screen.getByRole('combobox'), { target: { value: 'matchup' } });

      await waitFor(() => {
        expect(mockNotes.getDeckNotes).toHaveBeenCalledWith('deck-1', 'matchup');
      });
    });
  });

  describe('Close Button', () => {
    it('should call onClose when close button clicked', async () => {
      mockNotes.getDeckNotes.mockResolvedValue([]);
      const onClose = vi.fn();

      render(<DeckNotesPanel deckId="deck-1" onClose={onClose} />);

      await waitFor(() => {
        expect(screen.getByTitle('Close')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle('Close'));

      expect(onClose).toHaveBeenCalled();
    });

    it('should not show close button when onClose not provided', async () => {
      mockNotes.getDeckNotes.mockResolvedValue([]);

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.queryByTitle('Close')).not.toBeInTheDocument();
      });
    });
  });

  // ---------------------------------------------------------------------------
  // Sentry instrumentation — catch path coverage (Ray R-1: negative assertions
  // required to enforce the PII safeguard on newNoteContent and editContent)
  // ---------------------------------------------------------------------------
  describe('Sentry error reporting', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('calls reportError on load_notes failure, without newNoteContent or editContent in extra', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      mockNotes.getDeckNotes.mockRejectedValue(new Error('load failed'));

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalledOnce();
      });

      const callArgs = sentryCapture.mock.calls[0][1] as { tags?: Record<string, string>; extra?: Record<string, unknown> };
      expect(callArgs?.tags).toMatchObject({ component: 'DeckNotesPanel', action: 'load_notes' });
      // Ray R-1: explicit negative assertions — PII safeguard enforcement
      // extra is absent (no non-PII data available), or if present must not contain user-typed content
      expect(callArgs?.extra?.['newNoteContent']).toBeUndefined();
      expect(callArgs?.extra?.['editContent']).toBeUndefined();
    });

    it('still shows error UI when load_notes fails', async () => {
      mockNotes.getDeckNotes.mockRejectedValue(new Error('load failed'));

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('load failed')).toBeInTheDocument();
      });
    });

    it('calls reportError on add_note failure, without newNoteContent or editContent in extra', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      mockNotes.getDeckNotes.mockResolvedValue([]);
      mockNotes.createDeckNote.mockRejectedValue(new Error('add failed'));

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByText('+ Add Note')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('+ Add Note'));
      const textarea = screen.getByPlaceholderText('Write your note here...');
      fireEvent.change(textarea, { target: { value: 'my secret note content' } });
      fireEvent.click(screen.getByText('Save Note'));

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalled();
      });

      const addCall = sentryCapture.mock.calls.find(
        (c) => (c[1] as { tags?: Record<string, string> })?.tags?.action === 'add_note'
      );
      expect(addCall).toBeDefined();
      const addCallArgs = addCall![1] as { tags?: Record<string, string>; extra?: Record<string, unknown> };
      expect(addCallArgs?.tags).toMatchObject({ component: 'DeckNotesPanel', action: 'add_note' });
      // Ray R-1: negative assertions — user-typed note content must not appear in Sentry
      expect(addCallArgs?.extra?.['newNoteContent']).toBeUndefined();
      expect(addCallArgs?.extra?.['editContent']).toBeUndefined();
    });

    it('calls reportError on update_note failure, without newNoteContent or editContent in extra', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      const note = createMockNote({ id: 1, content: 'original content' });
      mockNotes.getDeckNotes.mockResolvedValue([note]);
      mockNotes.updateDeckNote.mockRejectedValue(new Error('update failed'));

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByTitle('Edit note')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle('Edit note'));
      const textarea = screen.getByDisplayValue('original content');
      fireEvent.change(textarea, { target: { value: 'edited secret content' } });
      fireEvent.click(screen.getByText('Save'));

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalled();
      });

      const updateCall = sentryCapture.mock.calls.find(
        (c) => (c[1] as { tags?: Record<string, string> })?.tags?.action === 'update_note'
      );
      expect(updateCall).toBeDefined();
      const updateCallArgs = updateCall![1] as { tags?: Record<string, string>; extra?: Record<string, unknown> };
      expect(updateCallArgs?.tags).toMatchObject({ component: 'DeckNotesPanel', action: 'update_note' });
      // Ray R-1: negative assertions — user-typed edit content must not appear in Sentry
      expect(updateCallArgs?.extra?.['newNoteContent']).toBeUndefined();
      expect(updateCallArgs?.extra?.['editContent']).toBeUndefined();
    });

    it('calls reportError on delete_note failure, without newNoteContent or editContent in extra', async () => {
      const sentryCapture = vi.mocked(Sentry.captureException);
      const note = createMockNote({ id: 1, content: 'to delete' });
      mockNotes.getDeckNotes.mockResolvedValue([note]);
      mockNotes.deleteDeckNote.mockRejectedValue(new Error('delete failed'));

      render(<DeckNotesPanel deckId="deck-1" />);

      await waitFor(() => {
        expect(screen.getByTitle('Delete note')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle('Delete note'));

      await waitFor(() => {
        expect(sentryCapture).toHaveBeenCalled();
      });

      const deleteCall = sentryCapture.mock.calls.find(
        (c) => (c[1] as { tags?: Record<string, string> })?.tags?.action === 'delete_note'
      );
      expect(deleteCall).toBeDefined();
      const deleteCallArgs = deleteCall![1] as { tags?: Record<string, string>; extra?: Record<string, unknown> };
      expect(deleteCallArgs?.tags).toMatchObject({ component: 'DeckNotesPanel', action: 'delete_note' });
      // Ray R-1: negative assertions — no user content in Sentry extra
      expect(deleteCallArgs?.extra?.['newNoteContent']).toBeUndefined();
      expect(deleteCallArgs?.extra?.['editContent']).toBeUndefined();
    });
  });
});
