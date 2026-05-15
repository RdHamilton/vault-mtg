import { BrowserRouter as Router, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { useAuth, useUser } from '@clerk/react';
import * as Sentry from '@sentry/react';
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
import Setup from './pages/Setup';
import KeyboardShortcutsHandler from './components/KeyboardShortcutsHandler';
import ProtectedRoute from './components/ProtectedRoute';
import { SseInitializer } from './components/SseInitializer';
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
      <ClerkApiClientSync />
      <SseInitializer />
      <SentryUserSync />
      <ReplayEventHandler />
      <KeyboardShortcutsHandler />
      <Layout>
        <Routes>
          {/* Public routes — no auth required */}
          <Route path="/" element={<Navigate to="/match-history" replace />} />
          <Route path="/download" element={<Download />} />
          <Route path="/setup" element={<Setup />} />

          {/* Protected routes — require Clerk authentication */}
          <Route element={<ProtectedRoute />}>
            <Route path="/match-history" element={<BffMatchHistory />} />
            <Route path="/quests" element={<Quests />} />
            <Route path="/draft" element={<Draft />} />
            <Route path="/draft-analytics" element={<DraftAnalytics />} />
            <Route path="/decks" element={<Decks />} />
            <Route path="/deck-builder/:deckID" element={<DeckBuilder />} />
            <Route path="/deck-builder/draft/:draftEventID" element={<DeckBuilder />} />
            <Route path="/collection" element={<Collection />} />
            <Route path="/meta" element={<Meta />} />
            <Route path="/charts/win-rate-trend" element={<WinRateTrend />} />
            <Route path="/charts/deck-performance" element={<DeckPerformance />} />
            <Route path="/charts/rank-progression" element={<RankProgression />} />
            <Route path="/charts/format-distribution" element={<FormatDistribution />} />
            <Route path="/charts/result-breakdown" element={<ResultBreakdown />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/history/drafts" element={<BffDraftHistory />} />
            <Route path="/draft/live" element={<DraftLive />} />
            <Route path="/api-keys" element={<ApiKeysPage />} />
          </Route>
        </Routes>
      </Layout>
      <ToastContainer />
    </Router>
  );
}

export default App;
