import { useState } from 'react';
import LoadingButton from '../../LoadingButton';
import { showToast } from '../../ToastContainer';

const DIAGNOSTICS_URL = 'http://127.0.0.1:9001/api/v1/system/diagnostics';

/**
 * Minimal typed shape of the daemon diagnostics response
 * (mirrors diagnosticsResponse in services/daemon/internal/localapi/diagnostics.go).
 */
interface DiagnosticsResponse {
  daemon_version: string;
  os: string;
  arch: string;
  uptime_seconds: number;
  started_at: string;
  cloud_api_url: string;
  session_id?: string;
  log_path: string;
  log_tail: string[];
  log_tail_error?: string;
}

export interface CopyDiagnosticsSectionProps {
  /**
   * Dependency-injected fetch function — defaults to globalThis.fetch.
   * Override in tests to avoid real network calls.
   */
  fetchFn?: typeof fetch;
  /**
   * Dependency-injected clipboard write function.
   * Override in tests to avoid interacting with the real clipboard API.
   */
  clipboardWriteFn?: (text: string) => Promise<void>;
}

/** Format the diagnostics blob as readable Markdown for Discord/support channels. */
function formatDiagnostics(data: DiagnosticsResponse): string {
  const lines: string[] = [
    '## VaultMTG Diagnostics',
    '',
    `**Daemon version:** ${data.daemon_version}`,
    `**OS / arch:** ${data.os} / ${data.arch}`,
    `**Uptime:** ${data.uptime_seconds}s (started ${data.started_at})`,
    `**Cloud API:** ${data.cloud_api_url}`,
  ];

  if (data.session_id) {
    lines.push(`**Session ID:** ${data.session_id}`);
  }

  lines.push(`**Log path:** ${data.log_path}`);

  if (data.log_tail_error) {
    lines.push(`**Log read error:** ${data.log_tail_error}`);
  }

  if (data.log_tail && data.log_tail.length > 0) {
    lines.push('', '### Last log lines', '```');
    lines.push(...data.log_tail);
    lines.push('```');
  }

  return lines.join('\n');
}

export function CopyDiagnosticsSection({
  fetchFn = globalThis.fetch.bind(globalThis),
  clipboardWriteFn = (text) => navigator.clipboard.writeText(text),
}: CopyDiagnosticsSectionProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCopyDiagnostics = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetchFn(DIAGNOSTICS_URL);

      if (!response.ok) {
        throw new Error(`Daemon returned ${response.status}`);
      }

      const data: DiagnosticsResponse = await response.json() as DiagnosticsResponse;
      const formatted = formatDiagnostics(data);

      await clipboardWriteFn(formatted);

      showToast.show('Diagnostics copied to clipboard!', 'success');
    } catch (err) {
      const message =
        err instanceof TypeError && err.message.includes('fetch')
          ? 'Daemon is not running. Start VaultMTG daemon and try again.'
          : err instanceof Error
          ? err.message
          : 'Failed to fetch diagnostics.';
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="settings-section" data-testid="copy-diagnostics-section">
      <h2 className="section-title">Copy Diagnostics</h2>
      <p className="setting-description settings-section-description">
        Copy a support bundle to your clipboard. Paste it in a{' '}
        Discord support channel or GitHub issue when reporting a bug. The bundle includes
        daemon version, OS, uptime, and the last 200 log lines — no credentials or account data.
      </p>

      <div className="setting-item">
        <div className="setting-control">
          <LoadingButton
            loading={loading}
            loadingText="Fetching diagnostics..."
            onClick={() => { void handleCopyDiagnostics(); }}
            variant="primary"
          >
            Copy Diagnostics
          </LoadingButton>
        </div>

        {error && (
          <div
            className="setting-hint settings-daemon-hint"
            data-testid="copy-diagnostics-error"
            role="alert"
          >
            {error}
          </div>
        )}
      </div>
    </div>
  );
}
