/**
 * useDraftEventStream — SSE consumer hook for live draft events.
 *
 * Opens an EventSource connection to the BFF `/api/v1/events` endpoint
 * (secured via Clerk `__session` cookie, set automatically by the Clerk JS
 * SDK) and filters for events whose `type` field starts with `draft.`.
 *
 * Features:
 * - Reconnects with exponential backoff (100ms base, 30s cap) on error.
 * - Exposes `latestEvent` (last parsed draft event or null) and `status`.
 * - Cleans up the EventSource on unmount — no memory leaks.
 */

import { useEffect, useRef, useState } from 'react';

/** Status of the underlying SSE connection. */
export type DraftEventStreamStatus = 'connecting' | 'open' | 'closed' | 'error';

/** Parsed wire format of a DaemonEvent sent by the BFF broker. */
export interface DaemonEvent {
  type: string;
  account_id: string;
  event_id: string;
  session_id: string;
  sequence: number;
  occurred_at: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  payload: Record<string, any> | null;
}

export interface UseDraftEventStreamReturn {
  /** Latest received draft event (type prefix `draft.`), or null. */
  latestEvent: DaemonEvent | null;
  /** Current SSE connection status. */
  status: DraftEventStreamStatus;
}

const BFF_BASE_URL =
  (typeof import.meta !== 'undefined' && import.meta.env?.VITE_BFF_URL) ||
  'http://localhost:8080/api/v1';

const SSE_URL = `${BFF_BASE_URL}/events`;

/** Prefix for draft-related event types. */
const DRAFT_EVENT_PREFIX = 'draft.';

/** Backoff config (ms). */
const BACKOFF_BASE_MS = 100;
const BACKOFF_MAX_MS = 30_000;

function computeBackoff(attempt: number): number {
  const exponential = BACKOFF_BASE_MS * Math.pow(2, attempt);
  return Math.min(exponential, BACKOFF_MAX_MS);
}

export function useDraftEventStream(): UseDraftEventStreamReturn {
  const [latestEvent, setLatestEvent] = useState<DaemonEvent | null>(null);
  const [status, setStatus] = useState<DraftEventStreamStatus>('connecting');

  // All mutable state lives in refs so callbacks captured in EventSource
  // handlers always see the current value without triggering re-renders.
  const sourceRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const attemptRef = useRef<number>(0);
  const unmountedRef = useRef<boolean>(false);

  // setLatestEvent / setStatus are stable, so we expose them through refs to
  // keep the effect-internal `connect` function free of hook dependencies.
  const setLatestEventRef = useRef(setLatestEvent);
  const setStatusRef = useRef(setStatus);

  useEffect(() => {
    unmountedRef.current = false;

    // Capture stable setState dispatchers in locals so the cleanup function
    // does not trigger the react-hooks/exhaustive-deps ref-in-cleanup warning.
    const setLatestEventLocal = setLatestEventRef.current;
    const setStatusLocal = setStatusRef.current;

    function connect() {
      if (unmountedRef.current) return;

      setStatusLocal('connecting');

      const source = new EventSource(SSE_URL, { withCredentials: true });
      sourceRef.current = source;

      source.onopen = () => {
        if (unmountedRef.current) {
          source.close();
          return;
        }
        attemptRef.current = 0;
        setStatusLocal('open');
      };

      const handleDraftMessage = (e: MessageEvent) => {
        if (unmountedRef.current) return;
        try {
          const ev = JSON.parse(e.data as string) as DaemonEvent;
          if (ev.type?.startsWith(DRAFT_EVENT_PREFIX)) {
            setLatestEventLocal(ev);
          }
        } catch {
          // Malformed JSON — ignore silently.
        }
      };

      // Unnamed data frames arrive via onmessage.
      source.onmessage = handleDraftMessage;

      // Named event frames (e.g. `event: draft.pack`) are dispatched as named
      // events on the EventSource and do NOT fire onmessage.
      source.addEventListener('draft.started', handleDraftMessage);
      source.addEventListener('draft.pack', handleDraftMessage);
      source.addEventListener('draft.ended', handleDraftMessage);

      source.onerror = () => {
        if (unmountedRef.current) {
          source.close();
          return;
        }

        setStatusLocal('error');
        source.close();
        sourceRef.current = null;

        const delay = computeBackoff(attemptRef.current);
        attemptRef.current += 1;

        reconnectTimerRef.current = setTimeout(() => {
          if (!unmountedRef.current) {
            connect();
          }
        }, delay);
      };
    }

    connect();

    return () => {
      unmountedRef.current = true;

      if (reconnectTimerRef.current !== null) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }

      if (sourceRef.current) {
        sourceRef.current.close();
        sourceRef.current = null;
      }

      setStatusLocal('closed');
    };
  }, []);

  return { latestEvent, status };
}
