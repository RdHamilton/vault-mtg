import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';

// ---------------------------------------------------------------------------
// Minimal EventSource mock
// ---------------------------------------------------------------------------

type EventCallback = (e: MessageEvent) => void;
type ErrorCallback = (e: Event) => void;

interface MockEventSourceInstance {
  url: string;
  withCredentials: boolean;
  readyState: number;
  onopen: (() => void) | null;
  onmessage: EventCallback | null;
  onerror: ErrorCallback | null;
  close: ReturnType<typeof vi.fn>;
  addEventListener: ReturnType<typeof vi.fn>;
  // Test helpers
  _triggerOpen: () => void;
  _triggerMessage: (data: string) => void;
  _triggerNamedEvent: (type: string, data: string) => void;
  _triggerError: () => void;
  _namedListeners: Map<string, EventCallback[]>;
}

let instances: MockEventSourceInstance[] = [];

const MockEventSource = vi.fn((url: string, opts?: { withCredentials?: boolean }) => {
  const namedListeners = new Map<string, EventCallback[]>();

  const instance: MockEventSourceInstance = {
    url,
    withCredentials: opts?.withCredentials ?? false,
    readyState: 0, // CONNECTING
    onopen: null,
    onmessage: null,
    onerror: null,
    close: vi.fn(() => {
      instance.readyState = 2; // CLOSED
    }),
    addEventListener: vi.fn((type: string, handler: EventCallback) => {
      const list = namedListeners.get(type) ?? [];
      list.push(handler);
      namedListeners.set(type, list);
    }),
    _namedListeners: namedListeners,
    _triggerOpen() {
      instance.readyState = 1; // OPEN
      instance.onopen?.();
    },
    _triggerMessage(data: string) {
      instance.onmessage?.({ data } as MessageEvent);
    },
    _triggerNamedEvent(type: string, data: string) {
      const handlers = namedListeners.get(type) ?? [];
      handlers.forEach((h) => h({ data } as MessageEvent));
    },
    _triggerError() {
      instance.onerror?.({} as Event);
    },
  };

  instances.push(instance);
  return instance;
});

