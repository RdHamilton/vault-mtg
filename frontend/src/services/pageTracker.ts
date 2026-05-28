/**
 * Module-level current page tracker for PostHog error events.
 *
 * Provides a shared read/write point for the current page slug so that
 * non-React modules (e.g. apiClient.ts) can include the current page in
 * `error_data_load_failed` without needing React context.
 *
 * PostHogRouteTracker calls setCurrentPage on every route change (including
 * initial mount). apiClient.ts calls getCurrentPage() when building the
 * error_data_load_failed payload.
 *
 * Design decision (Ray Q5): module-level let rather than React context or
 * Zustand so the adapter layer can read it without a React dependency.
 */

/** Current PostHog page slug. Starts as 'unknown'; updated by PostHogRouteTracker. */
let _currentPage: string = 'unknown';

/**
 * Update the module-level current page slug.
 * Called by PostHogRouteTracker on every route change, including initial mount.
 */
export function setCurrentPage(slug: string): void {
  _currentPage = slug;
}

/**
 * Read the module-level current page slug.
 * Used by apiClient.ts to populate the `page` field of error_data_load_failed.
 */
export function getCurrentPage(): string {
  return _currentPage;
}
