/**
 * Base API client for REST API communication.
 * Replaces direct Wails function calls with HTTP requests.
 */

export interface ApiConfig {
  baseUrl: string;
  timeout?: number;
}

export interface ApiResponse<T> {
  data: T;
  error?: string;
}

export interface ApiError {
  error: string;
  message?: string;
  code?: number | string;
  details?: string;
}

// Default configuration - can be overridden
let config: ApiConfig = {
  baseUrl: 'http://localhost:8080/api/v1',
  timeout: 30000,
};

// ---------------------------------------------------------------------------
// API Key management
// ---------------------------------------------------------------------------

/** localStorage key under which the BFF API key is persisted. */
const API_KEY_STORAGE_KEY = 'mtga-companion-api-key';

/**
 * Retrieve the stored API key from localStorage.
 * Returns an empty string when no key is present.
 */
export function getApiKey(): string {
  try {
    return localStorage.getItem(API_KEY_STORAGE_KEY) ?? '';
  } catch {
    return '';
  }
}

/**
 * Persist an API key to localStorage so it survives page reloads.
 * Pass an empty string to clear the stored key.
 */
export function setApiKey(key: string): void {
  try {
    if (key) {
      localStorage.setItem(API_KEY_STORAGE_KEY, key);
    } else {
      localStorage.removeItem(API_KEY_STORAGE_KEY);
    }
  } catch {
    // Storage unavailable — silently ignore.
  }
}

/**
 * Build the Authorization header object when an API key is stored.
 * Returns an empty object when no key is present so callers can spread safely.
 */
function authHeaders(): Record<string, string> {
  const key = getApiKey();
  return key ? { Authorization: `Bearer ${key}` } : {};
}

/**
 * Configure the API client.
 */
export function configureApi(newConfig: Partial<ApiConfig>): void {
  config = { ...config, ...newConfig };
}

/**
 * Get the current API configuration.
 */
export function getApiConfig(): ApiConfig {
  return { ...config };
}

/**
 * Custom error class for API errors.
 */
export class ApiRequestError extends Error {
  public readonly status: number;
  public readonly code?: number | string;
  public readonly details?: string;

  constructor(message: string, status: number, code?: number | string, details?: string) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

/**
 * Make an HTTP request to the API.
 */
async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options: RequestInit = {}
): Promise<T> {
  const url = `${config.baseUrl}${path}`;

  const controller = new AbortController();
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
      // Use message field if available (contains actual error details), otherwise use error field
      const errorMessage = errorData.message || errorData.error;
      throw new ApiRequestError(
        errorMessage,
        response.status,
        errorData.code,
        errorData.details
      );
    }

    // Handle 204 No Content
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

/**
 * HTTP GET request.
 */
export function get<T>(path: string, options?: RequestInit): Promise<T> {
  return request<T>('GET', path, undefined, options);
}

/**
 * HTTP POST request.
 */
export function post<T>(path: string, body?: unknown, options?: RequestInit): Promise<T> {
  return request<T>('POST', path, body, options);
}

/**
 * HTTP PUT request.
 */
export function put<T>(path: string, body?: unknown, options?: RequestInit): Promise<T> {
  return request<T>('PUT', path, body, options);
}

/**
 * HTTP PATCH request.
 */
export function patch<T>(path: string, body?: unknown, options?: RequestInit): Promise<T> {
  return request<T>('PATCH', path, body, options);
}

/**
 * HTTP DELETE request.
 */
export function del<T>(path: string, options?: RequestInit): Promise<T> {
  return request<T>('DELETE', path, undefined, options);
}

/**
 * Result from a raw GET request that exposes response headers.
 */
export interface RawGetResult<T> {
  data: T;
  headers: Headers;
}

/**
 * HTTP GET request that returns both parsed data and raw response headers.
 * Use this when you need to inspect response headers (e.g. X-Cache-Degraded).
 */
export async function getRaw<T>(path: string, options: RequestInit = {}): Promise<RawGetResult<T>> {
  const url = `${config.baseUrl}${path}`;

  const controller = new AbortController();
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

/**
 * Check if the API server is reachable.
 */
export async function healthCheck(): Promise<boolean> {
  try {
    const response = await fetch(`${config.baseUrl.replace('/api/v1', '')}/health`, {
      method: 'GET',
    });
    return response.ok;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// SSE helper
// ---------------------------------------------------------------------------

/**
 * Open a Server-Sent Events connection to the given path.
 *
 * The connection is gated on an API key being present in storage — if no key
 * is stored the function returns null and the caller decides how to handle the
 * unauthenticated state.
 *
 * EventSource does not support custom request headers natively, so the API key
 * is appended as a `token` query-parameter which the BFF SSE handler reads and
 * maps to a Bearer credential.  This is the standard workaround for SSE auth.
 *
 * @param path  API path relative to baseUrl (e.g. "/events")
 * @returns     A connected EventSource, or null when no API key is stored.
 */
export function createSSEConnection(path: string): EventSource | null {
  const key = getApiKey();
  if (!key) {
    return null;
  }

  const url = new URL(`${config.baseUrl}${path}`);
  url.searchParams.set('token', key);
  return new EventSource(url.toString());
}
