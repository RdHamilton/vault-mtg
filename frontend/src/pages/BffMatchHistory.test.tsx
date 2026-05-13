import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import BffMatchHistory from './BffMatchHistory';
import type { MatchHistoryResponse } from '@/services/api/bffMatchHistory';

// Mock the BFF adapter
vi.mock('@/services/api/bffMatchHistory', () => ({
  getMatchHistory: vi.fn(),
}));

// Track registered SSE callbacks so tests can fire them manually.
let statsUpdatedCallback: (() => void) | null = null;
vi.mock('@/services/websocketClient', () => ({
  EventsOn: vi.fn((event: string, cb: () => void) => {
    if (event === 'stats:updated') {
      statsUpdatedCallback = cb;
    }
    return () => { statsUpdatedCallback = null; };
  }),
}));

// Import after mock so we get the vi.fn() version
import { getMatchHistory } from '@/services/api/bffMatchHistory';
const mockGetMatchHistory = vi.mocked(getMatchHistory);

function makeResponse(overrides: Partial<MatchHistoryResponse> = {}): MatchHistoryResponse {
  return {
    matches: [],
    total: 0,
    limit: 20,
    offset: 0,
    ...overrides,
  };
}

describe('BffMatchHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading state', () => {
    it('renders loading spinner initially', async () => {
      let resolve: (v: MatchHistoryResponse) => void;
      mockGetMatchHistory.mockReturnValue(new Promise((r) => { resolve = r; }));

      render(<BffMatchHistory />);

      expect(screen.getByText('Loading matches...')).toBeInTheDocument();

      resolve!(makeResponse());
      await waitFor(() => {
        expect(screen.queryByText('Loading matches...')).not.toBeInTheDocument();
      });
    });
  });

  describe('Empty state', () => {
    it('renders empty state when total === 0', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({ total: 0, matches: [] }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByTestId('match-history-empty')).toBeInTheDocument();
      });
      expect(screen.getByText('No matches yet')).toBeInTheDocument();
    });

    it('does not render table when total === 0', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({ total: 0, matches: [] }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.queryByTestId('match-history-table')).not.toBeInTheDocument();
      });
    });
  });

  describe('Table rendering', () => {
    it('renders table when data is returned', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 1,
        matches: [
          { id: 1, opponent_deck: 'Azorius Control', result: 'win', format: 'Standard', played_at: '2026-05-01T14:30:00Z' },
        ],
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByTestId('match-history-table')).toBeInTheDocument();
      });
    });

    it('renders column headers: Date, Format, Opponent Deck, Result', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 1,
        matches: [
          { id: 1, opponent_deck: 'Azorius Control', result: 'win', format: 'Standard', played_at: '2026-05-01T14:30:00Z' },
        ],
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByRole('table')).toBeInTheDocument();
      });

      const headers = screen.getAllByRole('columnheader');
      const headerTexts = headers.map((h) => h.textContent);
      expect(headerTexts).toContain('Date');
      expect(headerTexts).toContain('Format');
      expect(headerTexts).toContain('Opponent Deck');
      expect(headerTexts).toContain('Result');
    });

    it('renders match data in table rows', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 1,
        matches: [
          { id: 1, opponent_deck: 'Azorius Control', result: 'win', format: 'Standard', played_at: '2026-05-01T14:30:00Z' },
        ],
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByText('Azorius Control')).toBeInTheDocument();
      });
      expect(screen.getByText('Standard')).toBeInTheDocument();
      expect(screen.getByText('WIN')).toBeInTheDocument();
    });

    it('renders loss badge correctly', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 1,
        matches: [
          { id: 2, opponent_deck: 'Rakdos Midrange', result: 'loss', format: 'Historic', played_at: '2026-05-02T10:00:00Z' },
        ],
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByText('LOSS')).toBeInTheDocument();
      });
    });

    it('shows dash for missing opponent_deck', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 1,
        matches: [
          { id: 3, opponent_deck: '', result: 'win', format: 'Standard', played_at: '2026-05-01T14:30:00Z' },
        ],
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByText('—')).toBeInTheDocument();
      });
    });
  });

  describe('Pagination', () => {
    it('Previous button is disabled on first page', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 21,
        offset: 0,
        limit: 20,
        matches: Array.from({ length: 20 }, (_, i) => ({
          id: i + 1,
          opponent_deck: `Deck ${i}`,
          result: 'win',
          format: 'Standard',
          played_at: '2026-05-01T14:30:00Z',
        })),
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled();
      });
    });

    it('Next button is disabled when no more pages', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 5,
        offset: 0,
        limit: 20,
        matches: Array.from({ length: 5 }, (_, i) => ({
          id: i + 1,
          opponent_deck: `Deck ${i}`,
          result: 'win',
          format: 'Standard',
          played_at: '2026-05-01T14:30:00Z',
        })),
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
      });
    });

    it('Next button is enabled when more pages exist', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 25,
        offset: 0,
        limit: 20,
        matches: Array.from({ length: 20 }, (_, i) => ({
          id: i + 1,
          opponent_deck: `Deck ${i}`,
          result: 'win',
          format: 'Standard',
          played_at: '2026-05-01T14:30:00Z',
        })),
      }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
      });
    });

    it('clicking Next fetches the next page', async () => {
      const page1: MatchHistoryResponse = makeResponse({
        total: 25,
        offset: 0,
        limit: 20,
        matches: Array.from({ length: 20 }, (_, i) => ({
          id: i + 1,
          opponent_deck: `Deck ${i}`,
          result: 'win',
          format: 'Standard',
          played_at: '2026-05-01T14:30:00Z',
        })),
      });
      const page2: MatchHistoryResponse = makeResponse({
        total: 25,
        offset: 20,
        limit: 20,
        matches: [
          { id: 21, opponent_deck: 'Page2Deck', result: 'loss', format: 'Historic', played_at: '2026-05-02T10:00:00Z' },
        ],
      });

      mockGetMatchHistory
        .mockResolvedValueOnce(page1)
        .mockResolvedValueOnce(page2);

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Next' }));

      await waitFor(() => {
        expect(screen.getByText('Page2Deck')).toBeInTheDocument();
      });
      expect(mockGetMatchHistory).toHaveBeenCalledWith('clerk-test-token-stub', { limit: 20, offset: 20 });
    });

    it('Previous button enabled on page 2', async () => {
      const page1: MatchHistoryResponse = makeResponse({
        total: 25,
        offset: 0,
        limit: 20,
        matches: Array.from({ length: 20 }, (_, i) => ({
          id: i + 1,
          opponent_deck: `Deck ${i}`,
          result: 'win',
          format: 'Standard',
          played_at: '2026-05-01T14:30:00Z',
        })),
      });
      const page2: MatchHistoryResponse = makeResponse({
        total: 25,
        offset: 20,
        limit: 20,
        matches: [{ id: 21, opponent_deck: 'Page2Deck', result: 'loss', format: 'Historic', played_at: '2026-05-02T10:00:00Z' }],
      });

      mockGetMatchHistory
        .mockResolvedValueOnce(page1)
        .mockResolvedValueOnce(page2);

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Next' }));

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Previous' })).toBeEnabled();
      });
    });
  });

  describe('Page title', () => {
    it('renders Match History heading', async () => {
      mockGetMatchHistory.mockResolvedValue(makeResponse());

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Match History');
      });
    });
  });

  describe('SSE refresh on stats:updated', () => {
    it('re-fetches matches when stats:updated fires', async () => {
      statsUpdatedCallback = null;
      mockGetMatchHistory.mockResolvedValue(makeResponse({ total: 0 }));

      render(<BffMatchHistory />);

      await waitFor(() => {
        expect(mockGetMatchHistory).toHaveBeenCalledTimes(1);
      });

      expect(statsUpdatedCallback).not.toBeNull();

      mockGetMatchHistory.mockResolvedValue(makeResponse({
        total: 1,
        matches: [{ id: 99, opponent_deck: 'New Match', result: 'win', format: 'Standard', played_at: '2026-05-13T12:00:00Z' }],
      }));

      await act(async () => {
        statsUpdatedCallback!();
      });

      await waitFor(() => {
        expect(mockGetMatchHistory).toHaveBeenCalledTimes(2);
        expect(screen.getByText('New Match')).toBeInTheDocument();
      });
    });
  });
});
