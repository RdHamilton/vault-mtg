/**
 * useDaemonOnboarding
 *
 * Manages onboarding modal visibility for new users who haven't connected
 * the daemon yet.
 *
 * Rules:
 * - Show modal on first login if daemon is disconnected
 * - "Dismissed" state is stored in localStorage so it persists across sessions
 * - User can manually re-open via the status indicator in the nav
 * - Once daemon connects (status = connected), the modal auto-completes
 */

import { useState, useCallback } from 'react';

const STORAGE_KEY = 'vaultmtg_onboarding_dismissed';
const STORAGE_COMPLETED_KEY = 'vaultmtg_onboarding_completed';

export type DaemonOnboardingStatus = 'connected' | 'disconnected' | 'reconnecting' | 'loading' | 'error';

export interface UseDaemonOnboardingResult {
  /** Whether the onboarding modal should be shown */
  isOpen: boolean;
  /** Open the onboarding modal (e.g. from the status indicator) */
  open: () => void;
  /** Dismiss the modal without completing */
  dismiss: () => void;
  /** Mark onboarding as fully completed (daemon connected) */
  complete: () => void;
  /** Whether the user has previously dismissed or completed onboarding */
  hasSeenOnboarding: boolean;
}

function readHasSeen(): boolean {
  try {
    return (
      localStorage.getItem(STORAGE_KEY) === 'true' ||
      localStorage.getItem(STORAGE_COMPLETED_KEY) === 'true'
    );
  } catch {
    return false;
  }
}

/**
 * Hook that controls onboarding modal visibility based on daemon status and
 * whether the user has previously seen/dismissed the modal.
 *
 * @param daemonStatus  Current daemon health status from the health indicator
 * @param isSignedIn    Whether the user is signed in (from Clerk useAuth)
 */
export function useDaemonOnboarding(
  daemonStatus: DaemonOnboardingStatus,
  isSignedIn: boolean
): UseDaemonOnboardingResult {
  // manualOpen: true when the user explicitly opens the modal
  // manualClosed: true when the user has explicitly dismissed it this session
  const [manualOpen, setManualOpen] = useState(false);
  const [manualClosed, setManualClosed] = useState(false);
  const [hasSeenOnboarding, setHasSeenOnboarding] = useState(readHasSeen);

  // Auto-show condition: signed in, daemon disconnected, not seen before, not closed this session
  const autoShow =
    isSignedIn &&
    daemonStatus === 'disconnected' &&
    !hasSeenOnboarding &&
    !manualClosed;

  const isOpen = manualOpen || autoShow;

  const open = useCallback(() => {
    setManualOpen(true);
    setManualClosed(false);
  }, []);

  const dismiss = useCallback(() => {
    setManualOpen(false);
    setManualClosed(true);
    setHasSeenOnboarding(true);
    try {
      localStorage.setItem(STORAGE_KEY, 'true');
    } catch {
      // ignore storage errors
    }
  }, []);

  const complete = useCallback(() => {
    setManualOpen(false);
    setManualClosed(true);
    setHasSeenOnboarding(true);
    try {
      localStorage.setItem(STORAGE_COMPLETED_KEY, 'true');
      localStorage.setItem(STORAGE_KEY, 'true');
    } catch {
      // ignore storage errors
    }
  }, []);

  return { isOpen, open, dismiss, complete, hasSeenOnboarding };
}
