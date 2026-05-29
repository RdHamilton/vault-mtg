/**
 * Setup Page
 *
 * Entry point for first-time daemon installation and PKCE pairing.
 *
 * Responsibilities:
 * 1. Show platform-appropriate install warnings (Gatekeeper / SmartScreen)
 *    so beta users understand unsigned-binary warnings are expected (#1644).
 * 2. Poll daemon local health to detect when PKCE pairing completes (#1645).
 *    The daemon drives the PKCE OAuth flow — the SPA's only job here is to
 *    show progress and redirect once `configured: true` is returned.
 * 3. Render the daemon's auth state (4 states) when `auth_status` is present
 *    in the local /health response (#2142). Auth state is only available on the
 *    local endpoint per ADR-020 — the BFF health path is a DB-derived liveness
 *    signal and does not carry auth_status.
 *
 * ADR-020: The SPA does NOT mint API keys. The daemon handles the full PKCE
 * flow (opens browser → captures code on localhost callback → calls BFF
 * /v1/daemon/register). The SPA only polls localhost:9001/health and redirects
 * the user to the dashboard once setup is complete.
 */

import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { trackEvent, type Platform } from '@/services/analytics';
import { isDesktopApp } from '@/lib/runtimeContext';
import './Setup.css';

function detectPlatform(): Platform {
  const ua = navigator.userAgent.toLowerCase();
  const platform =
    typeof navigator.platform === 'string' ? navigator.platform.toLowerCase() : '';
  if (platform.includes('win') || ua.includes('windows')) return 'windows';
  if (platform.includes('mac') || ua.includes('mac')) return 'macos';
  return 'unknown';
}

// ---------------------------------------------------------------------------
// Daemon health polling
// ---------------------------------------------------------------------------

const DAEMON_HEALTH_URL = 'http://localhost:9001/health';
const POLL_INTERVAL_MS = 3_000;
const TIMEOUT_MS = 60_000;

type PairingState = 'waiting' | 'success' | 'error';

/**
 * The four daemon auth states surfaced on the local /health endpoint (#2142).
 *
 * Note on precedence: the daemon's computeAuthStatus routing makes auth_paused
 * outrank keychain_error — a paused daemon with a keychain error reports
 * auth_paused, not keychain_error. These are therefore not mutually independent.
 */
export type DaemonAuthStatus =
  | 'authenticated'
  | 'setup_required'
  | 'keychain_error'
  | 'auth_paused';

interface DaemonHealthResponse {
  configured?: boolean;
  status?: string;
  auth_status?: DaemonAuthStatus;
}

async function fetchDaemonHealth(): Promise<DaemonHealthResponse> {
  const res = await fetch(DAEMON_HEALTH_URL, { mode: 'cors' });
  if (!res.ok) throw new Error(`daemon health returned ${res.status}`);
  return res.json() as Promise<DaemonHealthResponse>;
}

// ---------------------------------------------------------------------------
// Warning sections
// ---------------------------------------------------------------------------

function GatekeeperWarning() {
  return (
    <div className="setup-warning-section" data-testid="gatekeeper-warning">
      <div className="setup-warning-icon setup-warning-icon--success" aria-hidden="true">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="12" cy="12" r="10" stroke="#22c55e" strokeWidth="2" />
          <path d="M8 12l3 3 5-5" stroke="#22c55e" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>
      <div className="setup-warning-content">
        <h3 className="setup-warning-title">macOS — No Security Bypass Needed</h3>
        <p className="setup-warning-body">
          The VaultMTG daemon is notarized by Apple using a Developer ID Application
          certificate. macOS Gatekeeper will recognize it as trusted software and open
          it without warnings.
        </p>
        <p className="setup-warning-note">
          Simply open the <code>.dmg</code>, drag the daemon to Applications, and
          launch it. No right-click workarounds required.
        </p>
      </div>
    </div>
  );
}

