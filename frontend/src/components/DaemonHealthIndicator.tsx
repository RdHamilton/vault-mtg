import { useState, useEffect, useCallback } from 'react';
import { useAuth } from '@clerk/react';
import { getDaemonHealth } from '@/services/api/bffHealth';
import './DaemonHealthIndicator.css';

type IndicatorState = 'connected' | 'disconnected' | 'reconnecting' | 'loading' | 'error';

const POLL_INTERVAL_MS = 30_000;

function tooltipText(state: IndicatorState): string {
  switch (state) {
    case 'connected':
      return 'Daemon connected';
    case 'disconnected':
      return 'Daemon not connected — data may be stale';
    case 'reconnecting':
      return 'Daemon reconnecting...';
    case 'loading':
      return 'Checking...';
    case 'error':
      return 'Checking...';
  }
}

/**
 * DaemonHealthIndicator
 *
 * Polls GET /api/v1/health/daemon every 30 seconds and renders a status dot:
 *   green  — daemon connected
 *   red    — daemon disconnected
 *   gray   — loading or error
 *
 * Uses the REST API adapter (getDaemonHealth) — never calls fetch directly.
 * Clerk auth token is obtained via useAuth().getToken() at the call site.
 */
const DaemonHealthIndicator = () => {
  const { getToken, isSignedIn } = useAuth();
  const [status, setStatus] = useState<IndicatorState>('loading');

  const fetchHealth = useCallback(async () => {
    if (!isSignedIn) {
      setStatus('error');
      return;
    }

    try {
      const token = await getToken();
      if (!token) {
        setStatus('error');
        return;
      }
      const result = await getDaemonHealth(token);
      if (result.status === 'connected') {
        setStatus('connected');
      } else if (result.status === 'reconnecting') {
        setStatus('reconnecting');
      } else {
        setStatus('disconnected');
      }
    } catch {
      setStatus('error');
    }
  }, [getToken, isSignedIn]);

  useEffect(() => {
    let cancelled = false;

    const runFetch = () => {
      if (!cancelled) {
        fetchHealth();
      }
    };

    runFetch();
    const intervalId = setInterval(runFetch, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
  }, [fetchHealth]);

  return (
    <div
      className={`daemon-health-indicator daemon-health-${status}`}
      title={tooltipText(status)}
      data-testid="daemon-health-indicator"
      aria-label={tooltipText(status)}
    />
  );
};

export default DaemonHealthIndicator;
