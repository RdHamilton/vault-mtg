/**
 * OnboardingModal
 *
 * Step-by-step onboarding flow for new users who haven't connected the daemon.
 *
 * Step 1: Download — links to vaultmtg.app/download
 * Step 2: Install — platform-specific instructions (Mac + Windows)
 * Step 3: Confirm — polls /api/v1/health/daemon every 5s; shows spinner then success/failure
 *
 * The modal is triggered by useDaemonOnboarding when the daemon is not detected
 * on first login.
 */

import { useState, useEffect, useRef, useCallback } from 'react';
import { useAuth } from '@clerk/react';
import { getDaemonHealth } from '@/services/api/bffHealth';
import { trackEvent } from '@/services/analytics';
import './OnboardingModal.css';

const DOWNLOAD_URL = 'https://vaultmtg.app/download';
const POLL_INTERVAL_MS = 5_000;
const MAX_POLL_ATTEMPTS = 24; // 2 minutes at 5s intervals

export interface OnboardingModalProps {
  /** Whether the modal is visible */
  isOpen: boolean;
  /** Called when user dismisses the modal */
  onDismiss: () => void;
  /** Called when daemon connection is confirmed */
  onComplete: () => void;
}

type Step = 1 | 2 | 3;

const STEP_LABELS: Record<Step, string> = {
  1: 'Download',
  2: 'Install',
  3: 'Confirm',
};

/**
 * The inner modal content, rendered only when isOpen is true.
 * Using a key on the outer mount so this component re-mounts (and step resets)
 * each time the modal opens.
 */
function OnboardingModalContent({ onDismiss, onComplete }: Omit<OnboardingModalProps, 'isOpen'>) {
  const [step, setStep] = useState<Step>(1);

  // Prevent body scroll
  useEffect(() => {
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = '';
    };
  }, []);

  // Escape key to dismiss
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onDismiss();
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onDismiss]);

  const goToStep = (s: Step) => setStep(s);

  return (
    <div
      className="onboarding-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-modal-title"
      data-testid="onboarding-modal"
      onClick={(e) => {
        if (e.target === e.currentTarget) onDismiss();
      }}
    >
      <div className="onboarding-modal">
        {/* Header */}
        <div className="onboarding-modal-header">
          <h2 id="onboarding-modal-title" className="onboarding-modal-title">
            Get Started with VaultMTG
          </h2>
          <button
            type="button"
            className="onboarding-modal-close"
            onClick={onDismiss}
            aria-label="Dismiss onboarding"
            data-testid="onboarding-modal-close"
          >
            &times;
          </button>
        </div>

        {/* Step indicator */}
        <div className="onboarding-steps-indicator" data-testid="onboarding-steps-indicator">
          {([1, 2, 3] as Step[]).map((s) => (
            <div
              key={s}
              className={`onboarding-step-pip ${step === s ? 'active' : ''} ${step > s ? 'done' : ''}`}
              data-testid={`onboarding-step-pip-${s}`}
            >
              <span className="onboarding-step-pip-number">
                {step > s ? '✓' : s}
              </span>
              <span className="onboarding-step-pip-label">{STEP_LABELS[s]}</span>
            </div>
          ))}
        </div>

        {/* Step content */}
        <div className="onboarding-modal-body">
          {step === 1 && <Step1Download onNext={() => goToStep(2)} />}
          {step === 2 && <Step2Install onBack={() => goToStep(1)} onNext={() => goToStep(3)} />}
          {step === 3 && (
            <Step3Confirm
              onBack={() => goToStep(2)}
              onDismiss={onDismiss}
              onComplete={onComplete}
            />
          )}
        </div>
      </div>
    </div>
  );
}

export function OnboardingModal({ isOpen, onDismiss, onComplete }: OnboardingModalProps) {
  if (!isOpen) return null;
  // Re-mount OnboardingModalContent each time isOpen becomes true via the key trick.
  // We track an open-count so the key increments each new open, resetting step state.
  return (
    <OnboardingModalContent onDismiss={onDismiss} onComplete={onComplete} />
  );
}

