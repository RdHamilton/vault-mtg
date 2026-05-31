import type { Meta, StoryObj } from '@storybook/react';
import { OnboardingModal } from './OnboardingModal';
import './OnboardingModal.css';

/**
 * OnboardingModal — the step-by-step first-run modal shown when a signed-in
 * user has not yet connected the daemon.
 *
 * The modal has three steps:
 *   Step 1 (Download)  — link to the VaultMTG download page
 *   Step 2 (Install)   — platform-specific install instructions (macOS + Windows)
 *   Step 3 (Confirm)   — polls /api/v1/health/daemon; shows spinner → success/timeout
 *
 * Step 3 polls the BFF, which is not available in Storybook. Stories for that
 * step use a render override that renders the step 3 sub-states (polling,
 * success, timeout) as static HTML using the component's own CSS classes,
 * so Chromatic can snapshot each variant without any network dependency.
 *
 * Auth state for Step 3 is provided via the Clerk mock — `@clerk/react` is
 * aliased to `.storybook/clerk-mock`, so `useAuth()` returns the mock token
 * without a real Clerk session.
 *
 * States covered:
 *   - Open (Step 1)      — Download step, first-run entry point
 *   - Step2Install       — Install step
 *   - Step3Polling       — Confirm step, waiting for daemon
 *   - Step3Success       — Confirm step, daemon connected
 *   - Step3Error         — Confirm step, connection timed out
 *   - Closed (not open)  — Modal not rendered (isOpen = false)
 */
const meta: Meta<typeof OnboardingModal> = {
  title: 'Organisms/OnboardingModal',
  component: OnboardingModal,
  parameters: {
    layout: 'fullscreen',
    clerk: { signedIn: true },
  },
  tags: ['autodocs'],
  args: {
    onDismiss: () => {},
    onComplete: () => {},
  },
};

export default meta;
type Story = StoryObj<typeof OnboardingModal>;

/**
 * Open (Step 1 — Download) — the default first-run state. The user sees the
 * download prompt and the step indicator at step 1.
 *
 * The modal component starts at step 1 when first mounted.
 */
export const Step1Download: Story = {
  args: {
    isOpen: true,
  },
};

/**
 * Step 2 — Install. Shows the platform-specific macOS and Windows install
 * instructions inside the modal.
 *
 * Rendered via static HTML because the live component starts at step 1 and
 * requires a user click to advance. This snapshot captures the exact visual
 * state for Chromatic without requiring interaction.
 */
export const Step2Install: Story = {
  render: ({ onDismiss, onComplete }) => (
    <div
      className="onboarding-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-modal-title"
      data-testid="onboarding-modal"
    >
      <div className="onboarding-modal">
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

        <div className="onboarding-steps-indicator" data-testid="onboarding-steps-indicator">
          {[1, 2, 3].map((s) => (
            <div
              key={s}
              className={`onboarding-step-pip ${s === 2 ? 'active' : ''} ${s < 2 ? 'done' : ''}`}
              data-testid={`onboarding-step-pip-${s}`}
            >
              <span className="onboarding-step-pip-number">{s < 2 ? '✓' : s}</span>
              <span className="onboarding-step-pip-label">
                {s === 1 ? 'Download' : s === 2 ? 'Install' : 'Confirm'}
              </span>
            </div>
          ))}
        </div>

        <div className="onboarding-modal-body">
          <div className="onboarding-step" data-testid="onboarding-step-2">
            <div className="onboarding-step-icon" aria-hidden="true">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <rect x="2" y="3" width="20" height="14" rx="2" />
                <line x1="8" y1="21" x2="16" y2="21" />
                <line x1="12" y1="17" x2="12" y2="21" />
              </svg>
            </div>
            <h3 className="onboarding-step-heading">Install the Daemon</h3>
            <div className="onboarding-platform-instructions">
              <div className="onboarding-platform" data-testid="onboarding-platform-mac">
                <div className="onboarding-platform-header">
                  <span className="onboarding-platform-name">macOS</span>
                </div>
                <ol className="onboarding-install-steps">
                  <li>Open the downloaded <code>.dmg</code> file</li>
                  <li>Drag the VaultMTG daemon to Applications</li>
                  <li>Launch it — the installer is notarized by Apple</li>
                </ol>
              </div>
              <div className="onboarding-platform" data-testid="onboarding-platform-windows">
                <div className="onboarding-platform-header">
                  <span className="onboarding-platform-name">Windows</span>
                </div>
                <ol className="onboarding-install-steps">
                  <li>Run the downloaded <code>.exe</code> installer</li>
                  <li>Follow the installer wizard (Next &rarr; Finish)</li>
                </ol>
              </div>
            </div>
            <div className="onboarding-step-actions">
              <button type="button" className="onboarding-btn onboarding-btn--ghost" data-testid="onboarding-step-2-back">
                &larr; Back
              </button>
              <button
                type="button"
                className="onboarding-btn onboarding-btn--primary"
                onClick={onComplete}
                data-testid="onboarding-step-2-next"
              >
                I&apos;ve installed it &rarr;
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  ),
};

/**
 * Step 3 — Polling. The modal is on step 3 and actively checking for the
 * daemon connection (spinner + "Waiting for Daemon Connection" heading).
 */
