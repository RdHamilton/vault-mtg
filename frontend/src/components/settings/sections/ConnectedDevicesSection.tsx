/**
 * ConnectedDevicesSection — Settings accordion section (#2632)
 *
 * Displays the authenticated user's active daemon registrations and provides
 * a per-row Revoke action. Mounted once on settings open; one-time fetch on
 * mount (no refresh button per plan).
 *
 * States:
 *   loading    — spinner while GET /api/v1/daemons is in flight
 *   populated  — list of device rows; each row has Revoke button
 *   empty      — "No devices connected." when devices array is empty
 *   load error — error message when the list fetch fails
 *   row error  — per-row error when a revoke call fails (row stays in list)
 *
 * Out-of-scope (never rendered):
 *   - last_used_at (Ray Q4 binding)
 *   - full device_id UUID (truncated to 8 chars + ellipsis)
 *   - confirm dialog on revoke (plan default: no confirm)
 *   - refresh button (out of scope)
 */

import { useState, useEffect } from 'react';
import { useAuth } from '@clerk/react';
import { listDaemons, revokeDaemon } from '@/services/api/bffDaemons';
import type { DaemonDevice } from '@/services/api/bffDaemons';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Truncate a device_id UUID to the first 8 characters followed by an ellipsis.
 * Never renders the full UUID.
 */
function truncateDeviceId(deviceId: string): string {
  return `${deviceId.slice(0, 8)}…`;
}

/**
 * Format an ISO date string to a human-readable date only (no time).
 * Falls back to the raw string if parsing fails.
 */
function formatPairedAt(isoString: string): string {
  try {
    return new Date(isoString).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return isoString;
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ConnectedDevicesSection() {
  const { getToken } = useAuth();

  const [devices, setDevices] = useState<DaemonDevice[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  // Per-row revoke errors keyed by device_id
  const [revokeErrors, setRevokeErrors] = useState<Record<string, string>>({});

  // One-time fetch on mount. getToken is intentionally excluded from deps:
  // this is a one-shot load (no refresh per plan), and Clerk's getToken is
  // stable in production. Including it would cause re-fetch on every render
  // in test environments where the mock returns new references.
  useEffect(() => {
    let cancelled = false;

    const fetchDevices = async () => {
      try {
        const token = await getToken();
        if (!token) throw new Error('Not authenticated');
        const result = await listDaemons(token);
        if (!cancelled) {
          setDevices(result.devices);
        }
      } catch (err) {
        if (!cancelled) {
          setLoadError(
            err instanceof Error ? err.message : 'Failed to load connected devices.'
          );
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void fetchDevices();
    return () => {
      cancelled = true;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleRevoke = async (deviceId: string) => {
    // Clear any previous error for this row
    setRevokeErrors((prev) => {
      const next = { ...prev };
      delete next[deviceId];
      return next;
    });

    try {
      const token = await getToken();
      if (!token) throw new Error('Not authenticated');
      await revokeDaemon(deviceId, token);
      // Optimistic removal on success (204 or 404)
      setDevices((prev) => prev.filter((d) => d.device_id !== deviceId));
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Failed to revoke device. Please try again.';
      setRevokeErrors((prev) => ({ ...prev, [deviceId]: message }));
    }
  };

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  if (loading) {
    return (
      <div className="settings-section">
        <h2 className="section-title">Connected Devices</h2>
        <div data-testid="connected-devices-loading" className="connected-devices-loading">
          <span className="spinner" aria-label="Loading connected devices" />
        </div>
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="settings-section">
        <h2 className="section-title">Connected Devices</h2>
        <div data-testid="connected-devices-error" className="connected-devices-error">
          {loadError}
        </div>
      </div>
    );
  }

  if (devices.length === 0) {
    return (
      <div className="settings-section">
        <h2 className="section-title">Connected Devices</h2>
        <p data-testid="connected-devices-empty" className="connected-devices-empty">
          No devices connected.
        </p>
      </div>
    );
  }

  return (
    <div className="settings-section">
      <h2 className="section-title">Connected Devices</h2>
      <p className="connected-devices-description">
        These are the devices currently paired with your account via the VaultMTG daemon.
        Revoking a device will disconnect it on its next heartbeat.
      </p>
      <div className="connected-devices-list">
        {devices.map((device, index) => (
          <div
            key={device.device_id}
            data-testid="device-row"
            className="device-row"
          >
            <div
              data-testid={`device-row-${index}`}
              className="device-row-inner"
            >
              <div className="device-info">
                <span className="device-id">
                  {truncateDeviceId(device.device_id)}
                </span>
                <span className="device-platform">{device.platform}</span>
                <span className="device-paired-at">
                  Paired {formatPairedAt(device.paired_at)}
                </span>
              </div>
              <div className="device-actions">
                <button
                  data-testid={`revoke-button-${index}`}
                  className="action-button action-button--danger"
                  onClick={() => void handleRevoke(device.device_id)}
                  aria-label={`Revoke device ${truncateDeviceId(device.device_id)}`}
                >
                  Revoke
                </button>
              </div>
            </div>
            {revokeErrors[device.device_id] && (
              <div
                data-testid={`revoke-error-${index}`}
                className="device-revoke-error"
                role="alert"
              >
                {revokeErrors[device.device_id]}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
