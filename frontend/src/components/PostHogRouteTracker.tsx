/**
 * PostHogRouteTracker — fires a `page_viewed` event on every React Router
 * route change, skipping the initial mount.
 *
 * Mount this once inside <Router> in App.tsx. It renders nothing.
 *
 * Slug map from posthog-spec-2026-05-10.md § A2:
 *   /home                         → home
 *   /match-history                → match_history
 *   /quests                       → quests
 *   /draft                        → draft_advisor
 *   /draft-analytics              → draft_analytics
 *   /decks                        → decks
 *   /deck-builder/*               → deck_builder
 *   /collection                   → collection
 *   /meta                         → meta
 *   /charts/win-rate-trend        → chart_win_rate
 *   /charts/deck-performance      → chart_deck_performance
 *   /charts/rank-progression      → chart_rank_progression
 *   /charts/format-distribution   → chart_format_distribution
 *   /charts/result-breakdown      → chart_result_breakdown
 *   /settings                     → settings
 *   /history/drafts               → bff_draft_history
 *   /draft/live                   → draft_live
 *   /api-keys                     → api_keys
 *   /profile                      → profile
 *   /download                     → download
 *   /setup                        → setup
 *   (unknown)                     → unknown
 */
import { useEffect, useRef } from 'react';
import { useLocation } from 'react-router-dom';
import { trackEvent } from '@/services/analytics';

/** Map a pathname to the canonical PostHog page slug. */
function toPageSlug(pathname: string): string {
  if (pathname === '/' || pathname === '/home') return 'home';
  if (pathname === '/match-history') return 'match_history';
  if (pathname === '/quests') return 'quests';
  if (pathname === '/draft-analytics') return 'draft_analytics';
  if (pathname === '/draft/live') return 'draft_live';
  if (pathname === '/draft') return 'draft_advisor';
  if (pathname.startsWith('/deck-builder')) return 'deck_builder';
  if (pathname === '/decks') return 'decks';
  if (pathname === '/collection') return 'collection';
  if (pathname === '/meta') return 'meta';
  if (pathname === '/charts/win-rate-trend') return 'chart_win_rate';
  if (pathname === '/charts/deck-performance') return 'chart_deck_performance';
  if (pathname === '/charts/rank-progression') return 'chart_rank_progression';
  if (pathname === '/charts/format-distribution') return 'chart_format_distribution';
  if (pathname === '/charts/result-breakdown') return 'chart_result_breakdown';
  if (pathname === '/settings') return 'settings';
  if (pathname === '/history/drafts') return 'bff_draft_history';
  if (pathname === '/api-keys') return 'api_keys';
  if (pathname === '/profile') return 'profile';
  if (pathname === '/download') return 'download';
  if (pathname === '/setup') return 'setup';
  return 'unknown';
}

export function PostHogRouteTracker(): null {
  const { pathname } = useLocation();
  // previousPathRef stores the last pathname so we can pass previous_page.
  // Starts as null — first render does not fire (skip-first rule).
  const previousPathRef = useRef<string | null>(null);

  useEffect(() => {
    if (previousPathRef.current === null) {
      // Skip initial mount — first value seeds the ref.
      previousPathRef.current = pathname;
      return;
    }

    const previous = previousPathRef.current;
    previousPathRef.current = pathname;

    trackEvent({
      name: 'page_viewed',
      properties: {
        page: toPageSlug(pathname),
        previous_page: toPageSlug(previous),
      },
    });
  }, [pathname]);

  return null;
}
