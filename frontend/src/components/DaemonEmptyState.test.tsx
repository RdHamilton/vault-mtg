/**
 * DaemonEmptyState component tests
 *
 * Tests rendering, CTA, and PostHog analytics event firing.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import DaemonEmptyState from './DaemonEmptyState';

// Mock analytics module to capture trackEvent calls
vi.mock('@/services/analytics', () => ({
  trackEvent: vi.fn(),
}));

import { trackEvent } from '@/services/analytics';

const renderComponent = (page: string, heading: string, subtext: string) => {
  return render(
    <MemoryRouter>
      <DaemonEmptyState page={page} heading={heading} subtext={subtext} />
    </MemoryRouter>
  );
};

describe('DaemonEmptyState', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with data-testid="daemon-empty-state"', () => {
    renderComponent('match_history', 'Daemon not connected', 'Install the daemon to continue.');
    expect(screen.getByTestId('daemon-empty-state')).toBeInTheDocument();
  });

  it('renders the heading', () => {
    renderComponent('match_history', 'Daemon not connected', 'Install the daemon to continue.');
    expect(screen.getByText('Daemon not connected')).toBeInTheDocument();
  });

  it('renders the subtext', () => {
    renderComponent('collection', 'Daemon offline', 'Start the daemon to sync your collection.');
    expect(screen.getByText('Start the daemon to sync your collection.')).toBeInTheDocument();
  });

  it('renders the CTA link pointing to /setup', () => {
    renderComponent('decks', 'Daemon not connected', 'Install the daemon.');
    const cta = screen.getByRole('link', { name: /go to setup/i });
    expect(cta).toBeInTheDocument();
    expect(cta).toHaveAttribute('href', '/setup');
  });

  it('fires error_empty_state_shown analytics event on mount', async () => {
    renderComponent('match_history', 'Daemon not connected', 'Install the daemon.');
    await waitFor(() => {
      expect(trackEvent).toHaveBeenCalledWith({
        name: 'error_empty_state_shown',
        properties: { page: 'match_history' },
      });
    });
  });

  it('fires analytics event with the correct page identifier', async () => {
    renderComponent('collection', 'Daemon not connected', 'Install the daemon.');
    await waitFor(() => {
      expect(trackEvent).toHaveBeenCalledWith({
        name: 'error_empty_state_shown',
        properties: { page: 'collection' },
      });
    });
  });

  it('fires the analytics event only once even on re-render', async () => {
    const { rerender } = renderComponent('decks', 'Daemon not connected', 'Install the daemon.');
    rerender(
      <MemoryRouter>
        <DaemonEmptyState page="decks" heading="Daemon not connected" subtext="Install the daemon." />
      </MemoryRouter>
    );
    await waitFor(() => {
      expect(trackEvent).toHaveBeenCalledTimes(1);
    });
  });

  it('renders the plug icon', () => {
    renderComponent('match_history', 'Daemon not connected', 'Install the daemon.');
    // The EmptyState renders the icon character as text
    expect(screen.getByText('🔌')).toBeInTheDocument();
  });
});
