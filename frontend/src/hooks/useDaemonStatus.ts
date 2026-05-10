/**
 * useDaemonStatus
 *
 * Lightweight hook that reads daemon connectivity from the BFF health endpoint.
 * Used by daemon-dependent pages (Match History, Collection, Decks) to decide
 * whether to show content or a "daemon not connected" empty state.
 *
 * Returns:
 *   - daemonConnected: true when daemon is confirmed connected
 *   - daemonChecked:   true once at least one health check has completed
 *
 * The hook reads from the BFF (/api/v1/health/daemon) via getDaemonHealth,
 * which requires a Clerk session token.
 */

import { useState, useEffect, useCallback } from 'react';
import { useAuth } from '@clerk/react';
import { getDaemonHealth } from '@/services/api/bffHealth';

export type DaemonStatusResult = {
  /** True when the daemon is confirmed connected. */
  daemonConnected: boolean;
  /** True once at least one health check has finished (success or failure). */
  daemonChecked: boolean;
};

const POLL_INTERVAL_MS = 30_000;

/**
 * Hook returning daemon connectivity status.
 * Polls every 30 seconds — same cadence as DaemonHealthIndicator.
 */
export function useDaemonStatus(): DaemonStatusResult {
  const { getToken, isSignedIn } = useAuth();
  const [daemonConnected, setDaemonConnected] = useState(false);
  const [daemonChecked, setDaemonChecked] = useState(false);

  const checkHealth = useCallback(async () => {
    if (!isSignedIn) {
      setDaemonChecked(true);
      return;
    }
    try {
      const token = await getToken();
      if (!token) {
        setDaemonChecked(true);
        return;
      }
      const result = await getDaemonHealth(token);
      setDaemonConnected(result.status === 'connected');
    } catch {
      // Network error or daemon unreachable → treat as not connected
      setDaemonConnected(false);
    } finally {
      setDaemonChecked(true);
    }
  }, [getToken, isSignedIn]);

  useEffect(() => {
    let cancelled = false;

    const run = () => {
      if (!cancelled) checkHealth();
    };

    run();
    const id = setInterval(run, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [checkHealth]);

  return { daemonConnected, daemonChecked };
}
