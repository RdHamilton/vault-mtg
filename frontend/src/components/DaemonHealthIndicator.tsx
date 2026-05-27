import { useState, useEffect, useRef, useCallback } from 'react';
import { useAuth } from '@clerk/react';
import { getDaemonHealth } from '@/services/api/bffHealth';
import { trackEvent } from '@/services/analytics';
import './DaemonHealthIndicator.css';

type IndicatorState = 'connected' | 'disconnected' | 'reconnecting' | 'loading' | 'error';

export type DaemonHealthState = IndicatorState;

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

export interface DaemonHealthIndicatorProps {
  /**
   * Optional callback fired when the indicator dot is clicked while the
   * daemon is disconnected. Used by Layout to open the onboarding modal.
   */
  onOpenOnboarding?: () => void;
  /**
   * Optional callback fired after every health poll with the latest status.
   * Used by Layout to sync daemon status into the onboarding hook.
   */
  onStatusChange?: (status: DaemonHealthState) => void;
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
const DaemonHealthIndicator = ({ onOpenOnboarding, onStatusChange }: DaemonHealthIndicatorProps = {}) => {
  const { getToken, isSignedIn } = useAuth();
  const [status, setStatus] = useState<IndicatorState>('loading');
  // Track previous status to detect first transition TO connected, and
  // transitions FROM connected/reconnecting for error_daemon_connection_failed.
  const prevStatusRef = useRef<IndicatorState>('loading');
  const connectedFiredRef = useRef(false);
  // Track when the daemon first became connected so we can compute
  // duration_connected_seconds for error_daemon_connection_failed.
  const connectedSinceRef = useRef<number | null>(null);

  const updateStatus = useCallback((newStatus: IndicatorState) => {
    setStatus(newStatus);
    onStatusChange?.(newStatus);
  }, [onStatusChange]);

  const fetchHealth = useCallback(async () => {
    if (!isSignedIn) {
      updateStatus('error');
      return;
    }

    try {
      const token = await getToken();
      if (!token) {
        updateStatus('error');
        return;
      }
      const result = await getDaemonHealth(token);
      if (result.status === 'connected') {
        // Fire funnel_daemon_connected on first transition TO connected.
        if (!connectedFiredRef.current && prevStatusRef.current !== 'connected') {
          trackEvent({ name: 'funnel_daemon_connected' });
          connectedFiredRef.current = true;
          connectedSinceRef.current = Date.now();
        }
        prevStatusRef.current = 'connected';
        updateStatus('connected');
      } else if (result.status === 'reconnecting') {
        // Fire error_daemon_connection_failed when transitioning FROM connected
        // or reconnecting TO reconnecting (i.e. the daemon was previously healthy).
        if (prevStatusRef.current === 'connected') {
          const durationMs = connectedSinceRef.current !== null
            ? Date.now() - connectedSinceRef.current
            : 0;
          trackEvent({
            name: 'error_daemon_connection_failed',
            properties: {
              previous_status: 'connected',
              duration_connected_seconds: Math.floor(durationMs / 1000),
            },
          });
          connectedSinceRef.current = null;
        }
        prevStatusRef.current = 'reconnecting';
        updateStatus('reconnecting');
      } else {
        // Daemon is disconnected or unknown — fire error if we had a prior healthy state.
        if (prevStatusRef.current === 'connected' || prevStatusRef.current === 'reconnecting') {
          const durationMs = connectedSinceRef.current !== null
            ? Date.now() - connectedSinceRef.current
            : 0;
          trackEvent({
            name: 'error_daemon_connection_failed',
            properties: {
              previous_status: prevStatusRef.current as 'connected' | 'reconnecting',
              duration_connected_seconds: Math.floor(durationMs / 1000),
            },
          });
          connectedSinceRef.current = null;
        }
        prevStatusRef.current = 'disconnected';
        updateStatus('disconnected');
      }
    } catch {
      updateStatus('error');
    }
  }, [getToken, isSignedIn, updateStatus]);

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

  const isClickable = (status === 'disconnected' || status === 'reconnecting' || status === 'error') && !!onOpenOnboarding;

  return (
    <div
      className={`daemon-health-indicator daemon-health-${status}${isClickable ? ' daemon-health-clickable' : ''}`}
      title={isClickable ? `${tooltipText(status)} — click to open setup guide` : tooltipText(status)}
      data-testid="daemon-health-indicator"
      aria-label={isClickable ? `${tooltipText(status)} — click to open setup guide` : tooltipText(status)}
      role={isClickable ? 'button' : undefined}
      tabIndex={isClickable ? 0 : undefined}
      onClick={isClickable ? onOpenOnboarding : undefined}
      onKeyDown={isClickable ? (e) => { if (e.key === 'Enter' || e.key === ' ') onOpenOnboarding?.(); } : undefined}
    />
  );
};

export default DaemonHealthIndicator;
