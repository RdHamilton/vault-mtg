import { useState, useCallback } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '@clerk/react';
import Footer from './Footer';
import AuthBar from './AuthBar';
import DaemonHealthIndicator, { type DaemonHealthState } from './DaemonHealthIndicator';
import { OnboardingModal } from './OnboardingModal';
import { usePostHogIdentity } from '@/hooks/usePostHogIdentity';
import { useDaemonOnboarding } from '@/hooks/useDaemonOnboarding';
import ReportBugButton from './ReportBugButton';
import './Layout.css';

interface LayoutProps {
  children: React.ReactNode;
}

const Layout = ({ children }: LayoutProps) => {
  const location = useLocation();
  const { isSignedIn } = useAuth();
  // Identify signed-in user with PostHog and fire funnel_sign_up_completed once per session.
  usePostHogIdentity();

  // Track daemon health status from the indicator so the onboarding hook can use it.
  const [daemonStatus, setDaemonStatus] = useState<DaemonHealthState>('loading');

  // Onboarding modal logic: show when daemon disconnected on first login.
  const { isOpen: onboardingOpen, open: openOnboarding, dismiss: dismissOnboarding, complete: completeOnboarding } =
    useDaemonOnboarding(daemonStatus, isSignedIn ?? false);

  const handleDaemonStatusChange = useCallback((status: DaemonHealthState) => {
    setDaemonStatus(status);
  }, []);

  const isActive = (path: string) => location.pathname === path;

  // Derive activeTab from current route (computed value, not state)
  const getActiveTab = (): 'match-history' | 'quests' | 'draft' | 'decks' | 'collection' | 'meta' | 'charts' | 'download' => {
    if (location.pathname === '/match-history' || location.pathname === '/') {
      return 'match-history';
    } else if (location.pathname === '/quests') {
      return 'quests';
    } else if (location.pathname === '/draft' || location.pathname === '/draft-analytics') {
      return 'draft';
    } else if (location.pathname === '/decks' || location.pathname.startsWith('/deck-builder')) {
      return 'decks';
    } else if (location.pathname === '/collection') {
      return 'collection';
    } else if (location.pathname === '/meta') {
      return 'meta';
    } else if (location.pathname.startsWith('/charts/')) {
      return 'charts';
    } else if (location.pathname === '/download') {
      return 'download';
    }
    return 'match-history';
  };

  const activeTab = getActiveTab();



  return (
    <div className="app-container" data-testid="app-container">
      {/* Top Navigation Tabs */}
      <div className="tab-bar" data-testid="nav-tab-bar">
        <div className="tab-links">
          <Link
            to="/match-history"
            className={`tab ${activeTab === 'match-history' ? 'active' : ''}`}
            data-testid="nav-tab-match-history"
          >
            Match History
          </Link>
          <Link
            to="/quests"
            className={`tab ${activeTab === 'quests' ? 'active' : ''}`}
            data-testid="nav-tab-quests"
          >
            Quests
          </Link>
          <Link
            to="/draft"
            className={`tab ${activeTab === 'draft' ? 'active' : ''}`}
            data-testid="nav-tab-draft"
          >
            Draft
          </Link>
          <Link
            to="/decks"
            className={`tab ${activeTab === 'decks' ? 'active' : ''}`}
            data-testid="nav-tab-decks"
          >
            Decks
          </Link>
          <Link
            to="/collection"
            className={`tab ${activeTab === 'collection' ? 'active' : ''}`}
            data-testid="nav-tab-collection"
          >
            Collection
          </Link>
          <Link
            to="/meta"
            className={`tab ${activeTab === 'meta' ? 'active' : ''}`}
            data-testid="nav-tab-meta"
          >
            Meta
          </Link>
          <Link
            to="/charts/win-rate-trend"
            className={`tab ${activeTab === 'charts' ? 'active' : ''}`}
            data-testid="nav-tab-charts"
          >
            Charts
          </Link>
          <Link
            to="/download"
            className={`tab ${activeTab === 'download' ? 'active' : ''}`}
            data-testid="nav-tab-download"
          >
            Download
          </Link>
          <Link
            to="/settings"
            className={`tab ${isActive('/settings') ? 'active' : ''}`}
            data-testid="nav-tab-settings"
          >
            Settings
          </Link>
        </div>
        <div className="tab-bar-right">
          {isSignedIn && <ReportBugButton />}
          <AuthBar />
          <div className="connection-status-indicator">
            <DaemonHealthIndicator
              onOpenOnboarding={openOnboarding}
              onStatusChange={handleDaemonStatusChange}
            />
          </div>
        </div>
      </div>

      {/* Sub-navigation for Draft */}
      {activeTab === 'draft' && (
        <div className="sub-tab-bar" data-testid="draft-sub-tab-bar">
          <Link
            to="/draft"
            className={`sub-tab ${isActive('/draft') ? 'active' : ''}`}
            data-testid="sub-tab-current-draft"
          >
            Current Draft
          </Link>
          <Link
            to="/draft-analytics"
            className={`sub-tab ${isActive('/draft-analytics') ? 'active' : ''}`}
            data-testid="sub-tab-analytics"
          >
            Analytics
          </Link>
        </div>
      )}

      {/* Sub-navigation for Charts */}
      {activeTab === 'charts' && (
        <div className="sub-tab-bar" data-testid="charts-sub-tab-bar">
          <Link
            to="/charts/win-rate-trend"
            className={`sub-tab ${isActive('/charts/win-rate-trend') ? 'active' : ''}`}
          >
            Win Rate Trend
          </Link>
          <Link
            to="/charts/deck-performance"
            className={`sub-tab ${isActive('/charts/deck-performance') ? 'active' : ''}`}
          >
            Deck Performance
          </Link>
          <Link
            to="/charts/rank-progression"
            className={`sub-tab ${isActive('/charts/rank-progression') ? 'active' : ''}`}
          >
            Rank Progression
          </Link>
          <Link
            to="/charts/format-distribution"
            className={`sub-tab ${isActive('/charts/format-distribution') ? 'active' : ''}`}
          >
            Format Distribution
          </Link>
          <Link
            to="/charts/result-breakdown"
            className={`sub-tab ${isActive('/charts/result-breakdown') ? 'active' : ''}`}
          >
            Result Breakdown
          </Link>
        </div>
      )}

      {/* Main Content */}
      <div className="content" data-testid="main-content">
        {children}
      </div>

      {/* Footer with Stats */}
      <Footer />

      {/* Daemon onboarding modal — shown on first login if daemon not connected */}
      <OnboardingModal
        isOpen={onboardingOpen}
        onDismiss={dismissOnboarding}
        onComplete={completeOnboarding}
      />
    </div>
  );
};

export default Layout;