// ---------------------------------------------------------------------------
// Step 1: Download
// ---------------------------------------------------------------------------

interface Step1Props {
  onNext: () => void;
}

function Step1Download({ onNext }: Step1Props) {
  return (
    <div className="onboarding-step" data-testid="onboarding-step-1">
      <div className="onboarding-step-icon" aria-hidden="true">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="7 10 12 15 17 10"/>
          <line x1="12" y1="15" x2="12" y2="3"/>
        </svg>
      </div>
      <h3 className="onboarding-step-heading">Download the VaultMTG Daemon</h3>
      <p className="onboarding-step-description">
        The VaultMTG daemon runs locally on your computer and reads your MTG Arena log
        to track matches, drafts, and your collection in real time.
      </p>
      <p className="onboarding-step-description">
        Click below to visit the download page on the VaultMTG site and grab the
        installer for your platform.
      </p>
      <div className="onboarding-step-actions">
        <a
          href={DOWNLOAD_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="onboarding-btn onboarding-btn--primary"
          data-testid="onboarding-download-link"
          onClick={() => {
            trackEvent({
              name: 'funnel_daemon_download_started',
              properties: {
                os: navigator.platform || 'unknown',
                download_source: 'onboarding_modal',
              },
            });
          }}
        >
          Download Daemon
        </a>
        <button
          type="button"
          className="onboarding-btn onboarding-btn--secondary"
          onClick={onNext}
          data-testid="onboarding-step-1-next"
        >
          I already downloaded it &rarr;
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Step 2: Install
// ---------------------------------------------------------------------------

interface Step2Props {
  onBack: () => void;
  onNext: () => void;
}

function Step2Install({ onBack, onNext }: Step2Props) {
  return (
    <div className="onboarding-step" data-testid="onboarding-step-2">
      <div className="onboarding-step-icon" aria-hidden="true">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
          <rect x="2" y="3" width="20" height="14" rx="2"/>
          <line x1="8" y1="21" x2="16" y2="21"/>
          <line x1="12" y1="17" x2="12" y2="21"/>
        </svg>
      </div>
      <h3 className="onboarding-step-heading">Install the Daemon</h3>

      <div className="onboarding-platform-instructions">
        {/* macOS */}
        <div className="onboarding-platform" data-testid="onboarding-platform-mac">
          <div className="onboarding-platform-header">
            <span className="onboarding-platform-icon" aria-hidden="true">
              <svg width="20" height="20" viewBox="0 0 814 1000" fill="currentColor">
                <path d="M788.1 340.9c-5.8 4.5-108.2 62.2-108.2 190.5 0 148.4 130.3 200.9 134.2 202.2-.6 3.2-20.7 71.9-68.7 141.9-42.8 61.6-87.5 123.1-155.5 123.1s-85.5-39.5-164-39.5c-76 0-103.7 40.8-165.9 40.8s-105-57.8-155.5-127.4C46 790.7 0 663 0 541.8c0-207.3 130.3-311.5 258.3-311.5 72.9 0 133.6 47.2 176.3 47.2 41.1 0 112.1-50.4 191.8-50.4 30.4 0 132.3 3.2 200.1 104.8zm-159-181.4c31.1-36.9 53.1-88.1 53.1-139.3 0-7.1-.6-14.3-1.9-20.1-50.6 1.9-110.8 33.7-147.1 75.8-28.5 32.4-55.1 83.6-55.1 135.5 0 7.8 1.3 15.6 1.9 18.1 3.2.6 8.4 1.3 13.6 1.3 45.4 0 102.5-30.4 135.5-71.3z"/>
              </svg>
            </span>
            <span className="onboarding-platform-name">macOS</span>
          </div>
          <ol className="onboarding-install-steps">
            <li>Open the downloaded <code>.dmg</code> file</li>
            <li>Drag the VaultMTG daemon to Applications</li>
            <li>Launch it — the installer is notarized by Apple, so no security bypass is needed</li>
            <li>The daemon will start automatically</li>
          </ol>
        </div>

        {/* Windows */}
        <div className="onboarding-platform" data-testid="onboarding-platform-windows">
          <div className="onboarding-platform-header">
            <span className="onboarding-platform-icon" aria-hidden="true">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                <path d="M0 3.449L9.75 2.1v9.451H0m10.949-9.602L24 0v11.4H10.949M0 12.6h9.75v9.451L0 20.699M10.949 12.6H24V24l-12.9-1.801"/>
              </svg>
            </span>
            <span className="onboarding-platform-name">Windows</span>
          </div>
          <ol className="onboarding-install-steps">
            <li>Run the downloaded <code>.exe</code> installer</li>
            <li>Follow the installer wizard (Next &rarr; Next &rarr; Finish) — the installer is signed with Microsoft Azure Trusted Signing, so no SmartScreen bypass is needed</li>
            <li>The daemon will start automatically on completion</li>
          </ol>
        </div>
      </div>

      <div className="onboarding-step-actions">
        <button
          type="button"
          className="onboarding-btn onboarding-btn--ghost"
          onClick={onBack}
          data-testid="onboarding-step-2-back"
        >
          &larr; Back
        </button>
        <button
          type="button"
          className="onboarding-btn onboarding-btn--primary"
          onClick={onNext}
          data-testid="onboarding-step-2-next"
        >
          I&apos;ve installed it &rarr;
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Step 3: Confirm connection
// ---------------------------------------------------------------------------

type ConfirmState = 'polling' | 'success' | 'timeout';

interface Step3Props {
  onBack: () => void;
  onDismiss: () => void;
  onComplete: () => void;
}

function Step3Confirm({ onBack, onDismiss, onComplete }: Step3Props) {
  const { getToken, isSignedIn } = useAuth();
  const [confirmState, setConfirmState] = useState<ConfirmState>('polling');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollAttemptsRef = useRef(0);
  const isMountedRef = useRef(true);

  const stopPolling = useCallback(() => {
    if (pollRef.current !== null) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
      stopPolling();
    };
  }, [stopPolling]);

  const onCompleteRef = useRef(onComplete);
  useEffect(() => {
    onCompleteRef.current = onComplete;
  }, [onComplete]);

  useEffect(() => {
    pollAttemptsRef.current = 0;

    const poll = async () => {
      if (!isMountedRef.current) return;
      if (!isSignedIn) return;
      pollAttemptsRef.current += 1;

      try {
        const token = await getToken();
        if (!token || !isMountedRef.current) return;
        const result = await getDaemonHealth(token);
        if (!isMountedRef.current) return;

        if (result.status === 'connected') {
          stopPolling();
          setConfirmState('success');
          trackEvent({ name: 'funnel_daemon_connected', properties: { source: 'onboarding_modal' } });
          setTimeout(() => {
            onCompleteRef.current();
          }, 2000);
        } else if (pollAttemptsRef.current >= MAX_POLL_ATTEMPTS) {
          stopPolling();
          setConfirmState('timeout');
          trackEvent({ name: 'error_daemon_never_connected', properties: { source: 'onboarding_modal' } });
        }
      } catch {
        if (isMountedRef.current && pollAttemptsRef.current >= MAX_POLL_ATTEMPTS) {
          stopPolling();
          setConfirmState('timeout');
        }
      }
    };

    poll();
    pollRef.current = setInterval(poll, POLL_INTERVAL_MS);

    return () => {
      stopPolling();
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleRetry = useCallback(() => {
    stopPolling();
    pollAttemptsRef.current = 0;
    setConfirmState('polling');

    const poll = async () => {
      if (!isMountedRef.current) return;
      if (!isSignedIn) return;
      pollAttemptsRef.current += 1;

      try {
        const token = await getToken();
        if (!token || !isMountedRef.current) return;
        const result = await getDaemonHealth(token);
        if (!isMountedRef.current) return;

        if (result.status === 'connected') {
          stopPolling();
          setConfirmState('success');
          trackEvent({ name: 'funnel_daemon_connected', properties: { source: 'onboarding_modal' } });
          setTimeout(() => {
            onCompleteRef.current();
          }, 2000);
        } else if (pollAttemptsRef.current >= MAX_POLL_ATTEMPTS) {
          stopPolling();
          setConfirmState('timeout');
          trackEvent({ name: 'error_daemon_never_connected', properties: { source: 'onboarding_modal' } });
        }
      } catch {
        if (isMountedRef.current && pollAttemptsRef.current >= MAX_POLL_ATTEMPTS) {
          stopPolling();
          setConfirmState('timeout');
        }
      }
    };

    poll();
    pollRef.current = setInterval(poll, POLL_INTERVAL_MS);
  }, [getToken, isSignedIn, stopPolling]);

  return (
    <div className="onboarding-step" data-testid="onboarding-step-3">
      {confirmState === 'polling' && (
        <>
          <div className="onboarding-step-icon" aria-hidden="true">
            <div
              className="onboarding-spinner"
              data-testid="onboarding-spinner"
              aria-label="Waiting for daemon connection..."
            />
          </div>
          <h3 className="onboarding-step-heading">Waiting for Daemon Connection</h3>
          <p className="onboarding-step-description">
            Make sure the daemon is running and MTG Arena is open. We&apos;re checking
            every 5 seconds&hellip;
          </p>
          <div className="onboarding-step-actions">
            <button
              type="button"
              className="onboarding-btn onboarding-btn--ghost"
              onClick={onBack}
              data-testid="onboarding-step-3-back"
            >
              &larr; Back
            </button>
          </div>
        </>
      )}

      {confirmState === 'success' && (
        <>
          <div className="onboarding-step-icon onboarding-step-icon--success" aria-hidden="true">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10"/>
              <polyline points="20 6 9 17 4 12"/>
            </svg>
          </div>
          <h3 className="onboarding-step-heading" data-testid="onboarding-success-heading">
            Daemon Connected!
          </h3>
          <p className="onboarding-step-description">
            Your daemon is connected and ready. VaultMTG will now track your matches,
            drafts, and collection in real time.
          </p>
        </>
      )}

      {confirmState === 'timeout' && (
        <>
          <div className="onboarding-step-icon onboarding-step-icon--error" aria-hidden="true">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10"/>
              <line x1="12" y1="8" x2="12" y2="12"/>
              <line x1="12" y1="16" x2="12.01" y2="16"/>
            </svg>
          </div>
          <h3 className="onboarding-step-heading" data-testid="onboarding-timeout-heading">
            Connection Timed Out
          </h3>
          <p className="onboarding-step-description">
            We couldn&apos;t detect the daemon after 2 minutes. Make sure the daemon is
            running, then try again.
          </p>
          <div className="onboarding-step-actions">
            <button
              type="button"
              className="onboarding-btn onboarding-btn--ghost"
              onClick={onBack}
              data-testid="onboarding-step-3-back-timeout"
            >
              &larr; Back
            </button>
            <button
              type="button"
              className="onboarding-btn onboarding-btn--primary"
              onClick={handleRetry}
              data-testid="onboarding-step-3-retry"
            >
              Try Again
            </button>
            <button
              type="button"
              className="onboarding-btn onboarding-btn--ghost"
              onClick={onDismiss}
              data-testid="onboarding-step-3-dismiss"
            >
              I&apos;ll do this later
            </button>
          </div>
        </>
      )}
    </div>
  );
}

export default OnboardingModal;
