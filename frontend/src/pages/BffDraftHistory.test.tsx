import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import BffDraftHistory from './BffDraftHistory';
import type { DraftHistoryResponse } from '@/services/api/bffDraftHistory';

// Mock the BFF adapter
vi.mock('@/services/api/bffDraftHistory', () => ({
  getDraftHistory: vi.fn(),
}));

// Import after mock so we get the vi.fn() version
import { getDraftHistory } from '@/services/api/bffDraftHistory';
const mockGetDraftHistory = vi.mocked(getDraftHistory);

function makeResponse(overrides: Partial<DraftHistoryResponse> = {}): DraftHistoryResponse {
  return {
    drafts: [],
    total: 0,
    limit: 20,
    offset: 0,
    ...overrides,
  };
}

describe('BffDraftHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading state', () => {
    it('renders loading spinner initially', async () => {
      let resolve: (v: DraftHistoryResponse) => void;
      mockGetDraftHistory.mockReturnValue(new Promise((r) => { resolve = r; }));

      render(<BffDraftHistory />);

      expect(screen.getByText('Loading drafts...')).toBeInTheDocument();

      resolve!(makeResponse());
      await waitFor(() => {
        expect(screen.queryByText('Loading drafts...')).not.toBeInTheDocument();
      });
    });
  });

  describe('Empty state', () => {
    it('renders empty state when total === 0', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({ total: 0, drafts: [] }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByTestId('draft-history-empty')).toBeInTheDocument();
      });
      expect(screen.getByText('No drafts yet')).toBeInTheDocument();
    });

    it('does not render table when total === 0', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({ total: 0, drafts: [] }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.queryByTestId('draft-history-table')).not.toBeInTheDocument();
      });
    });
  });

  describe('Table rendering', () => {
    it('renders table when data is returned', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({
        total: 1,
        drafts: [
          { id: 1, set_code: 'BLB', wins: 3, losses: 2, drafted_at: '2026-05-01T10:00:00Z' },
        ],
      }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByTestId('draft-history-table')).toBeInTheDocument();
      });
    });

    it('renders column headers: Date, Set, Wins, Losses', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({
        total: 1,
        drafts: [
          { id: 1, set_code: 'BLB', wins: 3, losses: 2, drafted_at: '2026-05-01T10:00:00Z' },
        ],
      }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByRole('table')).toBeInTheDocument();
      });

      const headers = screen.getAllByRole('columnheader');
      const headerTexts = headers.map((h) => h.textContent);
      expect(headerTexts).toContain('Date');
      expect(headerTexts).toContain('Set');
      expect(headerTexts).toContain('Wins');
      expect(headerTexts).toContain('Losses');
    });

    it('renders draft data in table rows', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({
        total: 1,
        drafts: [
          { id: 1, set_code: 'BLB', wins: 3, losses: 2, drafted_at: '2026-05-01T10:00:00Z' },
        ],
      }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByText('BLB')).toBeInTheDocument();
      });
      expect(screen.getByText('3')).toBeInTheDocument();
      expect(screen.getByText('2')).toBeInTheDocument();
    });

    it('renders multiple drafts', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({
        total: 2,
        drafts: [
          { id: 1, set_code: 'BLB', wins: 3, losses: 2, drafted_at: '2026-05-01T10:00:00Z' },
          { id: 2, set_code: 'DSK', wins: 7, losses: 0, drafted_at: '2026-04-15T08:00:00Z' },
        ],
      }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByText('BLB')).toBeInTheDocument();
        expect(screen.getByText('DSK')).toBeInTheDocument();
      });
    });
  });

  describe('Pagination', () => {
    it('Previous button is disabled on first page', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({
        total: 21,
        offset: 0,
        limit: 20,
        drafts: Array.from({ length: 20 }, (_, i) => ({
          id: i + 1,
          set_code: 'BLB',
          wins: 2,
          losses: 1,
          drafted_at: '2026-05-01T10:00:00Z',
        })),
      }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled();
      });
    });

    it('Next button is disabled when no more pages', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({
        total: 3,
        offset: 0,
        limit: 20,
        drafts: Array.from({ length: 3 }, (_, i) => ({
          id: i + 1,
          set_code: 'BLB',
          wins: 2,
          losses: 1,
          drafted_at: '2026-05-01T10:00:00Z',
        })),
      }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
      });
    });

    it('Next button is enabled when more pages exist', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse({
        total: 25,
        offset: 0,
        limit: 20,
        drafts: Array.from({ length: 20 }, (_, i) => ({
          id: i + 1,
          set_code: 'BLB',
          wins: 2,
          losses: 1,
          drafted_at: '2026-05-01T10:00:00Z',
        })),
      }));

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
      });
    });

    it('clicking Next fetches next page', async () => {
      const page1: DraftHistoryResponse = makeResponse({
        total: 25,
        offset: 0,
        limit: 20,
        drafts: Array.from({ length: 20 }, (_, i) => ({
          id: i + 1,
          set_code: 'BLB',
          wins: 2,
          losses: 1,
          drafted_at: '2026-05-01T10:00:00Z',
        })),
      });
      const page2: DraftHistoryResponse = makeResponse({
        total: 25,
        offset: 20,
        limit: 20,
        drafts: [
          { id: 21, set_code: 'FDN', wins: 5, losses: 3, drafted_at: '2026-04-01T10:00:00Z' },
        ],
      });

      mockGetDraftHistory
        .mockResolvedValueOnce(page1)
        .mockResolvedValueOnce(page2);

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Next' }));

      await waitFor(() => {
        expect(screen.getByText('FDN')).toBeInTheDocument();
      });
      expect(mockGetDraftHistory).toHaveBeenCalledWith('clerk-test-token-stub', { limit: 20, offset: 20 });
    });
  });

  describe('Page title', () => {
    it('renders Draft History heading', async () => {
      mockGetDraftHistory.mockResolvedValue(makeResponse());

      render(<BffDraftHistory />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Draft History');
      });
    });
  });
});
