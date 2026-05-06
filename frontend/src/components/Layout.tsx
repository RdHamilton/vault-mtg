import { useState, useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import Footer from './Footer';
import AuthBar from './AuthBar';
import { system } from '@/services/api';
import { EventsOn, EventsOff } from '@/services/websocketClient';
import { gui } from '@/types/models';
import './Layout.css';

interface LayoutProps {
  children: React.ReactNode;
}

const Layout = ({ children }: LayoutProps) => {
  const location = useLocation();
  const [connectionStatus, setConnectionStatus] = useState<gui.ConnectionStatus>(
    new gui.ConnectionStatus({
      status: 'standalone',
      connected: false,
      mode: 'standalone',
      url: '',
      port: 0,
    })
  );
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



  // Load connection status on mount
  useEffect(() => {
    const loadConnectionStatus = async () => {
      try {
        const status = await system.getStatus();
        setConnectionStatus(gui.ConnectionStatus.createFrom(status));
      } catch (error) {
        console.error('Failed to load connection status:', error);
      }
    };

    loadConnectionStatus();

    // Listen for daemon events
    const handleDaemonStatus = () => loadConnectionStatus();
    const handleDaemonConnected = () => loadConnectionStatus();

    EventsOn('daemon:status', handleDaemonStatus);
    EventsOn('daemon:connected', handleDaemonConnected);

    return () => {
      EventsOff('daemon:status');
      EventsOff('daemon:connected');
    };
  }, []);

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
          <AuthBar />
          <div className="connection-status-indicator">
            <div className={`status-badge-compact status-${connectionStatus.status}`} title={connectionStatus.status} data-testid="connection-status-badge">
              <span className="status-dot-compact"></span>
            </div>
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
    </div>
  );
};

export default Layout;
