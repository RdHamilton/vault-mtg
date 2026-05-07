/**
 * BFF daemon health adapter.
 *
 * Targets GET /api/v1/health/daemon on the BFF.
 * The endpoint is Clerk-protected and returns a simple status object.
 *
 * Response shape: { "status": "connected" | "disconnected" | "reconnecting" }
 */

import { getApiConfig, ApiRequestError } from '../apiClient';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type DaemonHealthStatus = 'connected' | 'disconnected' | 'reconnecting';

export interface DaemonHealthResponse {
  status: DaemonHealthStatus;
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

/**
 * Fetch the daemon health status from the BFF.
 *
 * Targets: GET /api/v1/health/daemon
 *
 * The endpoint is Clerk-protected — pass the Clerk session token obtained via
 * `useAuth().getToken()` as the `clerkToken` parameter.
 *
 * @param clerkToken  Clerk session JWT returned by useAuth().getToken()
 */
export async function getDaemonHealth(
  clerkToken: string
): Promise<DaemonHealthResponse> {
  const { baseUrl } = getApiConfig();
  const url = `${baseUrl}/health/daemon`;

  const response = await fetch(url, {
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

  return response.json() as Promise<DaemonHealthResponse>;
}
