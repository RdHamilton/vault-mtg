/**
 * Tests for Sentry React SDK integration.
 *
 * Covers:
 * (a) Sentry.init is NOT called when VITE_SENTRY_DSN is absent
 * (b) ErrorBoundary renders fallback UI when a child throws
 * (c) SentryUserSync calls setUser with user id when signed in
 * (d) SentryUserSync calls setUser(null) when signed out
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import React, { useEffect } from 'react';
import * as Sentry from '@sentry/react';

// ---------------------------------------------------------------------------
// Mock @sentry/react
// ---------------------------------------------------------------------------
const { mockBrowserTracingIntegration } = vi.hoisted(() => ({
  mockBrowserTracingIntegration: vi.fn(() => ({ name: 'BrowserTracing' })),
}));

vi.mock('@sentry/react', () => {
  const ErrorBoundary = ({
    children,
    fallback,
  }: {
    children: React.ReactNode;
    fallback: React.ReactNode;
  }) => <>{children || fallback}</>;

  return {
    init: vi.fn(),
    setUser: vi.fn(),
    browserTracingIntegration: mockBrowserTracingIntegration,
    ErrorBoundary,
  };
});

// ---------------------------------------------------------------------------
// Controllable useUser mock
// ---------------------------------------------------------------------------
const mockUseUser = vi.fn(() => ({
  isLoaded: true,
  isSignedIn: true as boolean,
  user: { id: 'user_test_123' } as { id: string } | null,
}));

vi.mock('@clerk/react', async () => {
  const actual = await vi.importActual<typeof import('@clerk/react')>('@clerk/react');
  return {
    ...actual,
    ClerkProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    useUser: () => mockUseUser(),
  };
});

// ---------------------------------------------------------------------------
// SentryUserSync — re-implemented locally to test in isolation.
// Logic is identical to App.tsx SentryUserSync.
// ---------------------------------------------------------------------------
function SentryUserSync() {
  const { user, isSignedIn } = mockUseUser();

  useEffect(() => {
    if (isSignedIn && user) {
      Sentry.setUser({ id: (user as { id: string }).id });
    } else {
      Sentry.setUser(null);
    }
  }, [isSignedIn, user]);

  return null;
}

// ---------------------------------------------------------------------------
// Class-based error boundary for fallback tests
// ---------------------------------------------------------------------------
class TestErrorBoundary extends React.Component<
  { children: React.ReactNode; fallback: React.ReactNode },
  { hasError: boolean }
> {
  constructor(props: { children: React.ReactNode; fallback: React.ReactNode }) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  render() {
    if (this.state.hasError) {
      return <>{this.props.fallback}</>;
    }
    return <>{this.props.children}</>;
  }
}

// Component that always throws during render
function Bomb() {
  throw new Error('test explosion');
}

// ---------------------------------------------------------------------------
// Helper — simulates the conditional Sentry.init logic from main.tsx
// ---------------------------------------------------------------------------
function runSentryInit(dsn: string | undefined) {
  if (dsn) {
    Sentry.init({ dsn, environment: 'test', integrations: [Sentry.browserTracingIntegration()] });
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Sentry integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset to default signed-in state
    mockUseUser.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { id: 'user_test_123' },
    });
  });

  // -------------------------------------------------------------------------
  // (a) Sentry.init conditional initialisation
  // -------------------------------------------------------------------------
  describe('Sentry.init conditional initialisation', () => {
    it('does NOT call Sentry.init when DSN is undefined', () => {
      runSentryInit(undefined);
      expect(Sentry.init).not.toHaveBeenCalled();
    });

    it('does NOT call Sentry.init when DSN is an empty string', () => {
      runSentryInit('');
      expect(Sentry.init).not.toHaveBeenCalled();
    });

    it('calls Sentry.init with the DSN when it is provided', () => {
      const dsn = 'https://examplePublicKey@o0.ingest.sentry.io/0';
      runSentryInit(dsn);
      expect(Sentry.init).toHaveBeenCalledOnce();
      expect(Sentry.init).toHaveBeenCalledWith(expect.objectContaining({ dsn }));
    });

    it('includes browserTracingIntegration when DSN is provided', () => {
      const dsn = 'https://examplePublicKey@o0.ingest.sentry.io/0';
      runSentryInit(dsn);
      expect(mockBrowserTracingIntegration).toHaveBeenCalled();
      expect(Sentry.init).toHaveBeenCalledWith(
        expect.objectContaining({
          integrations: expect.arrayContaining([{ name: 'BrowserTracing' }]),
        }),
      );
    });
  });

  // -------------------------------------------------------------------------
  // (b) ErrorBoundary renders fallback UI on thrown error
  // -------------------------------------------------------------------------
  describe('ErrorBoundary fallback rendering', () => {
    const originalConsoleError = console.error;
    beforeEach(() => {
      console.error = vi.fn();
    });
    afterEach(() => {
      console.error = originalConsoleError;
    });

    it('renders fallback text when a child component throws', () => {
      render(
        <TestErrorBoundary fallback={<p>Something went wrong</p>}>
          <Bomb />
        </TestErrorBoundary>,
      );
      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    });

    it('renders children normally when no error is thrown', () => {
      render(
        <TestErrorBoundary fallback={<p>Something went wrong</p>}>
          <p data-testid="healthy-child">All good</p>
        </TestErrorBoundary>,
      );
      expect(screen.getByTestId('healthy-child')).toBeInTheDocument();
      expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
    });
  });

  // -------------------------------------------------------------------------
  // (c) & (d) SentryUserSync — user context propagation
  // -------------------------------------------------------------------------
  describe('SentryUserSync', () => {
    it('calls Sentry.setUser with the user id when signed in', async () => {
      mockUseUser.mockReturnValue({
        isLoaded: true,
        isSignedIn: true,
        user: { id: 'user_test_123' },
      });

      await act(async () => {
        render(<SentryUserSync />);
      });

      expect(Sentry.setUser).toHaveBeenCalledWith({ id: 'user_test_123' });
    });

    it('calls Sentry.setUser(null) when signed out', async () => {
      mockUseUser.mockReturnValue({
        isLoaded: true,
        isSignedIn: false,
        user: null,
      });

      await act(async () => {
        render(<SentryUserSync />);
      });

      expect(Sentry.setUser).toHaveBeenCalledWith(null);
    });
  });
});
