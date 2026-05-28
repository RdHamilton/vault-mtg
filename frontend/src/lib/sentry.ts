import * as Sentry from '@sentry/react';

interface ReportErrorOptions {
  component: string;
  action: string;
  extra?: Record<string, unknown>;
}

/**
 * Captures an error to Sentry with component + action tags.
 *
 * Mirrors the BFF `ReportError(ctx, err, tags...)` pattern. Tag keys
 * ("component", "action") are identical to the BFF convention so Sentry
 * dashboards can filter events from both layers with the same facets.
 *
 * Rules:
 * - Purely additive — never rethrows; caller retains its existing error-handling
 * - Nil/undefined err is a no-op
 * - Never pass user-typed content (note bodies, deck names, free text) in `extra`
 */
export function reportError(err: unknown, options: ReportErrorOptions): void {
  if (err === null || err === undefined) return;
  Sentry.captureException(err, {
    tags: {
      component: options.component,
      action: options.action,
    },
    ...(options.extra !== undefined ? { extra: options.extra } : {}),
  });
}
