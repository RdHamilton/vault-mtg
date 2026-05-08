/**
 * DraftLive component tests — ticket #1390
 *
 * Mocks:
 *   - useDraftEventStream  → controlled via mockStream
 *   - useDraftSession      → controlled via mockSession
 *   - getDraftRatings      → controlled via mockGetDraftRatings
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import DraftLive from './DraftLive';
import type { DraftSessionState, UseDraftSessionReturn } from '@/hooks/useDraftSession';

// ---------------------------------------------------------------------------
// Mock hooks and adapters
// ---------------------------------------------------------------------------

const mockStreamReturn = {
  latestEvent: null as import('@/hooks/useDraftEventStream').DaemonEvent | null,
  status: 'open' as import('@/hooks/useDraftEventStream').DraftEventStreamStatus,
};

const mockSessionReturn: UseDraftSessionReturn = {
  state: {
    sessionStatus: 'idle',
    packNumber: 0,
    pickNumber: 0,
    currentPackCards: [],
    pickedCards: [],
  },
  dispatch: vi.fn(),
};

vi.mock('@/hooks', () => ({
  useDraftEventStream: vi.fn(() => mockStreamReturn),
  useDraftSession: vi.fn(() => mockSessionReturn),
}));

vi.mock('@/services/api/bffDraftRatings', () => ({
  getDraftRatings: vi.fn(),
}));

// Clerk useAuth mock
vi.mock('@clerk/react', () => ({
  useAuth: vi.fn(() => ({ getToken: vi.fn().mockResolvedValue('test-token') })),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

import { useDraftEventStream, useDraftSession } from '@/hooks';
import { getDraftRatings } from '@/services/api/bffDraftRatings';

const mockUseDraftEventStream = vi.mocked(useDraftEventStream);
const mockUseDraftSession = vi.mocked(useDraftSession);
const mockGetDraftRatings = vi.mocked(getDraftRatings);

function buildSession(overrides: Partial<DraftSessionState> = {}): DraftSessionState {
  return {
    sessionStatus: 'idle',
    packNumber: 0,
    pickNumber: 0,
    currentPackCards: [],
    pickedCards: [],
    ...overrides,
  };
}

function buildRatingsResult(cards: { arena_id: number; name: string; gihwr?: number }[]) {
  return {
    data: {
      set_code: 'ONE',
      draft_format: 'PremierDraft',
      cached_at: '2026-01-01T00:00:00Z',
      card_ratings: cards.map((c) => ({
        arena_id: c.arena_id,
        name: c.name,
        gihwr: c.gihwr,
      })),
      color_ratings: [],
    },
    cacheDegraded: false,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('DraftLive', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetDraftRatings.mockResolvedValue(buildRatingsResult([]));
  });

  // ── Empty / idle state ────────────────────────────────────────────────────

  describe('idle state (no active draft)', () => {
    it('renders empty state when session is idle', () => {
      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'idle' }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      expect(screen.getByTestId('empty-state')).toBeInTheDocument();
      expect(screen.getByText('No active draft')).toBeInTheDocument();
      expect(
        screen.getByText('Start a draft in Arena to see your live pick recommendations')
      ).toBeInTheDocument();
    });

    it('still shows container and stream status in idle state', () => {
      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'connecting' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'idle' }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      expect(screen.getByTestId('draft-live-container')).toBeInTheDocument();
      expect(screen.getByTestId('stream-status')).toHaveTextContent('connecting');
    });
  });

  // ── Complete state ────────────────────────────────────────────────────────

  describe('complete state', () => {
    it('shows draft complete empty state', () => {
      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'complete' }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      expect(screen.getByText('Draft complete')).toBeInTheDocument();
    });
  });

  // ── Active state ──────────────────────────────────────────────────────────

  describe('active state — pack display', () => {
    it('renders pack section with card names and grades', async () => {
      mockGetDraftRatings.mockResolvedValue(
        buildRatingsResult([
          { arena_id: 101, name: 'Elesh Norn', gihwr: 67 },
          { arena_id: 102, name: 'Plains', gihwr: 50 },
        ])
      );

      const dispatchFn = vi.fn();
      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({
          sessionStatus: 'active',
          packNumber: 1,
          pickNumber: 3,
          currentPackCards: [101, 102],
        }),
        dispatch: dispatchFn,
      });

      render(<DraftLive />);

      // Both cards appear after ratings load.
      await waitFor(() => {
        expect(screen.getByTestId('pack-card-101')).toBeInTheDocument();
        expect(screen.getByTestId('pack-card-102')).toBeInTheDocument();
      });
    });

    it('highlights the top pick card', async () => {
      mockGetDraftRatings.mockResolvedValue(
        buildRatingsResult([
          { arena_id: 201, name: 'Windfall', gihwr: 68 },
          { arena_id: 202, name: 'Island', gihwr: 49 },
        ])
      );

      const startedEvent: import('@/hooks/useDraftEventStream').DaemonEvent = {
        type: 'draft.started',
        account_id: 'acc1',
        event_id: 'e0',
        session_id: 's1',
        sequence: 0,
        occurred_at: '2026-05-08T00:00:00Z',
        payload: { set_code: 'ONE', draft_type: 'PremierDraft' },
      };

      mockUseDraftEventStream.mockReturnValue({ latestEvent: startedEvent, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({
          sessionStatus: 'active',
          packNumber: 1,
          pickNumber: 1,
          currentPackCards: [201, 202],
        }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      await waitFor(() => {
        expect(screen.getByTestId('top-pick-badge')).toBeInTheDocument();
      });

      // The card with higher GIHWR is marked as top pick.
      const topCard = screen.getByTestId('pack-card-201');
      expect(topCard).toHaveAttribute('data-top-pick', 'true');

      // The other card is NOT marked as top pick.
      const otherCard = screen.getByTestId('pack-card-202');
      expect(otherCard).not.toHaveAttribute('data-top-pick');
    });

    it('shows pack/pick progress metadata', async () => {
      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({
          sessionStatus: 'active',
          packNumber: 2,
          pickNumber: 5,
          currentPackCards: [],
        }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      expect(screen.getByTestId('draft-live-progress')).toHaveTextContent('Pack 2 · Pick 5');
    });
  });

  // ── Pick history ─────────────────────────────────────────────────────────

  describe('pick history', () => {
    it('shows pick history section with picked cards', async () => {
      mockGetDraftRatings.mockResolvedValue(
        buildRatingsResult([
          { arena_id: 301, name: 'Black Lotus', gihwr: 72 },
          { arena_id: 302, name: 'Sol Ring', gihwr: 65 },
        ])
      );

      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({
          sessionStatus: 'active',
          packNumber: 1,
          pickNumber: 3,
          currentPackCards: [],
          pickedCards: [301, 302],
        }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      await waitFor(() => {
        expect(screen.getByTestId('picked-card-301')).toBeInTheDocument();
        expect(screen.getByTestId('picked-card-302')).toBeInTheDocument();
      });
    });

    it('shows "No picks yet" when pickedCards is empty', () => {
      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({
          sessionStatus: 'active',
          currentPackCards: [],
          pickedCards: [],
        }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      expect(screen.getByText('No picks yet')).toBeInTheDocument();
    });
  });

  // ── SSE dispatch ─────────────────────────────────────────────────────────

  describe('SSE event dispatch', () => {
    it('dispatches latestEvent to session state machine', async () => {
      const dispatchFn = vi.fn();
      const fakeEvent: import('@/hooks/useDraftEventStream').DaemonEvent = {
        type: 'draft.pack',
        account_id: 'acc1',
        event_id: 'evt1',
        session_id: 'sess1',
        sequence: 1,
        occurred_at: '2026-05-08T00:00:00Z',
        payload: { card_ids: [101, 102], pack_number: 0, pick_number: 0 },
      };

      mockUseDraftEventStream.mockReturnValue({ latestEvent: fakeEvent, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'active', currentPackCards: [101, 102] }),
        dispatch: dispatchFn,
      });

      render(<DraftLive />);

      await waitFor(() => {
        expect(dispatchFn).toHaveBeenCalledWith(
          expect.objectContaining({ type: 'draft.pack' })
        );
      });
    });
  });

  // ── Ratings fetch ─────────────────────────────────────────────────────────

  describe('ratings loading', () => {
    it('fetches ratings when set code and format are available from draft.started event', async () => {
      const startedEvent: import('@/hooks/useDraftEventStream').DaemonEvent = {
        type: 'draft.started',
        account_id: 'acc1',
        event_id: 'evt0',
        session_id: 'sess1',
        sequence: 0,
        occurred_at: '2026-05-08T00:00:00Z',
        payload: { set_code: 'ONE', draft_type: 'PremierDraft' },
      };

      mockUseDraftEventStream.mockReturnValue({ latestEvent: startedEvent, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'active' }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      await waitFor(() => {
        expect(mockGetDraftRatings).toHaveBeenCalledWith('ONE', 'Premier Draft');
      });
    });

    it('shows error message when ratings fetch fails', async () => {
      mockGetDraftRatings.mockRejectedValue(new Error('Network failure'));

      const startedEvent: import('@/hooks/useDraftEventStream').DaemonEvent = {
        type: 'draft.started',
        account_id: 'acc1',
        event_id: 'evt0',
        session_id: 'sess1',
        sequence: 0,
        occurred_at: '2026-05-08T00:00:00Z',
        payload: { set_code: 'BLB', draft_type: 'QuickDraft' },
      };

      mockUseDraftEventStream.mockReturnValue({ latestEvent: startedEvent, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'active' }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      await waitFor(() => {
        expect(screen.getByTestId('ratings-error')).toHaveTextContent('Network failure');
      });
    });

    it('shows set name and format label in meta bar', async () => {
      const startedEvent: import('@/hooks/useDraftEventStream').DaemonEvent = {
        type: 'draft.started',
        account_id: 'acc1',
        event_id: 'evt0',
        session_id: 'sess1',
        sequence: 0,
        occurred_at: '2026-05-08T00:00:00Z',
        payload: { set_code: 'MKM', draft_type: 'QuickDraft' },
      };

      mockUseDraftEventStream.mockReturnValue({ latestEvent: startedEvent, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'active' }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      await waitFor(() => {
        expect(screen.getByTestId('draft-live-set')).toHaveTextContent('MKM');
        expect(screen.getByTestId('draft-live-format')).toHaveTextContent('Quick Draft');
      });
    });
  });

  // ── Grade rendering ────────────────────────────────────────────────────────

  describe('grade rendering', () => {
    it('shows A+ grade for card with gihwr >= 65', async () => {
      mockGetDraftRatings.mockResolvedValue(
        buildRatingsResult([{ arena_id: 401, name: 'Mythic Bomb', gihwr: 66 }])
      );

      const startedEvent: import('@/hooks/useDraftEventStream').DaemonEvent = {
        type: 'draft.started',
        account_id: 'acc1',
        event_id: 'e0',
        session_id: 's1',
        sequence: 0,
        occurred_at: '2026-05-08T00:00:00Z',
        payload: { set_code: 'ONE', draft_type: 'PremierDraft' },
      };

      mockUseDraftEventStream.mockReturnValue({ latestEvent: startedEvent, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({
          sessionStatus: 'active',
          currentPackCards: [401],
        }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      await waitFor(() => {
        expect(screen.getByTestId('card-grade-401')).toHaveTextContent('A+');
      });
    });

    it('shows — grade for card with no ratings data', async () => {
      mockGetDraftRatings.mockResolvedValue(buildRatingsResult([]));

      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: 'open' });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({
          sessionStatus: 'active',
          currentPackCards: [999],
        }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      await act(async () => {});

      expect(screen.getByTestId('card-grade-999')).toHaveTextContent('—');
    });
  });

  // ── Stream status ─────────────────────────────────────────────────────────

  describe('stream status display', () => {
    it.each([
      ['open', 'open'],
      ['connecting', 'connecting'],
      ['error', 'error'],
      ['closed', 'closed'],
    ] as const)('shows stream status "%s"', (inputStatus, expectedText) => {
      mockUseDraftEventStream.mockReturnValue({ latestEvent: null, status: inputStatus });
      mockUseDraftSession.mockReturnValue({
        state: buildSession({ sessionStatus: 'active', currentPackCards: [] }),
        dispatch: vi.fn(),
      });

      render(<DraftLive />);

      expect(screen.getByTestId('stream-status')).toHaveTextContent(expectedText);
    });
  });
});
