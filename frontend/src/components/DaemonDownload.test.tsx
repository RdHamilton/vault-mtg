import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import DaemonDownload from './DaemonDownload';

const RELEASES_BASE =
  'https://github.com/RdHamilton/MTGA-Companion/releases/latest/download';

describe('DaemonDownload', () => {
  describe('Section Structure', () => {
    it('should render the download section container', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-download-section')).toBeInTheDocument();
    });

    it('should render the page title', () => {
      render(<DaemonDownload />);
      const title = screen.getByTestId('daemon-download-title');
      expect(title).toBeInTheDocument();
      expect(title).toHaveTextContent('Get Started with MTGA Companion');
    });

    it('should render the download buttons container', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-download-buttons')).toBeInTheDocument();
    });

    it('should render the getting started section', () => {
      render(<DaemonDownload />);
      expect(screen.getByTestId('daemon-getting-started')).toBeInTheDocument();
    });
  });

  describe('Download Links', () => {
    it('should render a link for Windows (amd64)', () => {
      render(<DaemonDownload />);
      const link = screen.getByTestId('download-link-windows-amd64');
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/mtga-companion-daemon-windows-amd64.exe`
      );
    });

    it('should render a link for macOS Apple Silicon (arm64)', () => {
      render(<DaemonDownload />);
      const link = screen.getByTestId('download-link-darwin-arm64');
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/mtga-companion-daemon-darwin-arm64.dmg`
      );
    });

    it('should render a link for macOS Intel (amd64)', () => {
      render(<DaemonDownload />);
      const link = screen.getByTestId('download-link-darwin-amd64');
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute(
        'href',
        `${RELEASES_BASE}/mtga-companion-daemon-darwin-amd64.dmg`
      );
    });

    it('should render exactly 3 download links', () => {
      render(<DaemonDownload />);
      const buttons = screen.getByTestId('daemon-download-buttons');
      const links = buttons.querySelectorAll('a');
      expect(links).toHaveLength(3);
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
      expect(screen.getByText('Windows (amd64)')).toBeInTheDocument();
      expect(screen.getByText('macOS (Apple Silicon)')).toBeInTheDocument();
      expect(screen.getByText('macOS (Intel)')).toBeInTheDocument();
    });

    it('should display platform descriptions', () => {
      render(<DaemonDownload />);
      expect(screen.getByText('Windows 10/11 64-bit')).toBeInTheDocument();
      expect(screen.getByText('macOS 12+ on M1/M2/M3')).toBeInTheDocument();
      expect(screen.getByText('macOS 12+ on Intel')).toBeInTheDocument();
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
      const windowsLink = screen.getByTestId('download-link-windows-amd64');
      expect(windowsLink).toHaveClass('daemon-download-button--primary');
    });

    it('should mark macOS links as secondary on Windows platform', () => {
      render(<DaemonDownload />);
      const arm64Link = screen.getByTestId('download-link-darwin-arm64');
      const amd64Link = screen.getByTestId('download-link-darwin-amd64');
      expect(arm64Link).toHaveClass('daemon-download-button--secondary');
      expect(amd64Link).toHaveClass('daemon-download-button--secondary');
    });

    it('should show Recommended label on Windows link', () => {
      render(<DaemonDownload />);
      const windowsLink = screen.getByTestId('download-link-windows-amd64');
      expect(windowsLink).toHaveTextContent('Recommended');
    });
  });

  describe('OS Detection — macOS Intel', () => {
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

    it('should mark macOS Intel as recommended', () => {
      render(<DaemonDownload />);
      const amd64Link = screen.getByTestId('download-link-darwin-amd64');
      expect(amd64Link).toHaveClass('daemon-download-button--primary');
    });

    it('should not mark arm64 as recommended on Intel Mac', () => {
      render(<DaemonDownload />);
      const arm64Link = screen.getByTestId('download-link-darwin-arm64');
      expect(arm64Link).toHaveClass('daemon-download-button--secondary');
    });
  });

  describe('Getting Started Steps', () => {
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
    it('should render main title as h1', () => {
      render(<DaemonDownload />);
      const h1 = screen.getByRole('heading', { level: 1 });
      expect(h1).toHaveTextContent('Get Started with MTGA Companion');
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
