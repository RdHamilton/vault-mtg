import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import DaemonDownload from './DaemonDownload';

const RELEASES_BASE =
  'https://github.com/RdHamilton/MTGA-Companion/releases/latest/download';

// Mock useFeatureFlag so we can control the flag state per test suite.
vi.mock('@/hooks/useFeatureFlag', () => ({
  useFeatureFlag: vi.fn(),
}));

import { useFeatureFlag } from '@/hooks/useFeatureFlag';

const mockUseFeatureFlag = vi.mocked(useFeatureFlag);

describe('DaemonDownload', () => {
  describe('Feature flag — enabled (download buttons visible)', () => {
    beforeEach(() => {
      mockUseFeatureFlag.mockReturnValue({ enabled: true });
    });

    describe('Section Structure', () => {
      it('should render the download section container', () => {
        render(<DaemonDownload />);
        expect(screen.getByTestId('daemon-download-section')).toBeInTheDocument();
      });

      it('should render the page title', () => {
        render(<DaemonDownload />);
        const title = screen.getByTestId('daemon-download-title');
        expect(title).toBeInTheDocument();
        expect(title).toHaveTextContent('Get Started with VaultMTG');
      });

      it('should render the download buttons container', () => {
        render(<DaemonDownload />);
        expect(screen.getByTestId('daemon-download-buttons')).toBeInTheDocument();
      });

      it('should render the getting started section', () => {
        render(<DaemonDownload />);
        expect(screen.getByTestId('daemon-getting-started')).toBeInTheDocument();
      });

      it('should not render the coming-soon CTA', () => {
        render(<DaemonDownload />);
        expect(screen.queryByTestId('daemon-download-coming-soon')).not.toBeInTheDocument();
      });

      it('should not render the skeleton', () => {
        render(<DaemonDownload />);
        expect(screen.queryByTestId('daemon-download-skeleton')).not.toBeInTheDocument();
      });
    });

    describe('Download Links', () => {
      it('should render a link for Windows', () => {
        render(<DaemonDownload />);
        const link = screen.getByTestId('download-link-vaultmtg-daemon-windows-amd64');
        expect(link).toBeInTheDocument();
        expect(link).toHaveAttribute(
          'href',
          `${RELEASES_BASE}/vaultmtg-daemon-windows-amd64.exe`
        );
      });

      it('should render a link for macOS Universal', () => {
        render(<DaemonDownload />);
        const link = screen.getByTestId('download-link-vaultmtg-daemon-darwin-universal');
        expect(link).toBeInTheDocument();
        expect(link).toHaveAttribute(
          'href',
          `${RELEASES_BASE}/vaultmtg-daemon-darwin-universal.dmg`
        );
      });

      it('should render exactly 2 download links', () => {
        render(<DaemonDownload />);
        const buttons = screen.getByTestId('daemon-download-buttons');
        const links = buttons.querySelectorAll('a');
        expect(links).toHaveLength(2);
      });

      it('each download link should have the download attribute', () => {
        render(<DaemonDownload />);
        const buttons = screen.getByTestId('daemon-download-buttons');
        const links = buttons.querySelectorAll('a');
        links.forEach((link) => {
          expect(link).toHaveAttribute('download');
        });
      });

      it('should display platform labels', () => {
        render(<DaemonDownload />);
        expect(screen.getByText('Windows (64-bit)')).toBeInTheDocument();
        expect(screen.getByText('macOS (Universal)')).toBeInTheDocument();
      });

      it('should display platform descriptions', () => {
        render(<DaemonDownload />);
        expect(screen.getByText('Windows 10/11 64-bit')).toBeInTheDocument();
        expect(screen.getByText('macOS 12+ — Apple Silicon and Intel')).toBeInTheDocument();
      });
    });

    describe('OS Detection — Windows', () => {
      beforeEach(() => {
        Object.defineProperty(navigator, 'platform', {
          value: 'Win32',
          configurable: true,
        });
        Object.defineProperty(navigator, 'userAgent', {
          value:
            'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/123.0',
          configurable: true,
        });
      });

      it('should mark Windows as recommended on Windows platform', () => {
        render(<DaemonDownload />);
        const windowsLink = screen.getByTestId('download-link-vaultmtg-daemon-windows-amd64');
        expect(windowsLink).toHaveClass('daemon-download-button--primary');
      });

      it('should mark macOS link as secondary on Windows platform', () => {
        render(<DaemonDownload />);
        const macLink = screen.getByTestId('download-link-vaultmtg-daemon-darwin-universal');
        expect(macLink).toHaveClass('daemon-download-button--secondary');
      });

      it('should show Recommended label on Windows link', () => {
        render(<DaemonDownload />);
        const windowsLink = screen.getByTestId('download-link-vaultmtg-daemon-windows-amd64');
        expect(windowsLink).toHaveTextContent('Recommended');
      });
    });

    describe('OS Detection — macOS', () => {
      beforeEach(() => {
        Object.defineProperty(navigator, 'platform', {
          value: 'MacIntel',
          configurable: true,
        });
        Object.defineProperty(navigator, 'userAgent', {
          value:
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/123.0',
          configurable: true,
        });
      });

      it('should mark macOS Universal as recommended on macOS', () => {
        render(<DaemonDownload />);
        const macLink = screen.getByTestId('download-link-vaultmtg-daemon-darwin-universal');
        expect(macLink).toHaveClass('daemon-download-button--primary');
      });

      it('should mark Windows link as secondary on macOS', () => {
        render(<DaemonDownload />);
        const windowsLink = screen.getByTestId('download-link-vaultmtg-daemon-windows-amd64');
        expect(windowsLink).toHaveClass('daemon-download-button--secondary');
      });
    });
  });

  describe('Feature flag — disabled (coming soon CTA visible)', () => {
    beforeEach(() => {
      mockUseFeatureFlag.mockReturnValue({ enabled: false });
    });

    it('should render the download section container', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-download-section')).toBeInTheDocument();
    });

    it('should render the page title', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-download-title')).toHaveTextContent(
        'Get Started with VaultMTG'
      );
    });

    it('should render the coming-soon CTA', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-download-coming-soon')).toBeInTheDocument();
    });

    it('should display the coming-soon message text', () => {
      render(<DaemonDownload />);
      expect(
        screen.getByText(/The daemon installer will be available at beta launch/i)
      ).toBeInTheDocument();
    });

    it('should render the waitlist link', () => {
      render(<DaemonDownload />);
      const link = screen.getByTestId('daemon-download-waitlist-link');
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', 'https://vaultmtg.app/#waitlist');
    });

    it('should NOT render the download buttons', () => {
      render(<DaemonDownload />);
      expect(screen.queryByTestId('daemon-download-buttons')).not.toBeInTheDocument();
    });

    it('should NOT render any download link anchors', () => {
      render(<DaemonDownload />);
      expect(screen.queryByTestId('download-link-windows-amd64')).not.toBeInTheDocument();
      expect(screen.queryByTestId('download-link-darwin-arm64')).not.toBeInTheDocument();
      expect(screen.queryByTestId('download-link-darwin-amd64')).not.toBeInTheDocument();
    });

    it('should NOT render the skeleton', () => {
      render(<DaemonDownload />);
      expect(screen.queryByTestId('daemon-download-skeleton')).not.toBeInTheDocument();
    });

    it('should still render the getting started section', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-getting-started')).toBeInTheDocument();
    });
  });

  describe('Feature flag — loading (skeleton visible)', () => {
    beforeEach(() => {
      mockUseFeatureFlag.mockReturnValue({ enabled: null });
    });

    it('should render the skeleton', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-download-skeleton')).toBeInTheDocument();
    });

    it('should NOT render the download buttons', () => {
      render(<DaemonDownload />);
      expect(screen.queryByTestId('daemon-download-buttons')).not.toBeInTheDocument();
    });

    it('should NOT render the coming-soon CTA', () => {
      render(<DaemonDownload />);
      expect(screen.queryByTestId('daemon-download-coming-soon')).not.toBeInTheDocument();
    });

    it('should render aria-busy on the skeleton', () => {
      render(<DaemonDownload />);
      const skeleton = screen.getByTestId('daemon-download-skeleton');
      expect(skeleton).toHaveAttribute('aria-busy', 'true');
    });

    it('should still render the page title', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-download-title')).toBeInTheDocument();
    });

    it('should still render the getting started section', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-getting-started')).toBeInTheDocument();
    });
  });

  describe('Getting Started Steps', () => {
    beforeEach(() => {
      mockUseFeatureFlag.mockReturnValue({ enabled: true });
    });

    it('should render all 4 steps', () => {
      render(<DaemonDownload />);
      for (let i = 1; i <= 4; i++) {
        expect(screen.getByTestId(`getting-started-step-${i}`)).toBeInTheDocument();
      }
    });

    it('should render step 1 — Download', () => {
      render(<DaemonDownload />);
      const step = screen.getByTestId('getting-started-step-1');
      expect(step).toHaveTextContent('Download');
    });

    it('should render step 2 — Run the installer', () => {
      render(<DaemonDownload />);
      const step = screen.getByTestId('getting-started-step-2');
      expect(step).toHaveTextContent('Run the installer');
      // macOS uses .dmg, not install script
      expect(step).toHaveTextContent('.dmg');
      expect(step).not.toHaveTextContent('install script');
    });

    it('should render step 3 — Launch MTGA Arena', () => {
      render(<DaemonDownload />);
      const step = screen.getByTestId('getting-started-step-3');
      expect(step).toHaveTextContent('Launch MTGA Arena');
    });

    it('should render step 4 — Open the companion app', () => {
      render(<DaemonDownload />);
      const step = screen.getByTestId('getting-started-step-4');
      expect(step).toHaveTextContent('Open the companion app');
    });

    it('should render Getting Started heading', () => {
      render(<DaemonDownload />);
      const heading = screen.getByRole('heading', { name: 'Getting Started' });
      expect(heading).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    beforeEach(() => {
      mockUseFeatureFlag.mockReturnValue({ enabled: true });
    });

    it('should render main title as h1', () => {
      render(<DaemonDownload />);
      const h1 = screen.getByRole('heading', { level: 1 });
      expect(h1).toHaveTextContent('Get Started with VaultMTG');
    });

    it('should render Getting Started as h2', () => {
      render(<DaemonDownload />);
      const h2 = screen.getByRole('heading', { level: 2, name: 'Getting Started' });
      expect(h2).toBeInTheDocument();
    });

    it('should render step titles as h3', () => {
      render(<DaemonDownload />);
      const h3s = screen.getAllByRole('heading', { level: 3 });
      expect(h3s.length).toBe(4);
    });
  });
});
