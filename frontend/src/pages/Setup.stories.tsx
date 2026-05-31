import type { Meta, StoryObj } from '@storybook/react';
import { withRouter } from '../../.storybook/decorators';
import Setup from './Setup';
import './Setup.css';

/**
 * Setup — the daemon installation and PKCE-pairing page, rendered at `/setup`.
 *
 * The page polls `http://localhost:9001/health` to detect when PKCE pairing
 * completes. In Storybook, `isDesktopApp()` returns false, so polling is
 * skipped entirely (no localhost network noise, no ERR_CONNECTION_REFUSED
 * errors). The component renders in the "waiting" pairing state by default.
 *
 * Decorators:
 *   - withRouter — required because Setup calls `useNavigate()` to redirect
 *     after pairing succeeds.
 *
 * States covered:
 *   - Default (waiting) — polling not started (non-desktop context), shows the
 *     full 3-step install guide with the "Waiting for auth..." spinner
 *   - PairingSuccess    — static render of the success state
 *   - PairingError      — static render of the timeout/error state with Retry button
 *
 * The platform-specific warning sections (GatekeeperWarning / SmartScreenWarning)
 * are always rendered in Storybook because `detectPlatform()` returns 'unknown'
 * in the jsdom/browser Storybook environment, which triggers the "show both"
 * branch.
 */
const meta: Meta<typeof Setup> = {
  title: 'Organisms/Setup',
  component: Setup,
  decorators: [withRouter],
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof Setup>;

/**
 * Default (Waiting) — the component as it renders in Storybook. Polling is
 * skipped because `isDesktopApp()` is false in the browser context, so the
 * pairing status shows the "Waiting for auth..." spinner. Both platform warning
 * sections are visible (unknown platform → show both).
 */
export const Default: Story = {};

/**
 * PairingSuccess — the success state shown after the daemon PKCE pairing
 * completes. Rendered via static HTML using the component's own CSS classes
 * to give Chromatic a stable snapshot without requiring a real daemon.
 */
export const PairingSuccess: Story = {
  render: () => (
    <div className="setup-container" data-testid="setup-container">
      <div className="setup-header">
        <h1 className="setup-title">Install the VaultMTG Daemon</h1>
        <p className="setup-subtitle">
          The VaultMTG daemon runs in the background while you play MTG Arena and syncs your match
          history, draft picks, and collection to your account.
        </p>
      </div>

      <section className="setup-section" data-testid="setup-download-section">
        <h2 className="setup-section-title">Step 1 — Download</h2>
        <p className="setup-section-body">
          Download the installer for your platform from the{' '}
          <a href="/download" className="setup-link" target="_blank" rel="noopener noreferrer">
            download page
          </a>
          , then run it to install the daemon.
        </p>
      </section>

      <section className="setup-section" data-testid="setup-warnings-section">
        <h2 className="setup-section-title">Step 2 — Install</h2>
        <p className="setup-section-body">
          The VaultMTG daemon is signed and notarized for both macOS and Windows. No security bypass
          is required.
        </p>
      </section>

      <section className="setup-section" data-testid="setup-pairing-section">
        <h2 className="setup-section-title">Step 3 — Sign In</h2>
        <p className="setup-section-body">
          Once the daemon is installed and running, it will open your browser to complete sign-in.
          Your VaultMTG account will be linked automatically.
        </p>
        <div className="setup-pairing" data-testid="pairing-status">
          <div className="setup-pairing-success" data-testid="pairing-success">
            <span className="setup-pairing-checkmark" aria-hidden="true">&#10003;</span>
            <p className="setup-pairing-label">Auth complete — redirecting...</p>
          </div>
        </div>
      </section>
    </div>
  ),
};

/**
 * PairingError — the timeout/error state shown when the daemon was not detected
 * within the 60-second window. Shows the error message and a Retry button.
 */
export const PairingError: Story = {
  render: () => (
    <div className="setup-container" data-testid="setup-container">
      <div className="setup-header">
        <h1 className="setup-title">Install the VaultMTG Daemon</h1>
        <p className="setup-subtitle">
          The VaultMTG daemon runs in the background while you play MTG Arena and syncs your match
          history, draft picks, and collection to your account.
        </p>
      </div>

      <section className="setup-section" data-testid="setup-download-section">
        <h2 className="setup-section-title">Step 1 — Download</h2>
        <p className="setup-section-body">
          Download the installer for your platform from the{' '}
          <a href="/download" className="setup-link" target="_blank" rel="noopener noreferrer">
            download page
          </a>
          , then run it to install the daemon.
        </p>
      </section>

      <section className="setup-section" data-testid="setup-warnings-section">
        <h2 className="setup-section-title">Step 2 — Install</h2>
        <p className="setup-section-body">
          The VaultMTG daemon is signed and notarized for both macOS and Windows. No security bypass
          is required.
        </p>
      </section>

      <section className="setup-section" data-testid="setup-pairing-section">
        <h2 className="setup-section-title">Step 3 — Sign In</h2>
        <p className="setup-section-body">
          Once the daemon is installed and running, it will open your browser to complete sign-in.
          Your VaultMTG account will be linked automatically.
        </p>
        <div className="setup-pairing" data-testid="pairing-status">
          <div className="setup-pairing-error" data-testid="pairing-error">
            <p className="setup-pairing-label setup-pairing-label--error">
              Setup timed out. Make sure the VaultMTG daemon is running and try again.
            </p>
            <button className="setup-retry-button" data-testid="retry-button">
              Retry
            </button>
          </div>
        </div>
      </section>
    </div>
  ),
};
