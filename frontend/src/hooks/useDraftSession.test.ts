import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDraftSession } from './useDraftSession';
import type { DraftEvent } from './useDraftSession';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function packEvent(
  packNumber: number,
  selfPick: number,
  cards: number[]
): DraftEvent {
  return {
    type: 'draft.pack',
    payload: {
      draftPack: { PackCards: cards, SelfPick: selfPick },
      CourseName: 'PremierDraft_BLB',
    },
  };
}

function pickEvent(
  pickedCards: number[],
  packNumber = 0,
  pickNumber = 0
): DraftEvent {
  return {
    type: 'draft.pick',
    payload: { pickedCards, PackNumber: packNumber, PickNumber: pickNumber },
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useDraftSession', () => {
  describe('initial state', () => {
    it('starts idle', () => {
      const { result } = renderHook(() => useDraftSession());
      expect(result.current.state.sessionStatus).toBe('idle');
    });

    it('starts with empty currentPackCards', () => {
      const { result } = renderHook(() => useDraftSession());
      expect(result.current.state.currentPackCards).toEqual([]);
    });

    it('starts with empty pickedCards', () => {
      const { result } = renderHook(() => useDraftSession());
      expect(result.current.state.pickedCards).toEqual([]);
    });

    it('starts with packNumber 0', () => {
      const { result } = renderHook(() => useDraftSession());
      expect(result.current.state.packNumber).toBe(0);
    });

    it('starts with pickNumber 0', () => {
      const { result } = renderHook(() => useDraftSession());
      expect(result.current.state.pickNumber).toBe(0);
    });
  });

  describe('draft.started event', () => {
    it('sets sessionStatus to active', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch({ type: 'draft.started' }));
      expect(result.current.state.sessionStatus).toBe('active');
    });

    it('resets all fields to zero/empty', () => {
      const { result } = renderHook(() => useDraftSession());

      // Prime some state first
      act(() => result.current.dispatch({ type: 'draft.started' }));
      act(() => result.current.dispatch(packEvent(1, 1, [100, 200, 300])));
      act(() => result.current.dispatch(pickEvent([100], 0, 0)));

      // Now start a fresh draft
      act(() => result.current.dispatch({ type: 'draft.started' }));

      const s = result.current.state;
      expect(s.packNumber).toBe(0);
      expect(s.pickNumber).toBe(0);
      expect(s.currentPackCards).toEqual([]);
      expect(s.pickedCards).toEqual([]);
    });
  });

  describe('draft.pack event', () => {
    it('sets sessionStatus to active', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [10, 20])));
      expect(result.current.state.sessionStatus).toBe('active');
    });

    it('populates currentPackCards from nested draftPack.PackCards', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [111, 222, 333])));
      expect(result.current.state.currentPackCards).toEqual([111, 222, 333]);
    });

    it('sets pickNumber from draftPack.SelfPick (1-based)', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 3, [10, 20])));
      expect(result.current.state.pickNumber).toBe(3);
    });

    it('handles flat card_ids + pick_number payload shape', () => {
      const { result } = renderHook(() => useDraftSession());
      const evt: DraftEvent = {
        type: 'draft.pack',
        payload: { card_ids: [5, 6, 7], pick_number: 1, pack_number: 0 },
      };
      act(() => result.current.dispatch(evt));
      expect(result.current.state.currentPackCards).toEqual([5, 6, 7]);
      // pick_number 1 (0-based) → 2 (1-based)
      expect(result.current.state.pickNumber).toBe(2);
      // pack_number 0 (0-based) → 1 (1-based)
      expect(result.current.state.packNumber).toBe(1);
    });

    it('does not change pickedCards', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [10, 20])));
      act(() => result.current.dispatch(pickEvent([10])));
      const pickedBefore = result.current.state.pickedCards;

      act(() => result.current.dispatch(packEvent(1, 2, [20, 30, 40])));
      expect(result.current.state.pickedCards).toEqual(pickedBefore);
    });

    it('ignores event with missing payload gracefully', () => {
      const { result } = renderHook(() => useDraftSession());
      const before = result.current.state;
      act(() => result.current.dispatch({ type: 'draft.pack' }));
      expect(result.current.state).toEqual(before);
    });
  });

  describe('draft.pick event', () => {
    it('removes picked card from currentPackCards', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [100, 200, 300])));
      act(() => result.current.dispatch(pickEvent([200])));
      expect(result.current.state.currentPackCards).toEqual([100, 300]);
    });

    it('adds picked card to pickedCards', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [100, 200, 300])));
      act(() => result.current.dispatch(pickEvent([200])));
      expect(result.current.state.pickedCards).toContain(200);
    });

    it('accumulates multiple picks across packs', () => {
      const { result } = renderHook(() => useDraftSession());

      act(() => result.current.dispatch({ type: 'draft.started' }));
      act(() => result.current.dispatch(packEvent(1, 1, [1, 2, 3])));
      act(() => result.current.dispatch(pickEvent([1], 0, 0)));
      act(() => result.current.dispatch(packEvent(1, 2, [2, 3, 4])));
      act(() => result.current.dispatch(pickEvent([3], 0, 1)));

      expect(result.current.state.pickedCards).toEqual([1, 3]);
    });

    it('updates packNumber from 0-based PackNumber to 1-based', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(pickEvent([5], 1, 3)));
      // PackNumber=1 → packNumber=2
      expect(result.current.state.packNumber).toBe(2);
    });

    it('updates pickNumber from 0-based PickNumber to 1-based', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(pickEvent([5], 0, 4)));
      // PickNumber=4 → pickNumber=5
      expect(result.current.state.pickNumber).toBe(5);
    });

    it('supports flat card_id shape', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [10, 20, 30])));
      const evt: DraftEvent = {
        type: 'draft.pick',
        payload: { card_id: 10 },
      };
      act(() => result.current.dispatch(evt));
      expect(result.current.state.pickedCards).toContain(10);
      expect(result.current.state.currentPackCards).not.toContain(10);
    });

    it('keeps sessionStatus active', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [100])));
      act(() => result.current.dispatch(pickEvent([100])));
      expect(result.current.state.sessionStatus).toBe('active');
    });

    it('ignores event with missing payload gracefully', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [5])));
      const before = result.current.state;
      act(() => result.current.dispatch({ type: 'draft.pick' }));
      expect(result.current.state).toEqual(before);
    });
  });

  describe('draft.ended event', () => {
    it('sets sessionStatus to complete', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch({ type: 'draft.started' }));
      act(() => result.current.dispatch({ type: 'draft.ended' }));
      expect(result.current.state.sessionStatus).toBe('complete');
    });

    it('clears currentPackCards', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [10, 20])));
      act(() => result.current.dispatch({ type: 'draft.ended' }));
      expect(result.current.state.currentPackCards).toEqual([]);
    });

    it('preserves pickedCards after session ends', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch(packEvent(1, 1, [10, 20])));
      act(() => result.current.dispatch(pickEvent([10])));
      act(() => result.current.dispatch({ type: 'draft.ended' }));
      expect(result.current.state.pickedCards).toContain(10);
    });
  });

  describe('unknown event types', () => {
    it('ignores unknown event types without changing state', () => {
      const { result } = renderHook(() => useDraftSession());
      act(() => result.current.dispatch({ type: 'draft.started' }));
      const before = result.current.state;
      act(() => result.current.dispatch({ type: 'match.completed' }));
      expect(result.current.state).toEqual(before);
    });
  });

  describe('mid-session resume', () => {
    it('reconstructs state from replayed draft.pack events', () => {
      const { result } = renderHook(() => useDraftSession());

      // Simulate: SPA opened mid-draft, receives buffered events in order.
      act(() => result.current.dispatch({ type: 'draft.started' }));
      act(() => result.current.dispatch(packEvent(1, 1, [1, 2, 3, 4])));
      act(() => result.current.dispatch(pickEvent([1], 0, 0)));
      act(() => result.current.dispatch(packEvent(1, 2, [2, 3, 4, 5])));
      act(() => result.current.dispatch(pickEvent([3], 0, 1)));
      // Latest pack — this is where the player currently is.
      act(() => result.current.dispatch(packEvent(1, 3, [2, 4, 5, 6])));

      const s = result.current.state;
      expect(s.sessionStatus).toBe('active');
      expect(s.pickedCards).toEqual([1, 3]);
      expect(s.currentPackCards).toEqual([2, 4, 5, 6]);
      expect(s.pickNumber).toBe(3); // SelfPick from latest pack event
    });

    it('sets status active when first event is draft.pack (no draft.started)', () => {
      const { result } = renderHook(() => useDraftSession());
      // SPA opened while already in draft — draft.started not in buffer
      act(() => result.current.dispatch(packEvent(1, 5, [10, 20, 30])));
      expect(result.current.state.sessionStatus).toBe('active');
      expect(result.current.state.currentPackCards).toEqual([10, 20, 30]);
    });

    it('handles cross-pack resume: pack 2 state is correct', () => {
      const { result } = renderHook(() => useDraftSession());

      // Pack 1: 15 picks
      act(() => result.current.dispatch({ type: 'draft.started' }));
      for (let i = 1; i <= 15; i++) {
        act(() =>
          result.current.dispatch(packEvent(1, i, [i * 10, i * 10 + 1]))
        );
        act(() => result.current.dispatch(pickEvent([i * 10], 0, i - 1)));
      }

      // Pack 2 starts
      act(() => result.current.dispatch(packEvent(2, 1, [200, 201, 202])));
      act(() => result.current.dispatch(pickEvent([200], 1, 0)));

      const s = result.current.state;
      expect(s.pickedCards).toHaveLength(16);
      expect(s.packNumber).toBe(2); // PackNumber=1 (0-based) → 2
    });
  });
});
