import { useState } from 'react';
import LoadingButton from '../../LoadingButton';

/**
 * DangerZoneSection — top-level Settings accordion section for destructive
 * daemon-lifecycle actions (currently: uninstall daemon).
 *
 * Previously this lived as a sub-section inside DataRecoverySection.
 * Extracted in #2027 so that log-replay (Data Recovery) and daemon uninstall
 * (Danger Zone) are distinct, clearly labelled top-level concerns.
 *
 * The uninstall call delegates entirely to the onUninstallDaemon prop — no
 * direct fetch calls here (REST API adapter pattern per CLAUDE.md).
 *
 * The returned string from onUninstallDaemon is the backend's user-facing
 * residual-action message (platform-specific steps, e.g. "Drag VaultMTG to
 * the Trash to remove the app bundle"). It surfaces verbatim in the success
 * panel — the component does not fabricate its own copy.
 */

export interface DangerZoneSectionProps {
  isConnected: boolean;
  /**
   * Called when the user confirms the uninstall. The resolved string is the
   * backend's user-facing residual-action message rendered verbatim in the
   * success panel. When omitted, the section renders nothing (hidden).
   */
  onUninstallDaemon?: (purge: boolean) => Promise<string>;
}

export function DangerZoneSection({
  isConnected,
  onUninstallDaemon,
}: DangerZoneSectionProps) {
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
      const backendMessage = await onUninstallDaemon(purgeConfig);
      // Render the backend message verbatim — it carries the platform-specific
      // residual steps and reflects whether purge ran. Fall back to a neutral
      // message only if the backend returned an empty string.
      const message =
        backendMessage && backendMessage.trim().length > 0
          ? backendMessage
          : 'Daemon uninstall scheduled. The daemon will shut down momentarily — you can close this tab.';
      setUninstallResult({ kind: 'success', message });
    } catch (err) {
      setUninstallResult({
        kind: 'error',
        message:
          err instanceof Error ? err.message : 'Uninstall failed. Try the manual steps in the docs.',
      });
    } finally {
      setUninstalling(false);
      setConfirmingUninstall(false);
    }
  };

  if (!onUninstallDaemon) {
    return null;
  }

  return (
    <div className="settings-section" data-testid="danger-zone-section">
      <h2 className="section-title">Danger Zone — Uninstall Daemon</h2>
      <div className="setting-description settings-section-description">
        Stop the local daemon and remove its startup entry. Your VaultMTG account and cloud match
        history are not affected — that data lives on vaultmtg.app.
      </div>

      {uninstallResult ? (
        <div
          className={`setting-hint ${
            uninstallResult.kind === 'error' ? 'settings-error-box' : 'settings-success-box'
          }`}
          data-testid={
            uninstallResult.kind === 'error'
              ? 'danger-zone-error-result'
              : 'danger-zone-success-result'
          }
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
              data-testid="danger-zone-uninstall-button"
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
  );
}
