import { useMemo } from 'react';
import { trackEvent } from '@/services/analytics';
import './DaemonDownload.css';

const RELEASES_BASE =
  'https://github.com/RdHamilton/MTGA-Companion/releases/latest/download';

interface DownloadOption {
  label: string;
  platform: string;
  ext: string;
  description: string;
}

const DOWNLOAD_OPTIONS: DownloadOption[] = [
  {
    label: 'Windows (amd64)',
    platform: 'windows-amd64',
    ext: 'exe',
    description: 'Windows 10/11 64-bit',
  },
  {
    label: 'macOS (Apple Silicon)',
    platform: 'darwin-arm64',
    ext: 'dmg',
    description: 'macOS 12+ on M1/M2/M3',
  },
  {
    label: 'macOS (Intel)',
    platform: 'darwin-amd64',
    ext: 'dmg',
    description: 'macOS 12+ on Intel',
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
      'With the daemon running, open the MTGA Companion web app. Your match history and draft data will begin syncing.',
  },
];

function detectPlatform(): string {
  const ua = navigator.userAgent.toLowerCase();
  const platform =
    typeof navigator.platform === 'string'
      ? navigator.platform.toLowerCase()
      : '';

  if (platform.includes('win') || ua.includes('windows')) {
    return 'windows-amd64';
  }
  if (platform.includes('mac') || ua.includes('mac')) {
    // Detect Apple Silicon via userAgentData or processor hint
    const isAppleSilicon =
      (navigator as Navigator & { userAgentData?: { platform?: string } })
        .userAgentData?.platform === 'macOS' &&
      !ua.includes('intel');
    return isAppleSilicon ? 'darwin-arm64' : 'darwin-amd64';
  }
  // Default to macOS arm64 as a reasonable fallback
  return 'darwin-arm64';
}

function buildDownloadUrl(option: DownloadOption): string {
  return `${RELEASES_BASE}/mtga-companion-daemon-${option.platform}.${option.ext}`;
}

const DaemonDownload = () => {
  const detectedPlatform = useMemo(() => detectPlatform(), []);

  return (
    <section className="daemon-download" data-testid="daemon-download-section">
      <div className="daemon-download-header">
        <h1 className="daemon-download-title" data-testid="daemon-download-title">
          Get Started with MTGA Companion
        </h1>
        <p className="daemon-download-subtitle">
          Download the daemon for your platform to start tracking your MTG Arena matches,
          drafts, and collection in real time.
        </p>
      </div>

      <div className="daemon-download-buttons" data-testid="daemon-download-buttons">
        {DOWNLOAD_OPTIONS.map((option) => {
          const isDetected = option.platform === detectedPlatform;
          const href = buildDownloadUrl(option);
          return (
            <a
              key={option.platform}
              href={href}
              className={`daemon-download-button ${isDetected ? 'daemon-download-button--primary' : 'daemon-download-button--secondary'}`}
              data-testid={`download-link-${option.platform}`}
              download
              onClick={() => {
                trackEvent({
                  name: 'funnel_daemon_download_started',
                  properties: {
                    os: option.platform,
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
