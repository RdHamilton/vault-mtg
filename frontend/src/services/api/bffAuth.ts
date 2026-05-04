/**
 * BFF auth adapter.
 *
 * Targets POST /api/keys on the BFF (outside the /api/v1 namespace).
 * Requires a daemon-issued JWT passed as the Authorization: Bearer header.
 *
 * This is fully wired once Vercel→BFF connectivity is resolved (ticket #1025).
 * Use VITE_BFF_URL env var to point at the BFF; defaults to http://localhost:8080.
 */

import { getApiConfig, ApiRequestError } from '../apiClient';

// ---------------------------------------------------------------------------
// Types matching the BFF createAPIKeyResponse JSON (api_keys.go)
// ---------------------------------------------------------------------------

export interface CreateAPIKeyRequest {
  /** Daemon-issued JWT used as the Authorization: Bearer token. */
  daemonToken: string;
}

export interface CreateAPIKeyResponse {
  key: string;
  created_at: string;
}

// ---------------------------------------------------------------------------
// Helper: BFF base URL (no /api/v1 suffix)
// ---------------------------------------------------------------------------

function bffBaseUrl(): string {
  if (import.meta.env.VITE_BFF_URL) {
    return import.meta.env.VITE_BFF_URL as string;
  }
  const { baseUrl } = getApiConfig();
  return baseUrl.replace(/\/api\/v1$/, '');
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

/**
 * Create a new BFF API key for the authenticated user.
 *
 * Targets: POST /api/keys
 *
 * The endpoint is protected by DaemonJWTAuth middleware on the BFF.  Pass the
 * daemon-issued JWT as `daemonToken`; it is sent as Authorization: Bearer.
 *
 * The returned plaintext key is shown exactly once — the BFF never stores it
 * in recoverable form.  Callers must persist it immediately (e.g. via setApiKey).
 */
export async function createAPIKey(
  request: CreateAPIKeyRequest
): Promise<CreateAPIKeyResponse> {
  const url = `${bffBaseUrl()}/api/keys`;

  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${request.daemonToken}`,
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

  return response.json() as Promise<CreateAPIKeyResponse>;
}

// Expose bffBaseUrl for testing.
export { bffBaseUrl };
