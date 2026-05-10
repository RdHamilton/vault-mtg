/**
 * DaemonEmptyState
 *
 * Renders a first-run empty state for pages that require the local daemon
 * (Match History, Collection, Decks). Shown when the daemon is not connected.
 *
 * Props:
 *   page — identifier used for PostHog analytics (e.g. "match_history")
 *   heading — primary heading text
 *   subtext — supporting body text
 */
import { useEffect, useRef } from 'react';
import EmptyState from './EmptyState';
import { trackEvent } from '@/services/analytics';

export interface DaemonEmptyStateProps {
  /** Page identifier sent in analytics (e.g. "match_history", "collection", "decks") */
  page: string;
  /** Primary heading */
  heading: string;
  /** Supporting body text */
  subtext: string;
}

/**
 * Wraps EmptyState with:
 * - CTA pointing to /setup
 * - PostHog error_empty_state_shown event on mount (once per page load)
 */
const DaemonEmptyState = ({ page, heading, subtext }: DaemonEmptyStateProps) => {
  const firedRef = useRef(false);

  useEffect(() => {
    if (!firedRef.current) {
      firedRef.current = true;
      trackEvent({ name: 'error_empty_state_shown', properties: { page } });
    }
  }, [page]);

  return (
    <div data-testid="daemon-empty-state">
      <EmptyState
        icon="🔌"
        heading={heading}
        subtext={subtext}
        ctaLabel="Go to Setup"
        ctaHref="/setup"
        variant="no-data"
      />
    </div>
  );
};

export default DaemonEmptyState;
