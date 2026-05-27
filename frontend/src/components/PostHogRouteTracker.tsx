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
import { setCurrentPage } from '@/services/pageTracker';

// ── First-feature guard ───────────────────────────────────────────────────────

const FIRST_FEATURE_KEY = 'vaultmtg_ph_funnel_first_feature_used_fired';

type FirstFeature =
  | 'draft'
  | 'draft_analytics'
  | 'decks'
  | 'collection'
  | 'meta'
  | 'charts'
  | 'quests';

/**
 * Returns the qualifying feature slug for a pathname, or null if the route
 * does not count as a "first feature used" event.
 *
 * /match-history is intentionally excluded — it is the default post-signup
 * view and therefore not a signal of deliberate feature exploration.
 * /home, /setup, /download, /settings, /api-keys, /profile are excluded
 * for the same reason.
 */
function toFirstFeature(pathname: string): FirstFeature | null {
  if (pathname === '/draft' || pathname === '/draft/live') return 'draft';
  if (pathname === '/draft-analytics') return 'draft_analytics';
  if (pathname === '/decks' || pathname.startsWith('/deck-builder')) return 'decks';
  if (pathname === '/collection') return 'collection';
  if (pathname === '/meta') return 'meta';
  if (pathname.startsWith('/charts/')) return 'charts';
  if (pathname === '/quests') return 'quests';
  return null;
}

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
      // Skip initial mount — first value seeds the ref and the module-level page.
      previousPathRef.current = pathname;
      setCurrentPage(toPageSlug(pathname));
      return;
    }

    const previous = previousPathRef.current;
    previousPathRef.current = pathname;
    setCurrentPage(toPageSlug(pathname));

    trackEvent({
      name: 'page_viewed',
      properties: {
        page: toPageSlug(pathname),
        previous_page: toPageSlug(previous),
      },
    });

    // Fire funnel_first_feature_used once per session when the user first
    // navigates to a qualifying feature route. Guarded by sessionStorage so
    // it fires at most once even across route changes within the same tab.
    const feature = toFirstFeature(pathname);
    if (feature !== null && !sessionStorage.getItem(FIRST_FEATURE_KEY)) {
      trackEvent({
        name: 'funnel_first_feature_used',
        properties: { feature },
      });
      sessionStorage.setItem(FIRST_FEATURE_KEY, '1');
    }
  }, [pathname]);

  return null;
}
