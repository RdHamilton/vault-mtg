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
 *
 * ADR-020: The SPA does NOT mint API keys. The daemon handles the full PKCE
 * flow (opens browser → captures code on localhost callback → calls BFF
 * /v1/daemon/register). The SPA only polls localhost:9001/health and redirects
 * the user to the dashboard once setup is complete.
 */

import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { trackEvent, type Platform } from '@/services/analytics';
import gatekeeperScreenshot from '@/assets/gatekeeper-warning.svg';
import smartscreenScreenshot from '@/assets/smartscreen-warning.svg';
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

interface DaemonHealthResponse {
  configured?: boolean;
  status?: string;
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
      <div className="setup-warning-icon" aria-hidden="true">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="12" cy="12" r="10" stroke="#f59e0b" strokeWidth="2" />
          <path d="M12 8v4m0 4h.01" stroke="#f59e0b" strokeWidth="2" strokeLinecap="round" />
        </svg>
      </div>
      <div className="setup-warning-content">
        <h3 className="setup-warning-title">macOS Gatekeeper Warning</h3>
        <p className="setup-warning-body">
          When you open the VaultMTG daemon for the first time, macOS may show a message
          like <em>"cannot be opened because it is from an unidentified developer"</em>.
          This is expected for unsigned beta software — the app is safe.
        </p>
        <p className="setup-warning-body">
          To allow it, use either of these methods:
        </p>
        <ol className="setup-warning-steps">
          <li>
            <strong>Right-click method:</strong> Right-click (or Control-click) the app
            icon and choose <strong>Open</strong>. Click <strong>Open</strong> again in
            the dialog that appears.
          </li>
          <li>
            <strong>System Settings method:</strong> Open{' '}
            <strong>System Settings → Privacy &amp; Security</strong>. Scroll down to
            the Security section and click <strong>Open Anyway</strong> next to the
            VaultMTG daemon entry.
          </li>
        </ol>
        <p className="setup-warning-note">
          You only need to do this once. After the first approval, macOS will remember
          your choice and open the daemon normally on every subsequent launch.
        </p>
        <img
          src={gatekeeperScreenshot}
          alt="macOS Gatekeeper security warning dialog"
          className="setup-warning-screenshot"
          data-testid="gatekeeper-screenshot"
        />
      </div>
    </div>
  );
}

function SmartScreenWarning() {
  return (
    <div className="setup-warning-section" data-testid="smartscreen-warning">
      <div className="setup-warning-icon" aria-hidden="true">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="12" cy="12" r="10" stroke="#f59e0b" strokeWidth="2" />
          <path d="M12 8v4m0 4h.01" stroke="#f59e0b" strokeWidth="2" strokeLinecap="round" />
        </svg>
      </div>
      <div className="setup-warning-content">
        <h3 className="setup-warning-title">Windows SmartScreen Warning</h3>
        <p className="setup-warning-body">
          Windows Defender SmartScreen may show a blue dialog that says{' '}
          <em>"Windows protected your PC"</em>. This is expected for unsigned beta
          software from a new publisher — the installer is safe.
        </p>
        <p className="setup-warning-body">
          To continue with installation:
        </p>
        <ol className="setup-warning-steps">
          <li>
            Click <strong>More info</strong> in the SmartScreen dialog.
          </li>
          <li>
            Click <strong>Run anyway</strong>.
          </li>
          <li>
            Complete the installer (Next → Next → Finish).
          </li>
        </ol>
        <p className="setup-warning-note">
          You only need to do this once. SmartScreen will not prompt again for the
          same installer on your machine.
        </p>
        <img
          src={smartscreenScreenshot}
          alt="Windows SmartScreen protection dialog"
          className="setup-warning-screenshot"
          data-testid="smartscreen-screenshot"
        />
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
// Main Setup component
// ---------------------------------------------------------------------------

export default function Setup() {
  const navigate = useNavigate();
  const platform = detectPlatform();

  const [pairingState, setPairingState] = useState<PairingState>('waiting');
  const [pollActive, setPollActive] = useState(true);

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
    setPollActive(true);
  };

  useEffect(() => {
    if (!pollActive) return;

    trackEvent({ name: 'setup_page_viewed', properties: { platform } });

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
          >
            download page
          </a>
          , then run it to install the daemon.
        </p>
      </section>

      {/* Step 2: Platform-specific install warnings */}
      <section className="setup-section" data-testid="setup-warnings-section">
        <h2 className="setup-section-title">Step 2 — Bypass the Security Warning</h2>
        <p className="setup-section-body">
          Because VaultMTG is indie beta software, your OS may show a security warning
          on first run. This is normal — follow the instructions below to allow it.
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

      {/* Step 3: PKCE pairing status */}
      <section className="setup-section" data-testid="setup-pairing-section">
        <h2 className="setup-section-title">Step 3 — Sign In</h2>
        <p className="setup-section-body">
          Once the daemon is installed and running, it will open your browser to
          complete sign-in. Your VaultMTG account will be linked automatically.
        </p>
        <PairingStatus
          state={pairingState}
          onRetry={startPolling}
        />
      </section>
    </div>
  );
}
