import type { Meta, StoryObj } from '@storybook/react';
import './Footer.css';

/**
 * Footer — the stats bar rendered at the bottom of every page.
 *
 * The component fetches match statistics and daemon sync state from the BFF at
 * mount time, so it cannot be rendered in isolation without mocking those API
 * calls. The approach used here follows the same pattern as other story files
 * in this repo: render a static HTML representation of each key state using
 * the component's own CSS classes, giving Chromatic stable snapshots.
 *
 * States covered:
 *  - Loading  — footer shows "Loading stats..."
 *  - Empty    — no matches played yet
 *  - WithStats — a typical stats bar (matches, win rate, streak, last played,
 *               synced time)
 *
 * If Footer is refactored to accept dependency-injection props (as the Profile
 * and UserProfileSection components do), these render-based stories can be
 * replaced with proper args-based stories.
 */
const meta: Meta = {
  title: 'Organisms/Footer',
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj;

/** Loading state — shown immediately on mount while stats are fetched. */
export const Loading: Story = {
  render: () => (
    <footer className="app-footer" data-testid="footer">
      <div className="footer-content">
        <span className="footer-loading">Loading stats...</span>
      </div>
    </footer>
  ),
};

/**
 * Empty state — the user has not played any matches yet. The footer prompts
 * them to play to start seeing stats.
 */
export const Empty: Story = {
  render: () => (
    <footer className="app-footer" data-testid="footer">
      <div className="footer-content">
        <span className="footer-empty">
          No matches yet - play some games to see your stats!
        </span>
      </div>
    </footer>
  ),
};

/**
 * WithStats — a populated stats bar representative of a typical user session.
 * Shows total matches, win rate, current win streak, last played time, and
 * last daemon sync time.
 */
export const WithStats: Story = {
  render: () => (
    <footer className="app-footer" data-testid="footer">
      <div className="footer-content">
        <span className="footer-label">All Time</span>
        <span className="footer-separator">·</span>
        <span className="footer-stat">
          <strong>Matches:</strong>{' '}
          <span className="footer-num">142</span>
        </span>
        <span className="footer-separator">·</span>
        <span className="footer-stat">
          <strong>Win Rate:</strong>{' '}
          <span className="footer-num">58.5% (83-59)</span>
        </span>
        <span className="footer-separator">·</span>
        <span className="footer-stat streak-w">
          <strong>Streak:</strong>{' '}
          <span className="footer-num">W4</span>
        </span>
        <span className="footer-separator footer-separator-push">·</span>
        <span className="footer-stat footer-last-match">
          <strong>Last Played:</strong>{' '}
          <span className="footer-num">5/31/2026, 9:45:00 PM</span>
        </span>
        <span className="footer-separator">·</span>
        <span className="footer-stat footer-last-synced">
          <strong>Synced:</strong>{' '}
          <span className="footer-num">9:47:12 PM</span>
        </span>
      </div>
    </footer>
  ),
};

/**
 * LossStreak — same stats bar layout but with a loss streak to document the
 * `streak-l` CSS class for Chromatic.
 */
export const LossStreak: Story = {
  render: () => (
    <footer className="app-footer" data-testid="footer">
      <div className="footer-content">
        <span className="footer-label">All Time</span>
        <span className="footer-separator">·</span>
        <span className="footer-stat">
          <strong>Matches:</strong>{' '}
          <span className="footer-num">37</span>
        </span>
        <span className="footer-separator">·</span>
        <span className="footer-stat">
          <strong>Win Rate:</strong>{' '}
          <span className="footer-num">43.2% (16-21)</span>
        </span>
        <span className="footer-separator">·</span>
        <span className="footer-stat streak-l">
          <strong>Streak:</strong>{' '}
          <span className="footer-num">L3</span>
        </span>
        <span className="footer-separator footer-separator-push">·</span>
        <span className="footer-stat footer-last-match">
          <strong>Last Played:</strong>{' '}
          <span className="footer-num">5/31/2026, 8:22:00 PM</span>
        </span>
      </div>
    </footer>
  ),
};
