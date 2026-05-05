import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { render } from '../test/utils/testUtils';
import Layout from './Layout';
import { mockSystem, mockMatches } from '@/test/mocks/apiMock';
import { mockEventEmitter } from '@/test/mocks/websocketMock';

// Mock useDownload since Layout renders Footer which includes DownloadProgressBar
vi.mock('@/context/DownloadContext', () => ({
  useDownload: () => ({
    state: { tasks: [], activeTask: null },
    isDownloading: false,
    overallProgress: 0,
  }),
  DownloadProvider: ({ children }: { children: React.ReactNode }) => children,
}));

describe('Layout Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEventEmitter.clear();
    mockSystem.getStatus.mockResolvedValue({
      status: 'standalone',
      connected: false,
    });
  });

  describe('Navigation Tabs', () => {
    it('should render all navigation tabs', () => {
      render(
        <Layout>
          <div>Test Content</div>
        </Layout>,
        { initialRoute: '/' }
      );

      expect(screen.getByTestId('nav-tab-match-history')).toBeInTheDocument();
      expect(screen.getByTestId('nav-tab-quests')).toBeInTheDocument();
      expect(screen.getByTestId('nav-tab-draft')).toBeInTheDocument();
      expect(screen.getByTestId('nav-tab-charts')).toBeInTheDocument();
      expect(screen.getByTestId('nav-tab-settings')).toBeInTheDocument();
    });

    it('should highlight active tab based on current route', () => {
      render(
        <Layout>
          <div>Test Content</div>
        </Layout>,
        { initialRoute: '/draft' }
      );

      const draftTab = screen.getByTestId('nav-tab-draft');
      expect(draftTab).toHaveClass('active');
    });

    it('should navigate to correct route when tab is clicked', async () => {
      render(
        <Layout>
          <div>Test Content</div>
        </Layout>,
        { initialRoute: '/' }
      );

      const questsTab = screen.getByTestId('nav-tab-quests');
      await userEvent.click(questsTab);

      await waitFor(() => {
        expect(questsTab).toHaveClass('active');
      });
    });

    it('should show sub-navigation when Charts tab is active', async () => {
      render(
        <Layout>
          <div>Test Content</div>
        </Layout>,
        { initialRoute: '/charts/win-rate-trend' }
      );

      await waitFor(() => {
        expect(screen.getByText('Win Rate Trend')).toBeInTheDocument();
        expect(screen.getByText('Deck Performance')).toBeInTheDocument();
        expect(screen.getByText('Rank Progression')).toBeInTheDocument();
        expect(screen.getByText('Format Distribution')).toBeInTheDocument();
        expect(screen.getByText('Result Breakdown')).toBeInTheDocument();
      });
    });

    it('should not show sub-navigation when Charts tab is inactive', () => {
      render(
        <Layout>
          <div>Test Content</div>
        </Layout>,
        { initialRoute: '/match-history' }
      );

      // Sub-navigation should not be present
      expect(screen.queryByTestId('charts-sub-tab-bar')).not.toBeInTheDocument();
      expect(screen.queryByTestId('draft-sub-tab-bar')).not.toBeInTheDocument();
    });
  });

  describe('Connection Status', () => {
    it('should display connection status indicator', async () => {
      mockSystem.getStatus.mockResolvedValue({
        status: 'connected',
        connected: true,
      });

      render(
        <Layout>
          <div>Test Content</div>
        </Layout>
      );

      await waitFor(() => {
        const statusBadge = screen.getByTestId('connection-status-badge');
        expect(statusBadge).toBeInTheDocument();
        expect(statusBadge).toHaveClass('status-connected');
      });
    });

    it('should update connection status when daemon:connected event fires', async () => {
      mockSystem.getStatus
        .mockResolvedValueOnce({
          status: 'standalone',
          connected: false,
        })
        .mockResolvedValueOnce({
          status: 'connected',
          connected: true,
        });

      render(
        <Layout>
          <div>Test Content</div>
        </Layout>
      );

      await waitFor(() => {
        const statusBadge = screen.getByTestId('connection-status-badge');
        expect(statusBadge).toHaveClass('status-standalone');
      });

      // Trigger daemon:connected event
      mockEventEmitter.emit('daemon:connected');

      await waitFor(() => {
        const statusBadge = screen.getByTestId('connection-status-badge');
        expect(statusBadge).toHaveClass('status-connected');
      });
    });
  });

  describe('Content Rendering', () => {
    it('should render children content', () => {
      render(
        <Layout>
          <div data-testid="test-content">Test Content</div>
        </Layout>
      );

      expect(screen.getByTestId('test-content')).toBeInTheDocument();
      expect(screen.getByText('Test Content')).toBeInTheDocument();
    });

    it('should render Footer component', () => {
      mockMatches.getStats.mockResolvedValue({
        TotalMatches: 0,
        MatchesWon: 0,
        MatchesLost: 0,
        TotalGames: 0,
        GamesWon: 0,
        GamesLost: 0,
        WinRate: 0,
      });

      render(
        <Layout>
          <div>Test Content</div>
        </Layout>
      );

      // Footer should be present
      const footer = document.querySelector('.app-footer');
      expect(footer).toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    it('should handle connection status load error gracefully', async () => {
      mockSystem.getStatus.mockRejectedValue(new Error('Failed to load'));

      render(
        <Layout>
          <div>Test Content</div>
        </Layout>
      );

      // Component should still render
      expect(screen.getByTestId('nav-tab-match-history')).toBeInTheDocument();
    });

    it('should handle connection status error without crashing', async () => {
      mockSystem.getStatus.mockRejectedValue(new Error('Connection error'));

      expect(() => {
        render(
          <Layout>
            <div>Test Content</div>
          </Layout>
        );
      }).not.toThrow();

      // Layout should still render
      expect(screen.getByTestId('nav-tab-match-history')).toBeInTheDocument();
    });
  });
});
