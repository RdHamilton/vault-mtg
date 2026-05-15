import LoadingButton from '../../LoadingButton';
import { gui } from '@/types/models';

export interface DataRecoverySectionProps {
  isConnected: boolean;
  clearDataBeforeReplay: boolean;
  onClearDataBeforeReplayChange: (value: boolean) => void;
  isReplaying: boolean;
  replayProgress: gui.LogReplayProgress | null;
  onReplayLogs: () => void;
}

export function DataRecoverySection({
  isConnected,
  clearDataBeforeReplay,
  onClearDataBeforeReplayChange,
  isReplaying,
  replayProgress,
  onReplayLogs,
}: DataRecoverySectionProps) {
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

    </div>
  );
}