vi.stubGlobal('EventSource', MockEventSource);

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeDraftEvent(type: string, overrides: Partial<Record<string, unknown>> = {}): string {
  return JSON.stringify({
    type,
    account_id: 'acc_1',
    event_id: 'evt_1',
    session_id: 'sess_1',
    sequence: 1,
    occurred_at: '2026-05-08T00:00:00Z',
    payload: { draft_id: 'draft_abc' },
    ...overrides,
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useDraftEventStream', () => {
  beforeEach(() => {
    instances = [];
    MockEventSource.mockClear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.resetModules();
  });

  it('starts with status "connecting"', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    expect(result.current.status).toBe('connecting');
    expect(result.current.latestEvent).toBeNull();
  });

  it('transitions to "open" on EventSource open', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    expect(instances).toHaveLength(1);

    act(() => {
      instances[0]._triggerOpen();
    });

    expect(result.current.status).toBe('open');
  });

  it('opens EventSource with withCredentials: true', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    renderHook(() => useDraftEventStream());

    expect(instances).toHaveLength(1);
    expect(instances[0].withCredentials).toBe(true);
  });

  it('updates latestEvent when a draft.started message arrives', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
      instances[0]._triggerMessage(makeDraftEvent('draft.started'));
    });

    expect(result.current.latestEvent?.type).toBe('draft.started');
  });

  it('updates latestEvent when a draft.pack message arrives', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
      instances[0]._triggerMessage(makeDraftEvent('draft.pack'));
    });

    expect(result.current.latestEvent?.type).toBe('draft.pack');
  });

  it('updates latestEvent when a draft.ended message arrives', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
      instances[0]._triggerMessage(makeDraftEvent('draft.ended'));
    });

    expect(result.current.latestEvent?.type).toBe('draft.ended');
  });

  it('ignores non-draft events', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
      instances[0]._triggerMessage(makeDraftEvent('match.completed'));
    });

    expect(result.current.latestEvent).toBeNull();
  });

  it('handles named event frames (draft.pack as addEventListener)', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
      instances[0]._triggerNamedEvent('draft.pack', makeDraftEvent('draft.pack'));
    });

    expect(result.current.latestEvent?.type).toBe('draft.pack');
  });

  it('ignores malformed JSON without throwing', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    expect(() => {
      act(() => {
        instances[0]._triggerOpen();
        instances[0]._triggerMessage('not-valid-json');
      });
    }).not.toThrow();

    expect(result.current.latestEvent).toBeNull();
  });

  it('sets status to "error" on EventSource error', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { result } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
    });

    act(() => {
      instances[0]._triggerError();
    });

    expect(result.current.status).toBe('error');
  });

  it('closes the first EventSource on error', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
      instances[0]._triggerError();
    });

    expect(instances[0].close).toHaveBeenCalled();
  });

  it('reconnects after exponential backoff on error', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    renderHook(() => useDraftEventStream());

    // Trigger an error — should schedule a reconnect
    act(() => {
      instances[0]._triggerError();
    });

    expect(instances).toHaveLength(1); // not reconnected yet

    // Advance past the first backoff (100ms base)
    act(() => {
      vi.advanceTimersByTime(150);
    });

    expect(instances).toHaveLength(2); // new EventSource created
  });

  it('increases backoff delay on successive errors', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    renderHook(() => useDraftEventStream());

    // First error — 100ms backoff
    act(() => {
      instances[0]._triggerError();
    });
    act(() => {
      vi.advanceTimersByTime(150);
    });
    expect(instances).toHaveLength(2);

    // Second error — 200ms backoff
    act(() => {
      instances[1]._triggerError();
    });
    act(() => {
      vi.advanceTimersByTime(150);
    });
    // 150ms < 200ms — not reconnected yet
    expect(instances).toHaveLength(2);

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(instances).toHaveLength(3);
  });

  it('resets backoff attempt counter on successful open', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    renderHook(() => useDraftEventStream());

    // Fail once and reconnect
    act(() => {
      instances[0]._triggerError();
    });
    act(() => {
      vi.advanceTimersByTime(150);
    });
    expect(instances).toHaveLength(2);

    // Succeed on second connection
    act(() => {
      instances[1]._triggerOpen();
    });

    // Fail again — backoff should be reset to 100ms (attempt 0)
    act(() => {
      instances[1]._triggerError();
    });
    act(() => {
      vi.advanceTimersByTime(150);
    });
    expect(instances).toHaveLength(3); // reconnected at 100ms
  });

  it('cleans up EventSource on unmount', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { unmount } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
    });

    unmount();

    expect(instances[0].close).toHaveBeenCalled();
  });

  it('cancels pending reconnect timer on unmount', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { unmount } = renderHook(() => useDraftEventStream());

    // Trigger error to schedule reconnect
    act(() => {
      instances[0]._triggerError();
    });

    // Unmount before the timer fires
    unmount();

    // Advance time — no new EventSource should be created
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(instances).toHaveLength(1);
  });

  it('closes the EventSource on unmount (verifies no-leak cleanup)', async () => {
    // React setState calls after unmount do not propagate to result.current —
    // instead we verify the underlying EventSource was closed, which is the
    // leak-free behaviour we care about.
    const { useDraftEventStream } = await import('./useDraftEventStream');
    const { unmount } = renderHook(() => useDraftEventStream());

    act(() => {
      instances[0]._triggerOpen();
    });

    expect(instances[0].close).not.toHaveBeenCalled();

    act(() => {
      unmount();
    });

    expect(instances[0].close).toHaveBeenCalledOnce();
  });

  it('caps backoff at 30 seconds', async () => {
    const { useDraftEventStream } = await import('./useDraftEventStream');
    renderHook(() => useDraftEventStream());

    // Trigger many errors to push past the cap (2^n * 100ms > 30000ms at n=9)
    for (let i = 0; i < 10; i++) {
      const idx = instances.length - 1;
      act(() => {
        instances[idx]._triggerError();
      });
      // Advance max delay to ensure reconnect always happens
      act(() => {
        vi.advanceTimersByTime(35_000);
      });
    }

    // After 10 cycles we should have 11 EventSource instances (1 original + 10 reconnects)
    expect(instances).toHaveLength(11);
  });
});
