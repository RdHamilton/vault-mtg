import { BrowserRouter as Router, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { useAuth, useUser } from '@clerk/react';
import * as Sentry from '@sentry/react';
import { useSettings } from './hooks/useSettings';
import Layout from './components/Layout';
import ToastContainer from './components/ToastContainer';
import WinRateTrend from './pages/WinRateTrend';
import DeckPerformance from './pages/DeckPerformance';
import RankProgression from './pages/RankProgression';
import FormatDistribution from './pages/FormatDistribution';
import ResultBreakdown from './pages/ResultBreakdown';
import Quests from './pages/Quests';
import Draft from './pages/Draft';
import DraftAnalytics from './pages/DraftAnalytics';
import Decks from './pages/Decks';
import DeckBuilder from './pages/DeckBuilder';
import Collection from './pages/Collection';
import Meta from './pages/Meta';
import Settings from './pages/Settings';
import Download from './pages/Download';
import BffMatchHistory from './pages/BffMatchHistory';
import BffDraftHistory from './pages/BffDraftHistory';
import DraftLive from './pages/DraftLive';
import ApiKeysPage from './pages/ApiKeys';
import Profile from './pages/Profile';
import Setup from './pages/Setup';
import Home from './pages/Home';
import KeyboardShortcutsHandler from './components/KeyboardShortcutsHandler';
import ProtectedRoute from './components/ProtectedRoute';
import { RouteErrorFallback } from './components/RouteErrorFallback';
import { SseInitializer } from './components/SseInitializer';
import { PostHogRouteTracker } from './components/PostHogRouteTracker';
import { EventsOn } from './services/adapter';
import { setClerkTokenProvider } from './services/apiClient';
import { updateReplayState } from './utils/replayState';
import { gui } from '@/types/models';
import './App.css';

// Re-export for backward compatibility - these are used by other components
// eslint-disable-next-line react-refresh/only-export-components
export { getReplayState, subscribeToReplayState } from './utils/replayState';
export type { ReplayState } from './utils/replayState';

// Registers a Clerk token provider with apiClient so every BFF call sends the
// current Clerk session JWT as Bearer instead of the legacy daemon API key.
// Without this, every Clerk-protected BFF route (matches, decks, cards, etc.)
// returns 401. Re-runs whenever Clerk swaps the getToken identity.
function ClerkApiClientSync() {
  const { getToken } = useAuth();

  useEffect(() => {
    setClerkTokenProvider(() => getToken());
    return () => setClerkTokenProvider(null);
  }, [getToken]);

  return null;
}

// Syncs the authenticated Clerk user into Sentry context.
// Sets user id when signed in; clears it on sign-out.
//
// PII decision (AC6 #1841): only { id } is forwarded to Sentry.
// Clerk user.id is an opaque identifier (e.g. "user_2abc...") — it cannot be
// used to identify a person without Clerk dashboard access. No email, name, or
// other PII is included. sendDefaultPii: false in Sentry.init (main.tsx) ensures
// IP addresses and other automatic fields are also scrubbed.
// The PostHog hashing rule (ADR-027 §3, hashAccountID) applies to the analytics
// pipeline only; Sentry crash reports use the raw Clerk ID for event correlation.
function SentryUserSync() {
  const { user, isSignedIn } = useUser();

  useEffect(() => {
    if (isSignedIn && user) {
      Sentry.setUser({ id: user.id });
    } else {
      Sentry.setUser(null);
    }
  }, [isSignedIn, user]);

  return null;
}

// Applies the persisted theme to the document root so CSS selectors like
// [data-theme="light"] can cascade across all components. For "auto" mode
// it reads the OS preference and subscribes to changes so the DOM stays in
// sync when the user switches OS themes without reloading (AC2).
function ThemeSync() {
  const { theme } = useSettings();

  useEffect(() => {
    const root = document.documentElement;

    if (theme === 'auto') {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      const apply = (dark: boolean) => {
        root.setAttribute('data-theme', dark ? 'dark' : 'light');
      };
      apply(mediaQuery.matches);
      const listener = (e: MediaQueryListEvent) => apply(e.matches);
      mediaQuery.addEventListener('change', listener);
      return () => {
        mediaQuery.removeEventListener('change', listener);
      };
    } else {
      root.setAttribute('data-theme', theme ?? 'dark');
    }
  }, [theme]);

  return null;
}

// Component that handles global replay events
function ReplayEventHandler() {
  const navigate = useNavigate();
  const [hasShownDraftNotification, setHasShownDraftNotification] = useState(false);

  useEffect(() => {
    console.log('[ReplayEventHandler] Setting up global replay event listeners');

    // Listen for replay events and update global state
    const unsubscribeStarted = EventsOn('replay:started', (data: gui.ReplayStatus) => {
      console.log('[ReplayEventHandler] Replay started:', data);
      updateReplayState({
        isActive: true,
        isPaused: false,
        progress: data,
      });
      setHasShownDraftNotification(false);
    });

    const unsubscribeProgress = EventsOn('replay:progress', (data: gui.ReplayStatus) => {
      console.log('[ReplayEventHandler] Replay progress:', data);
      updateReplayState({
        progress: data,
      });
    });

    const unsubscribePaused = EventsOn('replay:paused', (data: gui.ReplayStatus) => {
      console.log('[ReplayEventHandler] ✅✅✅ Replay paused EVENT RECEIVED:', data);
      console.log('[ReplayEventHandler] About to update state to isPaused=true');
      updateReplayState({
        isPaused: true,
      });
      console.log('[ReplayEventHandler] State update called');
    });

    const unsubscribeResumed = EventsOn('replay:resumed', (data: gui.ReplayStatus) => {
      console.log('[ReplayEventHandler] Replay resumed:', data);
      updateReplayState({
        isPaused: false,
      });
    });

    const unsubscribeCompleted = EventsOn('replay:completed', (data: gui.ReplayStatus) => {
      console.log('[ReplayEventHandler] Replay completed:', data);
      updateReplayState({
        isActive: false,
        isPaused: false,
        progress: data,
      });
      setHasShownDraftNotification(false);
    });

    const unsubscribeDraftDetected = EventsOn('replay:draft_detected', (data: unknown) => {
      const eventData = gui.ReplayDraftDetectedEvent.createFrom(data);
      console.log('[ReplayEventHandler] Draft detected during replay:', eventData);

      // Automatically navigate to Draft tab
      navigate('/draft');

      // Show notification only once per replay session
      if (!hasShownDraftNotification) {
        // We'll use a console log for now since alerts don't work in desktop mode
        // The toast system will handle the notification
        console.log('Draft event detected - navigated to Draft tab!');
        setHasShownDraftNotification(true);
      }
    });

    const unsubscribeError = EventsOn('replay:error', (data: unknown) => {
      const eventData = gui.ReplayErrorEvent.createFrom(data);
      console.error('[ReplayEventHandler] Replay error:', eventData);
      updateReplayState({
        isActive: false,
        isPaused: false,
      });
    });

    return () => {
      console.log('[ReplayEventHandler] Cleaning up global replay event listeners');
      unsubscribeStarted();
      unsubscribeProgress();
      unsubscribePaused();
      unsubscribeResumed();
      unsubscribeCompleted();
      unsubscribeDraftDetected();
      unsubscribeError();
    };
  }, [navigate, hasShownDraftNotification]);

  return null; // This component doesn't render anything
}

function App() {
  return (
    <Router>
      <PostHogRouteTracker />
      <ClerkApiClientSync />
      <SseInitializer />
      <SentryUserSync />
      <ThemeSync />
      <ReplayEventHandler />
      <KeyboardShortcutsHandler />
      <Layout>
        <Routes>
          {/* Public routes — no auth required */}
          <Route path="/" element={<Navigate to="/home" replace />} />
          <Route
            path="/download"
            element={
              <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                <Download />
              </Sentry.ErrorBoundary>
            }
          />
          <Route
            path="/setup"
            element={
              <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                <Setup />
              </Sentry.ErrorBoundary>
            }
          />

          {/* Protected routes — require Clerk authentication.
              Each route element is individually wrapped in Sentry.ErrorBoundary so
              a crash in one route does not blank the entire app. The top-level
              Sentry.ErrorBoundary in main.tsx remains as a last-resort catch for
              errors outside the route tree (e.g. ClerkProvider / AppProvider failures). */}
          <Route element={<ProtectedRoute />}>
            <Route
              path="/home"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Home />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/match-history"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <BffMatchHistory />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/quests"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Quests />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/draft"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Draft />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/draft-analytics"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <DraftAnalytics />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/decks"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Decks />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/deck-builder/:deckID"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <DeckBuilder />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/deck-builder/draft/:draftEventID"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <DeckBuilder />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/collection"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Collection />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/meta"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Meta />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/charts/win-rate-trend"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <WinRateTrend />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/charts/deck-performance"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <DeckPerformance />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/charts/rank-progression"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <RankProgression />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/charts/format-distribution"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <FormatDistribution />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/charts/result-breakdown"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <ResultBreakdown />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/settings"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Settings />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/history/drafts"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <BffDraftHistory />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/draft/live"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <DraftLive />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/api-keys"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <ApiKeysPage />
                </Sentry.ErrorBoundary>
              }
            />
            <Route
              path="/profile"
              element={
                <Sentry.ErrorBoundary fallback={<RouteErrorFallback />}>
                  <Profile />
                </Sentry.ErrorBoundary>
              }
            />
          </Route>
        </Routes>
      </Layout>
      <ToastContainer />
    </Router>
  );
}

export default App;
