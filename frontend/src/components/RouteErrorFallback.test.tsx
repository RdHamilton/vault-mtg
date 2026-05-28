/**
 * Tests for RouteErrorFallback — the shared fallback UI for per-route Sentry.ErrorBoundary.
 *
 * Covers:
 * (a) Snapshot — catches accidental UI regression of the fallback component
 * (b) Per-route boundary isolation — a throw inside one route renders the fallback;
 *     a sibling component (in a separate boundary) is unaffected
 * (c) Reload button is present and clickable
 *
 * NOTE: We do NOT test live Sentry SDK calls here. Sentry.ErrorBoundary's capture
 * behaviour is an integration concern and is not asserted in unit tests.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import React from 'react'
import { RouteErrorFallback } from './RouteErrorFallback'

// ---------------------------------------------------------------------------
// Minimal class-based ErrorBoundary used in isolation tests.
// Mirrors the structure Sentry.ErrorBoundary wraps around internally.
// ---------------------------------------------------------------------------
class TestBoundary extends React.Component<
  { children: React.ReactNode; fallback: React.ReactNode },
  { hasError: boolean }
> {
  constructor(props: { children: React.ReactNode; fallback: React.ReactNode }) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  render() {
    if (this.state.hasError) {
      return <>{this.props.fallback}</>
    }
    return <>{this.props.children}</>
  }
}

// Always-throwing component used to trigger error boundaries in tests.
function Bomb() {
  throw new Error('test explosion from Bomb')
}

// ---------------------------------------------------------------------------
// (a) Snapshot
// ---------------------------------------------------------------------------
describe('RouteErrorFallback', () => {
  it('matches snapshot', () => {
    const { container } = render(<RouteErrorFallback />)
    expect(container.firstChild).toMatchSnapshot()
  })

  // -------------------------------------------------------------------------
  // (b) Per-route isolation — a throw in one boundary does not affect sibling
  // -------------------------------------------------------------------------
  describe('per-route boundary isolation', () => {
    const originalConsoleError = console.error

    beforeEach(() => {
      // Suppress expected React error boundary noise in test output.
      console.error = vi.fn()
    })

    afterEach(() => {
      console.error = originalConsoleError
    })

    it('renders RouteErrorFallback when wrapped component throws', () => {
      render(
        <TestBoundary fallback={<RouteErrorFallback />}>
          <Bomb />
        </TestBoundary>,
      )
      expect(screen.getByTestId('route-error-fallback')).toBeInTheDocument()
      expect(screen.getByText('This page encountered an error.')).toBeInTheDocument()
    })

    it('does not show fallback when wrapped component does not throw', () => {
      render(
        <TestBoundary fallback={<RouteErrorFallback />}>
          <p data-testid="healthy-child">All good</p>
        </TestBoundary>,
      )
      expect(screen.getByTestId('healthy-child')).toBeInTheDocument()
      expect(screen.queryByTestId('route-error-fallback')).not.toBeInTheDocument()
    })

    it('sibling boundary is unaffected when one boundary catches an error', () => {
      // Two independent boundaries rendered side-by-side.
      // The first boundary throws; the second renders its child normally.
      render(
        <div>
          <TestBoundary fallback={<RouteErrorFallback />}>
            <Bomb />
          </TestBoundary>
          <TestBoundary fallback={<RouteErrorFallback />}>
            <p data-testid="sibling-child">Sibling is healthy</p>
          </TestBoundary>
        </div>,
      )

      // Throwing boundary shows the fallback.
      expect(screen.getByTestId('route-error-fallback')).toBeInTheDocument()
      // Sibling boundary shows its healthy child — not blanked out.
      expect(screen.getByTestId('sibling-child')).toBeInTheDocument()
    })
  })

  // -------------------------------------------------------------------------
  // (c) Reload button
  // -------------------------------------------------------------------------
  describe('reload button', () => {
    it('renders a "Reload page" button', () => {
      render(<RouteErrorFallback />)
      expect(screen.getByRole('button', { name: /reload page/i })).toBeInTheDocument()
    })

    it('calls window.location.reload when the button is clicked', () => {
      const reloadMock = vi.fn()
      Object.defineProperty(window, 'location', {
        writable: true,
        value: { ...window.location, reload: reloadMock },
      })

      render(<RouteErrorFallback />)
      fireEvent.click(screen.getByRole('button', { name: /reload page/i }))
      expect(reloadMock).toHaveBeenCalledOnce()
    })
  })
})
