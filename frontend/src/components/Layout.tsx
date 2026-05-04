import { useState, useEffect } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import Footer from './Footer';
import { system } from '@/services/api';
import { EventsOn, EventsOff } from '@/services/websocketClient';
import { getReplayState, subscribeToReplayState } from '../App';
import { gui } from '@/types/models';
import './Layout.css';

interface LayoutProps {
  children: React.ReactNode;
}

const Layout = ({ children }: LayoutProps) => {
  const location = useLocation();
  const navigate = useNavigate();
  const [connectionStatus, setConnectionStatus] = useState<gui.ConnectionStatus>(
    new gui.ConnectionStatus({
      status: 'standalone',
      connected: false,
      mode: 'standalone',
      url: '',
      port: 0,
    })
  );
  const [replayActive, setReplayActive] = useState(getReplayState().isActive);
  const [replayPaused, setReplayPaused] = useState(getReplayState().isPaused);

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

  // Subscribe to replay state changes
  useEffect(() => {
    console.log('[Layout] Subscribing to replay state changes');
    const unsubscribe = subscribeToReplayState((state) => {
      console.log('[Layout] Replay state updated:', state);
      setReplayActive(state.isActive);
      setReplayPaused(state.isPaused);
    });

    return unsubscribe;
  }, []);

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

  const handleResumeReplay = async () => {
    // Replay control not implemented in REST API yet
    console.log('Resume replay not implemented in REST API');
  };

  const handleStopReplay = async () => {
    // Replay control not implemented in REST API yet
    console.log('Stop replay not implemented in REST API');
    navigate('/settings');
  };

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
        <div className="connection-status-indicator">
          <div className={`status-badge-compact status-${connectionStatus.status}`} title={connectionStatus.status} data-testid="connection-status-badge">
            <span className="status-dot-compact"></span>
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

      {/* Floating Replay Control Banner - Only shown when replay is paused, not on settings or draft page */}
      {replayActive && replayPaused && location.pathname !== '/settings' && location.pathname !== '/draft' && (
        <div style={{
          position: 'fixed',
          bottom: '60px',
          right: '20px',
          background: '#ff9800',
          color: 'white',
          padding: '16px 24px',
          borderRadius: '8px',
          boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
          zIndex: 1000,
          display: 'flex',
          alignItems: 'center',
          gap: '16px',
          fontWeight: 'bold',
        }}>
          <span data-testid="replay-paused-label">⏸️ Replay Paused</span>
          <button
            onClick={handleResumeReplay}
            data-testid="replay-resume-button"
            style={{
              background: '#00c853',
              color: 'white',
              border: 'none',
              padding: '8px 16px',
              borderRadius: '4px',
              cursor: 'pointer',
              fontWeight: 'bold',
            }}
          >
            ▶️ Resume
          </button>
          <button
            onClick={handleStopReplay}
            data-testid="replay-stop-button"
            style={{
              background: '#f44336',
              color: 'white',
              border: 'none',
              padding: '8px 16px',
              borderRadius: '4px',
              cursor: 'pointer',
              fontWeight: 'bold',
            }}
          >
            ⏹️ Stop
          </button>
          <button
            onClick={() => navigate('/settings')}
            data-testid="replay-settings-button"
            style={{
              background: 'transparent',
              color: 'white',
              border: '1px solid white',
              padding: '8px 16px',
              borderRadius: '4px',
              cursor: 'pointer',
            }}
          >
            Settings
          </button>
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
