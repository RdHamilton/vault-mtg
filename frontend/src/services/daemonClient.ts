/**
 * Daemon API client — communicates with the local MTGA log-parsing daemon.
 *
 * Uses VITE_DAEMON_URL (defaults to http://localhost:9001/api/v1).
 * Port 9001 matches the daemon's localapi server (see
 * services/daemon/internal/localapi). It is also the same port hardcoded
 * in Setup.tsx for the /health probe, intentionally unified so the SPA
 * only ever has to discover one local daemon port.
 *
 * Surface (post Phase 2 PR #15): `get` + `post` only. Two modules call us:
 *   - system.ts        (the surviving /system/* + /feedback/ml-training routes)
 *   - drafts.ts        (the 3 Bucket C live-state wrappers from PR #14)
 *
 * Phase 1 stripped the daemon's HTTP surface to a small set of paths; the
 * old PUT/PATCH/DELETE/SSE/getRaw helpers had no remaining callers and
 * were deleted to keep the boundary honest. If a future daemon endpoint
 * needs a verb that is not here, add it back deliberately.
 *
 * Cloud/BFF routes must continue to import from ./apiClient.
 */

import type { ApiConfig, ApiError } from './apiClient';
import { ApiRequestError, getApiKey } from './apiClient';

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const config: ApiConfig = {
  baseUrl: import.meta.env.VITE_DAEMON_URL ?? 'http://localhost:9001/api/v1',
  timeout: 30000,
};

// ---------------------------------------------------------------------------
// Auth header (same localStorage key as apiClient)
// ---------------------------------------------------------------------------

function authHeaders(): Record<string, string> {
  const key = getApiKey();
  return key ? { Authorization: `Bearer ${key}` } : {};
}

// ---------------------------------------------------------------------------
// Core request
// ---------------------------------------------------------------------------

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options: RequestInit = {}
): Promise<T> {
  const url = `${config.baseUrl}${path}`;

  const controller = new globalThis.AbortController();
  const timeoutId = setTimeout(() => controller.abort(), config.timeout);

  try {
    const response = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders(),
        ...options.headers,
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: controller.signal,
      ...options,
    });

    clearTimeout(timeoutId);

    if (!response.ok) {
      let errorData: ApiError = { error: 'Unknown error' };
      try {
        errorData = await response.json();
      } catch {
        errorData = { error: response.statusText || 'Request failed' };
      }
      const errorMessage = errorData.message || errorData.error;
      throw new ApiRequestError(
        errorMessage,
        response.status,
        errorData.code,
        errorData.details
      );
    }

    if (response.status === 204) {
      return undefined as T;
    }

    const data = await response.json();
    return data.data as T;
  } catch (error) {
    clearTimeout(timeoutId);

    if (error instanceof ApiRequestError) {
      throw error;
    }

    if (error instanceof Error) {
      if (error.name === 'AbortError') {
        throw new ApiRequestError('Request timeout', 408);
      }
      throw new ApiRequestError(error.message, 0);
    }

    throw new ApiRequestError('Unknown error', 0);
  }
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

export function get<T>(path: string, options?: RequestInit): Promise<T> {
  return request<T>('GET', path, undefined, options);
}

export function post<T>(path: string, body?: unknown, options?: RequestInit): Promise<T> {
  return request<T>('POST', path, body, options);
}