function SmartScreenWarning() {
  return (
    <div className="setup-warning-section" data-testid="smartscreen-warning">
      <div className="setup-warning-icon setup-warning-icon--success" aria-hidden="true">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="12" cy="12" r="10" stroke="#22c55e" strokeWidth="2" />
          <path d="M8 12l3 3 5-5" stroke="#22c55e" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>
      <div className="setup-warning-content">
        <h3 className="setup-warning-title">Windows — No Security Bypass Needed</h3>
        <p className="setup-warning-body">
          The VaultMTG daemon installer is signed with Microsoft Azure Trusted Signing.
          Windows will recognize it as trusted software and install it without SmartScreen
          warnings.
        </p>
        <p className="setup-warning-note">
          Run the <code>.exe</code> installer and follow the wizard (Next &rarr; Next &rarr;
          Finish). On your very first install, Windows may briefly show a reputation check
          while SmartScreen learns the new certificate — this is normal and resolves within
          a few seconds.
        </p>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// PKCE pairing status panel
// ---------------------------------------------------------------------------

interface PairingStatusProps {
  state: PairingState;
  onRetry: () => void;
}

function PairingStatus({ state, onRetry }: PairingStatusProps) {
  return (
    <div className="setup-pairing" data-testid="pairing-status">
      {state === 'waiting' && (
        <div className="setup-pairing-waiting" data-testid="pairing-waiting">
          <div className="setup-pairing-spinner" aria-hidden="true" />
          <p className="setup-pairing-label">Waiting for auth...</p>
          <p className="setup-pairing-sublabel">
            The daemon will open your browser to complete sign-in. Once you log in,
            this page will advance automatically.
          </p>
        </div>
      )}
      {state === 'success' && (
        <div className="setup-pairing-success" data-testid="pairing-success">
          <span className="setup-pairing-checkmark" aria-hidden="true">&#10003;</span>
          <p className="setup-pairing-label">Auth complete — redirecting...</p>
        </div>
      )}
      {state === 'error' && (
        <div className="setup-pairing-error" data-testid="pairing-error">
          <p className="setup-pairing-label setup-pairing-label--error">
            Setup timed out. Make sure the VaultMTG daemon is running and try again.
          </p>
          <button
            className="setup-retry-button"
            onClick={onRetry}
            data-testid="retry-button"
          >
            Retry
          </button>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Auth status panel (#2142)
// ---------------------------------------------------------------------------

interface AuthStatusPanelProps {
  status: DaemonAuthStatus;
  onRetry: () => void;
}

function AuthStatusPanel({ status, onRetry }: AuthStatusPanelProps) {
  return (
    <div className="setup-auth-status" data-testid="auth-status-panel">
      {status === 'authenticated' && (
        <div
          className="setup-auth-status__item setup-auth-status__item--authenticated"
          data-testid="auth-status-authenticated"
        >
          <span className="setup-auth-status__dot setup-auth-status__dot--green" aria-hidden="true" />
          <span className="setup-auth-status__label">Connected</span>
        </div>
      )}
      {status === 'setup_required' && (
        <div
          className="setup-auth-status__item setup-auth-status__item--setup-required"
          data-testid="auth-status-setup-required"
        >
          <span className="setup-auth-status__dot setup-auth-status__dot--yellow" aria-hidden="true" />
          <span className="setup-auth-status__label">Setup required</span>
          <a
            href="/setup"
            className="setup-auth-status__cta"
            data-testid="auth-status-cta"
          >
            Complete setup
          </a>
        </div>
      )}
      {status === 'keychain_error' && (
        <div
          className="setup-auth-status__item setup-auth-status__item--keychain-error"
          data-testid="auth-status-keychain-error"
        >
          <span className="setup-auth-status__dot setup-auth-status__dot--red" aria-hidden="true" />
          <span className="setup-auth-status__label">Keychain unavailable</span>
          {/* TODO(#follow-on): replace # with real docs URL once docs page exists */}
          <a
            href="#"
            className="setup-auth-status__cta"
            data-testid="auth-status-cta"
          >
            Learn more
          </a>
        </div>
      )}
      {status === 'auth_paused' && (
        <div
          className="setup-auth-status__item setup-auth-status__item--auth-paused"
          data-testid="auth-status-auth-paused"
        >
          <span className="setup-auth-status__dot setup-auth-status__dot--orange" aria-hidden="true" />
          <span className="setup-auth-status__label">Sync paused</span>
          <button
            className="setup-auth-status__cta"
            onClick={onRetry}
            data-testid="auth-status-cta"
          >
            Retry setup
          </button>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Setup component
// ---------------------------------------------------------------------------

export default function Setup() {
  const navigate = useNavigate();
  const platform = detectPlatform();

  const [pairingState, setPairingState] = useState<PairingState>('waiting');
  const [pollActive, setPollActive] = useState(true);
  const [authStatus, setAuthStatus] = useState<DaemonAuthStatus | null>(null);

  // Refs to manage polling lifecycle across retries
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const stopPolling = () => {
    if (intervalRef.current) clearInterval(intervalRef.current);
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
  };

  const startPolling = () => {
    stopPolling();
    setPairingState('waiting');
    setAuthStatus(null);
    setPollActive(true);
  };

  useEffect(() => {
    if (!pollActive) return;

    trackEvent({ name: 'setup_page_viewed', properties: { platform } });

    // Only probe the local daemon in the desktop app context. In browser-only
    // sessions isDesktopApp() returns false, so we skip the interval entirely
    // to avoid ERR_CONNECTION_REFUSED noise from `http://localhost:9001/health`.
    // This matches the pattern used in useDaemonConnection.ts (#1927 AC1).
    if (!isDesktopApp()) {
      return;
    }

    // Timeout → error state after 60s
    timeoutRef.current = setTimeout(() => {
      stopPolling();
      setPairingState('error');
      setPollActive(false);
      trackEvent({ name: 'setup_pairing_timeout', properties: { platform } });
    }, TIMEOUT_MS);

    // Poll daemon health every 3s
    intervalRef.current = setInterval(async () => {
      try {
        const health = await fetchDaemonHealth();
        // Capture auth_status when the daemon provides it (#2142).
        // auth_status is a separate concern from the PKCE pairing state —
        // it is surfaced in the UI alongside the existing pairing flow.
        if (health.auth_status) {
          setAuthStatus(health.auth_status);
        }
        if (health.configured === true || health.status === 'ok') {
          stopPolling();
          setPairingState('success');
          setPollActive(false);
          trackEvent({ name: 'setup_pairing_success', properties: { platform } });
          // Brief delay so user sees success state before redirect
          setTimeout(() => navigate('/match-history'), 1500);
        }
      } catch {
        // Daemon not reachable yet — keep polling until timeout
      }
    }, POLL_INTERVAL_MS);

    return stopPolling;
  }, [pollActive, navigate, platform]);

  return (
    <div className="setup-container" data-testid="setup-container">
      <div className="setup-header">
        <h1 className="setup-title">Install the VaultMTG Daemon</h1>
        <p className="setup-subtitle">
          The VaultMTG daemon runs in the background while you play MTG Arena and
          syncs your match history, draft picks, and collection to your account.
        </p>
      </div>

      {/* Step 1: Download (links to /download page) */}
      <section className="setup-section" data-testid="setup-download-section">
        <h2 className="setup-section-title">Step 1 — Download</h2>
        <p className="setup-section-body">
          Download the installer for your platform from the{' '}
          <a
            href="/download"
            className="setup-link"
            data-testid="download-page-link"
            target="_blank"
            rel="noopener noreferrer"
          >
            download page
          </a>
          , then run it to install the daemon.
        </p>
      </section>

      {/* Step 2: Platform-specific install notes */}
      <section className="setup-section" data-testid="setup-warnings-section">
        <h2 className="setup-section-title">Step 2 — Install</h2>
        <p className="setup-section-body">
          The VaultMTG daemon is signed and notarized for both macOS and Windows.
          No security bypass is required.
        </p>

        {/* Always show both sections; highlight the detected platform */}
        <div className="setup-warnings" data-testid="setup-warnings">
          {(platform === 'macos' || platform === 'unknown') && (
            <GatekeeperWarning />
          )}
          {(platform === 'windows' || platform === 'unknown') && (
            <SmartScreenWarning />
          )}
          {/* When a platform is detected, show the other as a collapsible "wrong platform?" */}
          {platform === 'macos' && (
            <details className="setup-other-platform" data-testid="smartscreen-details">
              <summary>On Windows instead?</summary>
              <SmartScreenWarning />
            </details>
          )}
          {platform === 'windows' && (
            <details className="setup-other-platform" data-testid="gatekeeper-details">
              <summary>On macOS instead?</summary>
              <GatekeeperWarning />
            </details>
          )}
        </div>
      </section>

      {/* Step 3: PKCE pairing status / auth state */}
      <section className="setup-section" data-testid="setup-pairing-section">
        <h2 className="setup-section-title">Step 3 — Sign In</h2>
        <p className="setup-section-body">
          Once the daemon is installed and running, it will open your browser to
          complete sign-in. Your VaultMTG account will be linked automatically.
        </p>
        {authStatus !== null ? (
          <AuthStatusPanel status={authStatus} onRetry={startPolling} />
        ) : (
          <PairingStatus
            state={pairingState}
            onRetry={startPolling}
          />
        )}
      </section>
    </div>
  );
}
