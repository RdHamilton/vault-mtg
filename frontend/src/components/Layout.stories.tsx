import type { Meta, StoryObj } from '@storybook/react';
import { MemoryRouter } from 'react-router-dom';
import './Layout.css';
import './AuthBar.css';
import './DaemonHealthIndicator.css';

/**
 * Layout — the top-level application shell rendered on every page load.
 *
 * Renders the tab-bar (brand logo + nav links + AuthBar + DaemonHealthIndicator),
 * optional sub-navigation (draft / charts routes), main content area, and Footer.
 *
 * The live Layout component fetches data via DaemonHealthIndicator (BFF health
 * poll) and Footer (match stats), both of which are network-dependent. These
 * stories render the shell's HTML structure directly using the component's own
 * CSS classes, giving Chromatic stable offline snapshots while accurately
 * capturing every visual state.
 *
 * If Layout is later refactored to accept DI props for its sub-components,
 * these render-based stories can be replaced with proper args-based stories
 * that mount the actual component.
 *
 * States covered:
 *   - SignedIn        — nav shell with UserButton, connected daemon (green dot)
 *   - SignedOut       — nav shell with sign-in/sign-up buttons, no daemon dot
 *   - ActiveTab       — Home tab highlighted as active
 *   - DraftSubNav     — draft sub-navigation bar visible
 *   - DaemonLoading   — gray indicator dot (checking connection state)
 */
const meta: Meta = {
  title: 'Organisms/Layout',
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj;

const NAV_LINKS = [
  { label: 'Home', href: '/home' },
  { label: 'Match History', href: '/match-history' },
  { label: 'Quests', href: '/quests' },
  { label: 'Draft', href: '/draft' },
  { label: 'Decks', href: '/decks' },
  { label: 'Collection', href: '/collection' },
  { label: 'Meta', href: '/meta' },
  { label: 'Charts', href: '/charts/win-rate-trend' },
  { label: 'Download', href: '/download' },
  { label: 'Profile', href: '/profile' },
  { label: 'Settings', href: '/settings' },
];

function NavShell({
  activeTab = '',
  signedIn = true,
  daemonStatus = 'loading',
  children,
}: {
  activeTab?: string;
  signedIn?: boolean;
  daemonStatus?: 'connected' | 'disconnected' | 'loading';
  children?: React.ReactNode;
}) {
  const dotClass =
    daemonStatus === 'connected'
      ? 'daemon-health-indicator daemon-health-connected'
      : daemonStatus === 'disconnected'
        ? 'daemon-health-indicator daemon-health-disconnected'
        : 'daemon-health-indicator daemon-health-loading';

  return (
    <div className="app-container" data-testid="app-container">
      <div className="tab-bar" data-testid="nav-tab-bar">
        <div className="tab-bar-left">
          <a href="/home" className="nav-brand" data-testid="nav-brand" aria-label="VaultMTG home">
            <span className="nav-brand-wordmark">VaultMTG</span>
          </a>
          <div className="tab-links">
            {NAV_LINKS.map(({ label, href }) => (
              <a
                key={href}
                href={href}
                className={`tab${activeTab === label ? ' active' : ''}`}
                data-testid={`nav-tab-${label.toLowerCase().replace(/\s+/g, '-')}`}
              >
                {label}
              </a>
            ))}
          </div>
        </div>
        <div className="tab-bar-right">
          <div className="auth-bar" data-testid="auth-bar">
            {signedIn ? (
              <div className="auth-bar-signed-in" data-testid="auth-signed-in">
                <div
                  aria-label="User menu (Storybook mock)"
                  style={{
                    width: 32,
                    height: 32,
                    borderRadius: '50%',
                    background: 'var(--accent)',
                    color: '#fff',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 14,
                    fontWeight: 600,
                  }}
                >
                  P
                </div>
              </div>
            ) : (
              <div className="auth-bar-signed-out" data-testid="auth-signed-out">
                <button className="auth-btn auth-btn-signin" data-testid="sign-in-btn">
                  Sign In
                </button>
                <button className="auth-btn auth-btn-signup" data-testid="sign-up-btn">
                  Sign Up
                </button>
              </div>
            )}
          </div>
          <div className="connection-status-indicator">
            <div className={dotClass} title="Daemon status (Storybook mock)" />
          </div>
        </div>
      </div>

      {children}

      <div className="content" data-testid="main-content" style={{ flex: 1, padding: 'var(--space-6)' }}>
        <p style={{ color: 'var(--fg-muted)', fontSize: 'var(--text-sm)' }}>[page content slot]</p>
      </div>

      <footer className="app-footer">
        <div className="footer-content">
          <span className="footer-label">All Time</span>
          <span className="footer-separator">·</span>
          <span className="footer-stat">
            <strong>Matches:</strong> <span className="footer-num">142</span>
          </span>
          <span className="footer-separator">·</span>
          <span className="footer-stat">
            <strong>Win Rate:</strong> <span className="footer-num">58.5% (83-59)</span>
          </span>
        </div>
      </footer>
    </div>
  );
}

/**
 * SignedIn — the default shell state: authenticated user, Home tab active,
 * daemon connection indicator green.
 */
export const SignedIn: Story = {
  render: () => (
    <NavShell activeTab="Home" signedIn daemonStatus="connected" />
  ),
};

/**
 * SignedOut — signed-out user: sign-in + sign-up buttons in the tab-bar
 * right section, no daemon indicator.
 */
export const SignedOut: Story = {
  render: () => (
    <NavShell activeTab="" signedIn={false} daemonStatus="disconnected" />
  ),
};

/**
 * ActiveTab — Match History tab highlighted, demonstrating the active CSS class.
 */
export const ActiveTab: Story = {
  render: () => (
    <NavShell activeTab="Match History" signedIn daemonStatus="connected" />
  ),
};

/**
 * DraftSubNav — shows the sub-navigation bar rendered below the main tab-bar
 * when the Draft tab is active.
 */
export const DraftSubNav: Story = {
  render: () => (
    <NavShell activeTab="Draft" signedIn daemonStatus="connected">
      <div className="sub-tab-bar" data-testid="draft-sub-tab-bar">
        <a href="/draft" className="sub-tab active" data-testid="sub-tab-current-draft">
          Current Draft
        </a>
        <a href="/draft-analytics" className="sub-tab" data-testid="sub-tab-analytics">
          Analytics
        </a>
      </div>
    </NavShell>
  ),
};

/**
 * DaemonLoading — daemon connection indicator in the gray "checking" state,
 * shown on initial page load before the first health poll completes.
 */
export const DaemonLoading: Story = {
  render: () => (
    <NavShell activeTab="Home" signedIn daemonStatus="loading" />
  ),
};
