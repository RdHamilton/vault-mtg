/**
 * BFF daemons adapter — Connected Devices (#2632)
 *
 * Targets:
 *   GET    /api/v1/daemons              — list caller's active daemon registrations
 *   DELETE /api/v1/daemons/{device_id}  — soft-revoke a daemon registration
 *
 * Both endpoints are Clerk-protected (RequireClerkAuth on the BFF).
 * The Clerk session JWT is sourced via useAuth().getToken() at the call site
 * and forwarded as Authorization: Bearer per ADR-009.
 *
 * API contract: ADR-031 §3 + §4.
 *
 * 404 on DELETE is collapsed to success per ADR-031 §3 — the device is
 * already revoked / does not belong to this tenant; either way the desired
 * end-state (device not active) is achieved.
 *
 * last_used_at is typed in DaemonDevice so TypeScript stays honest about the
 * API shape, but it is NEVER rendered in the UI (Ray Q4 binding, #2632).
 */

import { getApiConfig, ApiRequestError } from '../apiClient';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface DaemonDevice {
  device_id: string;
  platform: string;
  daemon_ver: string;
  paired_at: string;
  /** Typed but NEVER rendered — Ray Q4 binding, #2632 */
  last_used_at: string | null;
}

export interface ListDaemonsResponse {
  devices: DaemonDevice[];
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

/**
 * List the caller's active daemon registrations.
 *
 * Targets: GET /api/v1/daemons
 *
 * @param clerkToken  Clerk session JWT from useAuth().getToken()
 */
export async function listDaemons(clerkToken: string): Promise<ListDaemonsResponse> {
  const { baseUrl } = getApiConfig();

  const response = await fetch(`${baseUrl}/daemons`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${clerkToken}`,
    },
  });

  if (!response.ok) {
    let errorMessage = response.statusText || 'Request failed';
    try {
      const body = (await response.json()) as { error?: string; message?: string };
      errorMessage = body.message ?? body.error ?? errorMessage;
    } catch {
      // ignore parse failure — use statusText
    }
    throw new ApiRequestError(errorMessage, response.status);
  }

  return response.json() as Promise<ListDaemonsResponse>;
}

/**
 * Soft-revoke a daemon registration by device_id.
 *
 * Targets: DELETE /api/v1/daemons/{device_id}
 *
 * Returns void on 204. 404 is collapsed to success per ADR-031 §3:
 * not-found / cross-tenant / already-revoked all share the same desired
 * end-state and must not leak distinguishable information to the caller.
 *
 * @param deviceId    The device_id to revoke
 * @param clerkToken  Clerk session JWT from useAuth().getToken()
 */
export async function revokeDaemon(deviceId: string, clerkToken: string): Promise<void> {
  const { baseUrl } = getApiConfig();

  const response = await fetch(`${baseUrl}/daemons/${encodeURIComponent(deviceId)}`, {
    method: 'DELETE',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${clerkToken}`,
    },
  });

  // 204 = success; 404 = already-revoked / not-found — both are treated as success
  if (response.status === 204 || response.status === 404) {
    return;
  }

  if (!response.ok) {
    let errorMessage = response.statusText || 'Request failed';
    try {
      const body = (await response.json()) as { error?: string; message?: string };
      errorMessage = body.message ?? body.error ?? errorMessage;
    } catch {
      // ignore parse failure — use statusText
    }
    throw new ApiRequestError(errorMessage, response.status);
  }
}
