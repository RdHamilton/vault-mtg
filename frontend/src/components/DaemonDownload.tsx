import { useMemo } from 'react';
import { trackEvent } from '@/services/analytics';
import { useFeatureFlag } from '@/hooks/useFeatureFlag';
import './DaemonDownload.css';

// Staging injects VITE_DAEMON_VERSION to pin downloads to a specific pre-release
// build (e.g. v0.3.1-rc11). Production leaves this unset so the URL always
// resolves to the latest stable release via GitHub's /releases/latest/download/.
const RELEASES_BASE = import.meta.env.VITE_DAEMON_VERSION
  ? `https://github.com/RdHamilton/MTGA-Companion/releases/download/${import.meta.env.VITE_DAEMON_VERSION}`
  : 'https://github.com/RdHamilton/MTGA-Companion/releases/latest/download';

const WAITLIST_URL = 'https://vaultmtg.app/#waitlist';

interface DownloadOption {
  label: string;
  /** Artifact filename (without extension) as it appears on the GitHub release. */
  artifact: string;
  /** Logical platform key used for OS detection matching. */
  platform: 'windows' | 'macos';
  ext: string;
  description: string;
}

const DOWNLOAD_OPTIONS: DownloadOption[] = [
  {
    label: 'Windows (64-bit)',
    artifact: 'vaultmtg-daemon-windows-amd64',
    platform: 'windows',
    ext: 'exe',
    description: 'Windows 10/11 64-bit',
  },
  {
    label: 'macOS (Universal)',
    artifact: 'vaultmtg-daemon-darwin-universal',
    platform: 'macos',
    ext: 'dmg',
    description: 'macOS 12+ — Apple Silicon and Intel',
  },
];

const GETTING_STARTED_STEPS = [
  {
    number: 1,
    title: 'Download',
    description: 'Download the daemon binary for your operating system using the button above.',
  },
  {
    number: 2,
    title: 'Run the installer',
    description:
      'On macOS: open the .dmg, drag the daemon to Applications, then launch it. On Windows: run the .exe installer and follow the Next → Next → Finish prompts.',
  },
  {
    number: 3,
    title: 'Launch MTGA Arena',
    description: 'Start MTG Arena as you normally would. The daemon will detect it automatically.',
  },
  {
    number: 4,
    title: 'Open the companion app',
    description:
      'With the daemon running, open the VaultMTG web app. Your match history and draft data will begin syncing.',
  },
];

function detectPlatform(): 'windows' | 'macos' {
  const ua = navigator.userAgent.toLowerCase();
  const platform =
    typeof navigator.platform === 'string'
      ? navigator.platform.toLowerCase()
      : '';

  if (platform.includes('win') || ua.includes('windows')) {
    return 'windows';
  }
  // Default to macOS (covers Mac + unknown)
  return 'macos';
}

function buildDownloadUrl(option: DownloadOption): string {
  return `${RELEASES_BASE}/${option.artifact}.${option.ext}`;
}

/** Skeleton placeholder shown while the PostHog feature flag loads. */
function DownloadButtonsSkeleton() {
  return (
    <div
      className="daemon-download-skeleton"
      data-testid="daemon-download-skeleton"
      aria-label="Loading download options"
      aria-busy="true"
    >
      <div className="daemon-download-skeleton-bar" />
      <div className="daemon-download-skeleton-bar" />
      <div className="daemon-download-skeleton-bar" />
    </div>
  );
}

/** CTA rendered when the daemon_download_enabled flag is off. */
function DownloadComingSoon() {
  return (
    <div
      className="daemon-download-coming-soon"
      data-testid="daemon-download-coming-soon"
    >
      <p className="daemon-download-coming-soon-message">
        The daemon installer will be available at beta launch.{' '}
        <a
          href={WAITLIST_URL}
          className="daemon-download-coming-soon-link"
          data-testid="daemon-download-waitlist-link"
          target="_blank"
          rel="noopener noreferrer"
        >
          Join the waitlist to get notified.
        </a>
      </p>
    </div>
  );
}

const DaemonDownload = () => {
  const detectedPlatform = useMemo(() => detectPlatform(), []);
  const { enabled: downloadEnabled } = useFeatureFlag('daemon_download_enabled');

  return (
    <section className="daemon-download" data-testid="daemon-download-section">
      <div className="daemon-download-header">
        <h1 className="daemon-download-title" data-testid="daemon-download-title">
          Get Started with VaultMTG
        </h1>
        <p className="daemon-download-subtitle">
          Download the daemon for your platform to start tracking your MTG Arena matches,
          drafts, and collection in real time.
        </p>
      </div>

      {downloadEnabled === null && <DownloadButtonsSkeleton />}

      {downloadEnabled === true && (
        <div className="daemon-download-buttons" data-testid="daemon-download-buttons">
          {DOWNLOAD_OPTIONS.map((option) => {
            const isDetected = option.platform === detectedPlatform;
            const href = buildDownloadUrl(option);
            return (
              <a
                key={option.artifact}
                href={href}
                className={`daemon-download-button ${isDetected ? 'daemon-download-button--primary' : 'daemon-download-button--secondary'}`}
                data-testid={`download-link-${option.artifact}`}
                download
                onClick={() => {
                  trackEvent({
                    name: 'funnel_daemon_download_started',
                    properties: {
                      os: option.artifact,
                      download_source: 'download_page',
                    },
                  });
                }}
              >
                <span className="daemon-download-button-label">{option.label}</span>
                {isDetected && (
                  <span className="daemon-download-button-recommended">Recommended</span>
                )}
                <span className="daemon-download-button-desc">{option.description}</span>
              </a>
            );
          })}
        </div>
      )}

      {downloadEnabled === false && <DownloadComingSoon />}

      <div className="daemon-getting-started" data-testid="daemon-getting-started">
        <h2 className="daemon-getting-started-title">Getting Started</h2>
        <ol className="daemon-getting-started-steps">
          {GETTING_STARTED_STEPS.map((step) => (
            <li
              key={step.number}
              className="daemon-getting-started-step"
              data-testid={`getting-started-step-${step.number}`}
            >
              <div className="step-number" aria-hidden="true">
                {step.number}
              </div>
              <div className="step-content">
                <h3 className="step-title">{step.title}</h3>
                <p className="step-description">{step.description}</p>
              </div>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
};

export default DaemonDownload;
