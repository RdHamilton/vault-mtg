import { useState } from 'react';
import LoadingButton from '../../LoadingButton';
import { gui } from '@/types/models';

export interface DataRecoverySectionProps {
  isConnected: boolean;
  clearDataBeforeReplay: boolean;
  onClearDataBeforeReplayChange: (value: boolean) => void;
  isReplaying: boolean;
  replayProgress: gui.LogReplayProgress | null;
  onReplayLogs: () => void;
  /**
   * Phase 2 PR #18 — daemon uninstall hook. When omitted, the Danger
   * Zone subsection is hidden entirely so legacy callers / tests keep
   * working without the danger UI rendering.
   */
  onUninstallDaemon?: (purge: boolean) => Promise<void>;
}

export function DataRecoverySection({
  isConnected,
  clearDataBeforeReplay,
  onClearDataBeforeReplayChange,
  isReplaying,
  replayProgress,
  onReplayLogs,
  onUninstallDaemon,
}: DataRecoverySectionProps) {
  const [confirmingUninstall, setConfirmingUninstall] = useState(false);
  const [purgeConfig, setPurgeConfig] = useState(false);
  const [uninstalling, setUninstalling] = useState(false);
  const [uninstallResult, setUninstallResult] = useState<
    { kind: 'success'; message: string } | { kind: 'error'; message: string } | null
  >(null);

  const handleConfirmUninstall = async () => {
    if (!onUninstallDaemon) return;
    setUninstalling(true);
    try {
      await onUninstallDaemon(purgeConfig);
      setUninstallResult({
        kind: 'success',
        message:
          'Daemon uninstall scheduled. The daemon will shut down momentarily — you can close this tab.',
      });
    } catch (err) {
      setUninstallResult({
        kind: 'error',
        message: err instanceof Error ? err.message : 'Uninstall failed. Try the manual steps in the docs.',
      });
    } finally {
      setUninstalling(false);
      setConfirmingUninstall(false);
    }
  };

  return (
    <div className="settings-section">
      <h2 className="section-title">Data Recovery</h2>
      <div className="setting-description settings-section-description">
        Recover historical data from MTGA log files by replaying them through the daemon.
      </div>

      <div className="setting-item">
        <label className="setting-label">
          Replay All MTGA Logs
          <span className="setting-description">
            Auto-discover and process ALL log files from your MTGA installation directory in chronological order.
            Use this for complete recovery after fresh install or extended downtime. Requires daemon connection.
          </span>
        </label>
        <div className="setting-control">
          <div className="checkbox-container">
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={clearDataBeforeReplay}
                onChange={(e) => onClearDataBeforeReplayChange(e.target.checked)}
                className="checkbox-input"
                disabled={isReplaying}
              />
              <span>Clear all data before replay (recommended for first-time setup)</span>
            </label>
          </div>
          <LoadingButton
            loading={isReplaying}
            loadingText="Replaying Logs..."
            onClick={onReplayLogs}
            disabled={!isConnected}
            variant="primary"
          >
            Replay Historical Logs
          </LoadingButton>
          {!isConnected && (
            <div className="setting-hint settings-daemon-hint">
              Daemon must be running to replay logs
            </div>
          )}
        </div>
      </div>

      {(isReplaying || replayProgress) && (
        <div className="setting-item">
          <div className="replay-progress-container">
            <h3 className={`replay-progress-title ${isReplaying ? '' : 'complete'}`}>
              {isReplaying ? 'Replaying Historical Logs...' : '✓ Replay Complete'}
            </h3>
            {replayProgress && (
              <>
                <div className="settings-grid-2col">
                  <div>Files: {replayProgress.processedFiles || 0} / {replayProgress.totalFiles || 0}</div>
                  <div>Entries: {replayProgress.totalEntries || 0}</div>
                  <div>Matches: {replayProgress.matchesImported || 0}</div>
                  <div>Decks: {replayProgress.decksImported || 0}</div>
                  <div>Quests: {replayProgress.questsImported || 0}</div>
                  {replayProgress.duration && (
                    <div>Duration: {replayProgress.duration.toFixed(1)}s</div>
                  )}
                </div>
                {replayProgress.currentFile && isReplaying && (
                  <div className="current-file-display">
                    Current: {replayProgress.currentFile}
                  </div>
                )}
                {isReplaying && (
                  <div className="settings-progress-bar">
                    <div
                      className="settings-progress-bar-fill"
                      style={{ width: `${((replayProgress.processedFiles || 0) / (replayProgress.totalFiles || 1)) * 100}%` }}
                    ></div>
                  </div>
                )}
                {!isReplaying && (
                  <div className="refresh-message">
                    Page will refresh in 2 seconds to show imported data...
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      )}

      {onUninstallDaemon && (
        <div className="settings-subsection">
          <h3 className="subsection-title">Danger Zone — Uninstall Daemon</h3>
          <div className="setting-description">
            Stop the local daemon and remove its startup entry. Your VaultMTG account and
            cloud match history are not affected — that data lives on vaultmtg.app.
          </div>

          {uninstallResult ? (
            <div
              className={`setting-hint ${
                uninstallResult.kind === 'error' ? 'settings-error-box' : 'settings-success-box'
              }`}
            >
              {uninstallResult.message}
            </div>
          ) : confirmingUninstall ? (
            <div className="setting-item">
              <div className="setting-control">
                <div className="checkbox-container">
                  <label className="checkbox-label">
                    <input
                      type="checkbox"
                      checked={purgeConfig}
                      onChange={(e) => setPurgeConfig(e.target.checked)}
                      className="checkbox-input"
                      disabled={uninstalling}
                    />
                    <span>Also wipe my local config + cached data (irreversible)</span>
                  </label>
                </div>
                <LoadingButton
                  loading={uninstalling}
                  loadingText="Uninstalling..."
                  onClick={handleConfirmUninstall}
                  variant="danger"
                >
                  Confirm Uninstall
                </LoadingButton>
                <button
                  className="action-button"
                  onClick={() => {
                    setConfirmingUninstall(false);
                    setPurgeConfig(false);
                  }}
                  disabled={uninstalling}
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <div className="setting-item">
              <div className="setting-control">
                <button
                  className="danger-button"
                  onClick={() => setConfirmingUninstall(true)}
                  disabled={!isConnected}
                >
                  Uninstall VaultMTG Daemon
                </button>
                {!isConnected && (
                  <div className="setting-hint settings-daemon-hint">
                    Daemon must be running to trigger uninstall
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