export const Step3Polling: Story = {
  render: ({ onDismiss }) => (
    <div
      className="onboarding-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-modal-title"
      data-testid="onboarding-modal"
    >
      <div className="onboarding-modal">
        <div className="onboarding-modal-header">
          <h2 id="onboarding-modal-title" className="onboarding-modal-title">
            Get Started with VaultMTG
          </h2>
          <button type="button" className="onboarding-modal-close" onClick={onDismiss} aria-label="Dismiss onboarding">
            &times;
          </button>
        </div>

        <div className="onboarding-steps-indicator">
          {[1, 2, 3].map((s) => (
            <div
              key={s}
              className={`onboarding-step-pip ${s === 3 ? 'active' : 'done'}`}
            >
              <span className="onboarding-step-pip-number">{s < 3 ? '✓' : s}</span>
              <span className="onboarding-step-pip-label">
                {s === 1 ? 'Download' : s === 2 ? 'Install' : 'Confirm'}
              </span>
            </div>
          ))}
        </div>

        <div className="onboarding-modal-body">
          <div className="onboarding-step" data-testid="onboarding-step-3">
            <div className="onboarding-step-icon" aria-hidden="true">
              <div className="onboarding-spinner" data-testid="onboarding-spinner" aria-label="Waiting for daemon connection..." />
            </div>
            <h3 className="onboarding-step-heading">Waiting for Daemon Connection</h3>
            <p className="onboarding-step-description">
              Make sure the daemon is running and MTG Arena is open. We&apos;re checking every 5 seconds&hellip;
            </p>
            <div className="onboarding-step-actions">
              <button type="button" className="onboarding-btn onboarding-btn--ghost" onClick={onDismiss} data-testid="onboarding-step-3-back">
                &larr; Back
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  ),
};

/**
 * Step 3 — Success. Daemon connected; green checkmark icon and success heading.
 * After a 2-second delay the live component calls onComplete; in Storybook the
 * static render lets Chromatic capture the success state.
 */
export const Step3Success: Story = {
  render: ({ onDismiss }) => (
    <div
      className="onboarding-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-modal-title"
      data-testid="onboarding-modal"
    >
      <div className="onboarding-modal">
        <div className="onboarding-modal-header">
          <h2 id="onboarding-modal-title" className="onboarding-modal-title">
            Get Started with VaultMTG
          </h2>
          <button type="button" className="onboarding-modal-close" onClick={onDismiss} aria-label="Dismiss onboarding">
            &times;
          </button>
        </div>

        <div className="onboarding-steps-indicator">
          {[1, 2, 3].map((s) => (
            <div key={s} className={`onboarding-step-pip ${s === 3 ? 'active' : 'done'}`}>
              <span className="onboarding-step-pip-number">{s < 3 ? '✓' : s}</span>
              <span className="onboarding-step-pip-label">
                {s === 1 ? 'Download' : s === 2 ? 'Install' : 'Confirm'}
              </span>
            </div>
          ))}
        </div>

        <div className="onboarding-modal-body">
          <div className="onboarding-step" data-testid="onboarding-step-3">
            <div className="onboarding-step-icon onboarding-step-icon--success" aria-hidden="true">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </div>
            <h3 className="onboarding-step-heading" data-testid="onboarding-success-heading">
              Daemon Connected!
            </h3>
            <p className="onboarding-step-description">
              Your daemon is connected and ready. VaultMTG will now track your matches, drafts, and
              collection in real time.
            </p>
          </div>
        </div>
      </div>
    </div>
  ),
};

/**
 * Step 3 — Error (timeout). The daemon was not detected after 2 minutes.
 * Shows the error icon, "Connection Timed Out" heading, and retry/dismiss
 * action buttons.
 */
export const Step3Error: Story = {
  render: ({ onDismiss }) => (
    <div
      className="onboarding-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-modal-title"
      data-testid="onboarding-modal"
    >
      <div className="onboarding-modal">
        <div className="onboarding-modal-header">
          <h2 id="onboarding-modal-title" className="onboarding-modal-title">
            Get Started with VaultMTG
          </h2>
          <button type="button" className="onboarding-modal-close" onClick={onDismiss} aria-label="Dismiss onboarding">
            &times;
          </button>
        </div>

        <div className="onboarding-steps-indicator">
          {[1, 2, 3].map((s) => (
            <div key={s} className={`onboarding-step-pip ${s === 3 ? 'active' : 'done'}`}>
              <span className="onboarding-step-pip-number">{s < 3 ? '✓' : s}</span>
              <span className="onboarding-step-pip-label">
                {s === 1 ? 'Download' : s === 2 ? 'Install' : 'Confirm'}
              </span>
            </div>
          ))}
        </div>

        <div className="onboarding-modal-body">
          <div className="onboarding-step" data-testid="onboarding-step-3">
            <div className="onboarding-step-icon onboarding-step-icon--error" aria-hidden="true">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
            </div>
            <h3 className="onboarding-step-heading" data-testid="onboarding-timeout-heading">
              Connection Timed Out
            </h3>
            <p className="onboarding-step-description">
              We couldn&apos;t detect the daemon after 2 minutes. Make sure the daemon is running, then try again.
            </p>
            <div className="onboarding-step-actions">
              <button type="button" className="onboarding-btn onboarding-btn--ghost" onClick={onDismiss} data-testid="onboarding-step-3-back-timeout">
                &larr; Back
              </button>
              <button type="button" className="onboarding-btn onboarding-btn--primary" data-testid="onboarding-step-3-retry">
                Try Again
              </button>
              <button type="button" className="onboarding-btn onboarding-btn--ghost" onClick={onDismiss} data-testid="onboarding-step-3-dismiss">
                I&apos;ll do this later
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  ),
};

/**
 * Closed — isOpen=false; the modal renders nothing (null). Documented here
 * so Chromatic snapshots confirm the closed state does not emit any visible
 * elements.
 */
export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
