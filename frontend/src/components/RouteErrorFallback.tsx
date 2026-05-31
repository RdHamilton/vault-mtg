/**
 * RouteErrorFallback — shared fallback UI rendered by per-route Sentry.ErrorBoundary instances.
 *
 * Rendered when a route's render tree throws an unhandled error.
 * Sentry.ErrorBoundary captures and reports the error automatically via its
 * built-in onError mechanism before showing this component — no manual
 * captureException call is needed here.
 *
 * The top-level Sentry.ErrorBoundary in main.tsx remains the last-resort catch
 * for errors outside the route tree (e.g., ClerkProvider or AppProvider failures).
 */

// Props intentionally kept minimal — the error object is not displayed to the user.
// Sentry.ErrorBoundary captures and reports the error internally before rendering this fallback.
export function RouteErrorFallback() {
  return (
    <div
      data-testid="route-error-fallback"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '200px',
        padding: '2rem',
        gap: '1rem',
        textAlign: 'center',
      }}
    >
      <p style={{ fontSize: '1rem', color: 'var(--fg-secondary, #9ca3af)' }}>
        This page encountered an error.
      </p>
      <button
        onClick={() => window.location.reload()}
        style={{
          padding: '0.5rem 1.25rem',
          borderRadius: '6px',
          border: '1px solid var(--border, #374151)',
          background: 'var(--bg-raised, #1f2937)',
          color: 'var(--fg, #f9fafb)',
          cursor: 'pointer',
          fontSize: '0.875rem',
        }}
      >
        Reload page
      </button>
    </div>
  )
}

export default RouteErrorFallback
