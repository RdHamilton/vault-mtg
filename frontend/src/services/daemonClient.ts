/**
 * Daemon API client — communicates with the local MTGA log-parsing daemon.
 *
 * Uses VITE_DAEMON_URL (defaults to http://localhost:9001/api/v1).
 * Port 9001 matches the daemon's localapi server (see
 * services/daemon/internal/localapi). It is also the same port hardcoded
 * in Setup.tsx for the /health probe, intentionally unified so the SPA
 * only ever has to discover one local daemon port.
 *
 * Import { get, post, put, del, getRaw } from this module for any route
 * served by the local daemon rather than the cloud BFF.
 *
 * Cloud/BFF routes (history, stats, draft-ratings, events, daemon/register)
 * must continue to import from ./apiClient.
 */

import type {
  ApiConfig,
  ApiError,
  RawGetResult,
} from './apiClient';
import { ApiRequestError, getApiKey } from './apiClient';

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

let config: ApiConfig = {
  baseUrl: import.meta.env.VITE_DAEMON_URL ?? 'http://localhost:9001/api/v1',
  timeout: 30000,
};

/**
 * Configure the daemon client (used in tests).
 */
export function configureDaemonApi(newConfig: Partial<ApiConfig>): void {
  config = { ...config, ...newConfig };
}

/**
 * Get the current daemon client configuration.
 */
export function getDaemonApiConfig(): ApiConfig {
  return { ...config };
}

// ---------------------------------------------------------------------------
// Auth header (same localStorage key as cloudClient)
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

export function put<T>(path: string, body?: unknown, options?: RequestInit): Promise<T> {
  return request<T>('PUT', path, body, options);
}

export function patch<T>(path: string, body?: unknown, options?: RequestInit): Promise<T> {
  return request<T>('PATCH', path, body, options);
}

export function del<T>(path: string, options?: RequestInit): Promise<T> {
  return request<T>('DELETE', path, undefined, options);
}

// ---------------------------------------------------------------------------
// Raw GET (exposes response headers — used by cards.ts for X-Cache-Degraded)
// ---------------------------------------------------------------------------

export async function getRaw<T>(path: string, options: RequestInit = {}): Promise<RawGetResult<T>> {
  const url = `${config.baseUrl}${path}`;

  const controller = new globalThis.AbortController();
  const timeoutId = setTimeout(() => controller.abort(), config.timeout);

  try {
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders(),
        ...options.headers,
      },
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
      throw new ApiRequestError(errorMessage, response.status, errorData.code, errorData.details);
    }

    if (response.status === 204) {
      return { data: undefined as T, headers: response.headers };
    }

    const json = await response.json();
    return { data: json.data as T, headers: response.headers };
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
// SSE helper (daemon events)
// ---------------------------------------------------------------------------

export function createDaemonSSEConnection(path: string): EventSource | null {
  const key = getApiKey();
  if (!key) {
    return null;
  }

  const url = new URL(`${config.baseUrl}${path}`);
  url.searchParams.set('token', key);
  return new EventSource(url.toString());
}
