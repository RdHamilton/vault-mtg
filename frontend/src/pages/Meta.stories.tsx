import type { Meta, StoryObj } from '@storybook/react';
import './Meta.css';

/**
 * Meta — the Metagame Dashboard page at `/meta`.
 *
 * The component fetches archetype data from the BFF at mount time via
 * `meta.getMetaArchetypes`, so it cannot be rendered in isolation without
 * mocking the API call.  Following the same pattern as Footer.stories.tsx,
 * each story renders a static HTML representation using the component's own
 * CSS classes so Chromatic gets stable snapshots across CI runs.
 *
 * States covered:
 *  - Populated   — Standard format with two Tier 1 archetypes rendered
 *  - EmptySupported — Supported format (Standard) with no data yet
 *    (`data-empty-reason="format_supported_no_data"`)
 *  - EmptyUnsupported — A hypothetical unsupported format
 *    (`data-empty-reason="format_unsupported"`)
 *  - Loading     — spinner shown while the initial fetch is in-flight
 *  - ErrorState  — API call failed, error banner shown
 *
 * If Meta is refactored to accept dependency-injection props for its API
 * service, these render-based stories can be replaced with args-based stories.
 */
const meta: Meta = {
  title: 'Organisms/Meta',
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj;

/** Populated — Standard format with archetypes, sources, and tier badges. */
export const Populated: Story = {
  render: () => (
    <div className="meta-page">
      <div className="meta-header">
        <div className="meta-title">
          <h1>Metagame Dashboard</h1>
          <p className="meta-description">Current metagame data from MTGGoldfish and MTGTop8</p>
        </div>
        <div className="meta-controls">
          <select className="format-select" defaultValue="standard">
            <option value="standard">Standard</option>
            <option value="historic">Historic</option>
            <option value="explorer">Explorer</option>
            <option value="pioneer">Pioneer</option>
            <option value="modern">Modern</option>
          </select>
          <button className="refresh-button">⟳ Refresh</button>
        </div>
      </div>

      <div className="meta-content">
        <div className="meta-summary">
          <div className="summary-stat">
            <span className="stat-value">2</span>
            <span className="stat-label">Archetypes</span>
          </div>
          <div className="summary-stat">
            <span className="stat-value">0</span>
            <span className="stat-label">Recent Tournaments</span>
          </div>
          <div className="summary-stat">
            <span className="stat-value">MTGGoldfish, MTGTop8</span>
            <span className="stat-label">Data Sources</span>
          </div>
          <div className="summary-stat">
            <span className="stat-value">Today</span>
            <span className="stat-label">Last Updated</span>
          </div>
        </div>

        <div className="tier-lists">
          <div className="tier-section tier-1-section">
            <h2 className="tier-header">
              <span className="tier-badge tier-1">Tier 1</span>
              <span className="tier-count">(2 decks)</span>
            </h2>
            <div className="archetype-list">
              <div className="archetype-card" role="button" tabIndex={0}>
                <div className="archetype-header">
                  <span className="archetype-name">Mono Red Aggro</span>
                  <span className="color-badge">
                    <span className="color-pip color-r" title="R">R</span>
                  </span>
                  <span className="trend-icon trend-up" title="Trending up">↗</span>
                </div>
                <div className="archetype-stats">
                  <div className="stat-item">
                    <span className="stat-icon">📊</span>
                    <span className="stat-text">15.5% meta share</span>
                  </div>
                  <div className="stat-item">
                    <span className="stat-icon">🏆</span>
                    <span className="stat-text">12 Top 8s</span>
                  </div>
                  <div className="stat-item">
                    <span className="stat-icon">🥇</span>
                    <span className="stat-text">3 Wins</span>
                  </div>
                </div>
              </div>
              <div className="archetype-card" role="button" tabIndex={0}>
                <div className="archetype-header">
                  <span className="archetype-name">Azorius Control</span>
                  <span className="color-badge">
                    <span className="color-pip color-w" title="W">W</span>
                    <span className="color-pip color-u" title="U">U</span>
                  </span>
                  <span className="trend-icon trend-stable" title="Stable">→</span>
                </div>
                <div className="archetype-stats">
                  <div className="stat-item">
                    <span className="stat-icon">📊</span>
                    <span className="stat-text">10.2% meta share</span>
                  </div>
                  <div className="stat-item">
                    <span className="stat-icon">🏆</span>
                    <span className="stat-text">8 Top 8s</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  ),
};

/**
 * EmptySupported — Standard (a supported format) with no archetype data yet.
 * The empty state copy tells the user data is "coming soon" and offers a
 * Refresh button.  `data-empty-reason="format_supported_no_data"` signals to
 * tests and monitoring that this is a transient, not permanent, empty state.
 */
export const EmptySupported: Story = {
  render: () => (
    <div className="meta-page">
      <div className="meta-header">
        <div className="meta-title">
          <h1>Metagame Dashboard</h1>
          <p className="meta-description">Current metagame data from MTGGoldfish and MTGTop8</p>
        </div>
        <div className="meta-controls">
          <select className="format-select" defaultValue="standard">
            <option value="standard">Standard</option>
          </select>
          <button className="refresh-button">⟳ Refresh</button>
        </div>
      </div>

      <div className="meta-content">
        <div className="meta-summary">
          <div className="summary-stat">
            <span className="stat-value">0</span>
            <span className="stat-label">Archetypes</span>
          </div>
          <div className="summary-stat">
            <span className="stat-value">0</span>
            <span className="stat-label">Recent Tournaments</span>
          </div>
          <div className="summary-stat">
            <span className="stat-value">MTGGoldfish, MTGTop8</span>
            <span className="stat-label">Data Sources</span>
          </div>
        </div>

        <div className="no-data" data-empty-reason="format_supported_no_data">
          <div className="no-data-icon">⏳</div>
          <h3>Metagame Data Coming Soon</h3>
          <p>
            <strong>Standard</strong> is a supported format. Archetype data hasn&apos;t been scraped
            yet — check back after the next data refresh, or trigger one manually.
          </p>
          <button className="retry-button">Refresh Now</button>
        </div>
      </div>
    </div>
  ),
};

/**
 * EmptyUnsupported — a format not tracked by our data sources.
 * The empty state copy tells the user the format is not supported.
 * `data-empty-reason="format_unsupported"` distinguishes this from the
 * supported-but-empty case so monitoring can alert on the right condition.
 */
export const EmptyUnsupported: Story = {
  render: () => (
    <div className="meta-page">
      <div className="meta-header">
        <div className="meta-title">
          <h1>Metagame Dashboard</h1>
          <p className="meta-description">Current metagame data from MTGGoldfish and MTGTop8</p>
        </div>
        <div className="meta-controls">
          <select className="format-select" defaultValue="alchemy">
            <option value="alchemy">Alchemy</option>
          </select>
          <button className="refresh-button">⟳ Refresh</button>
        </div>
      </div>

      <div className="meta-content">
        <div className="meta-summary">
          <div className="summary-stat">
            <span className="stat-value">0</span>
            <span className="stat-label">Archetypes</span>
          </div>
        </div>

        <div className="no-data" data-empty-reason="format_unsupported">
          <div className="no-data-icon">🚫</div>
          <h3>Format Not Supported</h3>
          <p>
            Metagame data is not available for <strong>alchemy</strong>. Our data sources
            (MTGGoldfish, MTGTop8) do not cover this format.
          </p>
        </div>
      </div>
    </div>
  ),
};

/** Loading — spinner shown while the initial archetype fetch is in-flight. */
export const Loading: Story = {
  render: () => (
    <div className="meta-page">
      <div className="meta-header">
        <div className="meta-title">
          <h1>Metagame Dashboard</h1>
          <p className="meta-description">Current metagame data from MTGGoldfish and MTGTop8</p>
        </div>
        <div className="meta-controls">
          <select className="format-select" defaultValue="standard" disabled>
            <option value="standard">Standard</option>
          </select>
          <button className="refresh-button" disabled>⟳ Refresh</button>
        </div>
      </div>

      <div className="meta-loading">
        <div className="loading-spinner" />
        <span>Loading meta data for standard...</span>
      </div>
    </div>
  ),
};

/** ErrorState — API call failed; error banner is shown above the content area. */
export const ErrorState: Story = {
  render: () => (
    <div className="meta-page">
      <div className="meta-header">
        <div className="meta-title">
          <h1>Metagame Dashboard</h1>
          <p className="meta-description">Current metagame data from MTGGoldfish and MTGTop8</p>
        </div>
        <div className="meta-controls">
          <select className="format-select" defaultValue="standard">
            <option value="standard">Standard</option>
          </select>
          <button className="refresh-button">⟳ Refresh</button>
        </div>
      </div>

      <div className="meta-error">
        <strong>Error:</strong> Failed to fetch: connection refused
      </div>
    </div>
  ),
};
