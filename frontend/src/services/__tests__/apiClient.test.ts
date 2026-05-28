import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  configureApi,
  getApiConfig,
  get,
  post,
  put,
  del,
  healthCheck,
  ApiRequestError,
  cloudClient,
  getApiKey,
  setApiKey,
  setClerkTokenProvider,
  resetErrorThrottle,
} from '../apiClient';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock analytics module so we can assert on trackEvent calls.
const mockTrackEvent = vi.fn();
vi.mock('../analytics', () => ({
  trackEvent: (...args: unknown[]) => mockTrackEvent(...args),
  Events: {
    ERROR_DATA_LOAD_FAILED: 'error_data_load_failed',
    ERROR_AUTH_FAILED: 'error_auth_failed',
  },
}));

describe('apiClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset to default config
    configureApi({
      baseUrl: 'http://localhost:8080/api/v1',
      timeout: 30000,
    });
    // Clear any stored API key between tests
    localStorage.clear();
    // Reset throttle state so tests are independent
    resetErrorThrottle();
  });

  afterEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    setClerkTokenProvider(null);
    resetErrorThrottle();
  });

  describe('configureApi', () => {
    it('should update API configuration', () => {
      configureApi({ baseUrl: 'http://example.com/api' });
      const config = getApiConfig();
      expect(config.baseUrl).toBe('http://example.com/api');
    });

    it('should merge with existing config', () => {
      configureApi({ timeout: 5000 });
      const config = getApiConfig();
      expect(config.timeout).toBe(5000);
      expect(config.baseUrl).toBe('http://localhost:8080/api/v1');
    });
  });

  describe('getApiKey / setApiKey', () => {
    it('returns empty string when no key is stored', () => {
      expect(getApiKey()).toBe('');
    });

    it('returns stored key after setApiKey', () => {
      setApiKey('test-key-abc');
      expect(getApiKey()).toBe('test-key-abc');
    });

    it('clears the key when empty string is passed', () => {
      setApiKey('some-key');
      setApiKey('');
      expect(getApiKey()).toBe('');
    });
  });

  describe('Authorization header injection', () => {
    it('should NOT include Authorization header when no API key is set', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { id: 1 } }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBeUndefined();
    });

    it('should include Authorization: Bearer header when API key is set', async () => {
      setApiKey('my-secret-key');

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { id: 1 } }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer my-secret-key');
    });

    it('should send Authorization header on POST requests', async () => {
      setApiKey('post-key');

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { success: true } }),
      });

      await post('/create', { name: 'test' });

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer post-key');
    });

    it('should send Authorization header on PUT requests', async () => {
      setApiKey('put-key');

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { updated: true } }),
      });

      await put('/items/1', { name: 'Updated' });

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer put-key');
    });

    it('should send Authorization header on DELETE requests', async () => {
      setApiKey('del-key');

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      await del('/items/1');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer del-key');
    });
  });

  describe('Clerk token provider', () => {
    it('prefers a Clerk session JWT over the legacy API key', async () => {
      setApiKey('legacy-key');
      setClerkTokenProvider(async () => 'clerk-jwt-xyz');

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: {} }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer clerk-jwt-xyz');
    });

    it('falls back to the legacy API key when provider returns null', async () => {
      setApiKey('legacy-key');
      setClerkTokenProvider(async () => null);

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: {} }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer legacy-key');
    });

    it('falls back to the legacy API key when provider throws', async () => {
      setApiKey('legacy-key');
      setClerkTokenProvider(async () => {
        throw new Error('clerk-error');
      });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: {} }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer legacy-key');
    });

    it('sends no Authorization header when provider returns null and no legacy key is stored', async () => {
      setClerkTokenProvider(async () => null);

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: {} }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBeUndefined();
    });

    it('reverts to legacy-key behavior after setClerkTokenProvider(null)', async () => {
      setApiKey('legacy-key');
      setClerkTokenProvider(async () => 'clerk-jwt');
      setClerkTokenProvider(null);

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: {} }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer legacy-key');
    });
  });

  describe('get', () => {
    it('should make a GET request', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { id: 1, name: 'Test' } }),
      });

      const result = await get('/test');

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/test',
        expect.objectContaining({
          method: 'GET',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        })
      );
      expect(result).toEqual({ id: 1, name: 'Test' });
    });

    it('should throw ApiRequestError on 404', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({ error: 'Resource not found' }),
      });

      try {
        await get('/nonexistent');
        expect.fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).status).toBe(404);
        expect((error as ApiRequestError).message).toBe('Resource not found');
      }
    });

    it('should throw ApiRequestError on 500', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: async () => ({ error: 'Server error', code: 'INTERNAL' }),
      });

      await expect(get('/error')).rejects.toThrow(ApiRequestError);
    });
  });

  describe('post', () => {
    it('should make a POST request with body', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { success: true } }),
      });

      const body = { name: 'Test', value: 123 };
      const result = await post('/create', body);

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/create',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(body),
        })
      );
      expect(result).toEqual({ success: true });
    });

    it('should handle 201 Created response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ data: { id: 'new-id' } }),
      });

      const result = await post('/items', { name: 'New Item' });
      expect(result).toEqual({ id: 'new-id' });
    });
  });

  describe('put', () => {
    it('should make a PUT request', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { updated: true } }),
      });

      const result = await put('/items/1', { name: 'Updated' });

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/items/1',
        expect.objectContaining({ method: 'PUT' })
      );
      expect(result).toEqual({ updated: true });
    });
  });

  describe('del', () => {
    it('should make a DELETE request', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      const result = await del('/items/1');

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/items/1',
        expect.objectContaining({ method: 'DELETE' })
      );
      expect(result).toBeUndefined();
    });
  });

  describe('healthCheck', () => {
    it('should return true when server is healthy', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
      });

      const result = await healthCheck();
      expect(result).toBe(true);
    });

    it('hits /healthz with the /api/v1 prefix stripped', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
      });

      await healthCheck();

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/healthz',
        expect.objectContaining({ method: 'GET' })
      );
    });

    it('should return false when server is unreachable', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      const result = await healthCheck();
      expect(result).toBe(false);
    });

    it('should return false when server returns error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 503,
      });

      const result = await healthCheck();
      expect(result).toBe(false);
    });
  });

  describe('error handling', () => {
    it('should handle network errors', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'));

      try {
        await get('/test');
        expect.fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).status).toBe(0);
        expect((error as ApiRequestError).message).toBe('Network error');
      }
    });

    it('should handle JSON parse errors in error response', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: async () => {
          throw new Error('Invalid JSON');
        },
      });

      try {
        await get('/test');
        expect.fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).status).toBe(500);
        expect((error as ApiRequestError).message).toBe('Internal Server Error');
      }
    });
  });
});

describe('ApiRequestError', () => {
  it('should create error with all properties', () => {
    const error = new ApiRequestError('Not found', 404, 'NOT_FOUND', 'Resource does not exist');

    expect(error.message).toBe('Not found');
    expect(error.status).toBe(404);
    expect(error.code).toBe('NOT_FOUND');
    expect(error.details).toBe('Resource does not exist');
    expect(error.name).toBe('ApiRequestError');
  });

  it('should be instanceof Error', () => {
    const error = new ApiRequestError('Test', 500);
    expect(error).toBeInstanceOf(Error);
    expect(error).toBeInstanceOf(ApiRequestError);
  });
});

describe('cloudClient alias', () => {
  it('is exported from apiClient', () => {
    expect(cloudClient).toBeDefined();
  });

  it('exposes get, post, put, patch, del helpers', () => {
    expect(typeof cloudClient.get).toBe('function');
    expect(typeof cloudClient.post).toBe('function');
    expect(typeof cloudClient.put).toBe('function');
    expect(typeof cloudClient.patch).toBe('function');
    expect(typeof cloudClient.del).toBe('function');
  });

  it('exposes getRaw helper', () => {
    expect(typeof cloudClient.getRaw).toBe('function');
  });

  it('exposes API key management helpers', () => {
    expect(typeof cloudClient.getApiKey).toBe('function');
    expect(typeof cloudClient.setApiKey).toBe('function');
  });

  it('exposes configuration helpers', () => {
    expect(typeof cloudClient.configureApi).toBe('function');
    expect(typeof cloudClient.getApiConfig).toBe('function');
  });

  it('exposes healthCheck', () => {
    expect(typeof cloudClient.healthCheck).toBe('function');
  });

  it('get is the same reference as the named export', () => {
    expect(cloudClient.get).toBe(get);
    expect(cloudClient.post).toBe(post);
    expect(cloudClient.put).toBe(put);
    expect(cloudClient.del).toBe(del);
  });
});

// ── error_data_load_failed analytics ──────────────────────────────────────────

describe('error_data_load_failed analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    configureApi({ baseUrl: 'http://localhost:8080/api/v1', timeout: 30000 });
    localStorage.clear();
    resetErrorThrottle();
  });

  afterEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    setClerkTokenProvider(null);
    resetErrorThrottle();
  });

  it('fires error_data_load_failed on 500 response', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: async () => ({ error: 'Server error' }),
    });

    await expect(get('/matches')).rejects.toThrow(ApiRequestError);

    expect(mockTrackEvent).toHaveBeenCalledWith({
      name: 'error_data_load_failed',
      properties: {
        page: expect.any(String),
        endpoint: '/matches',
        status_code: 500,
      },
    });
  });

  it('fires error_data_load_failed on 401 response', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      json: async () => ({ error: 'Unauthorized' }),
    });

    await expect(get('/matches')).rejects.toThrow(ApiRequestError);

    expect(mockTrackEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'error_data_load_failed',
        properties: expect.objectContaining({ status_code: 401 }),
      }),
    );
  });

  it('does NOT fire error_data_load_failed on 200 success', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ data: { id: 1 } }),
    });

    await get('/matches');

    const errorCalls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_data_load_failed',
    );
    expect(errorCalls).toHaveLength(0);
  });

  it('throttle: suppresses second emission for same (endpoint, status_code) within 10s', async () => {
    const failResponse = () => ({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: async () => ({ error: 'Unavailable' }),
    });

    mockFetch.mockResolvedValueOnce(failResponse());
    await expect(get('/matches')).rejects.toThrow();

    mockFetch.mockResolvedValueOnce(failResponse());
    await expect(get('/matches')).rejects.toThrow();

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_data_load_failed',
    );
    expect(calls).toHaveLength(1);
  });

  it('throttle: allows emission again after resetErrorThrottle (simulates window expiry)', async () => {
    const failResponse = () => ({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: async () => ({ error: 'Unavailable' }),
    });

    mockFetch.mockResolvedValueOnce(failResponse());
    await expect(get('/matches')).rejects.toThrow();

    resetErrorThrottle();

    mockFetch.mockResolvedValueOnce(failResponse());
    await expect(get('/matches')).rejects.toThrow();

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_data_load_failed',
    );
    expect(calls).toHaveLength(2);
  });

  it('skipErrorAnalytics: suppresses emission when flag is true', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: async () => ({ error: 'Server error' }),
    });

    await expect(
      get('/health/daemon', { skipErrorAnalytics: true } as RequestInit & { skipErrorAnalytics?: boolean }),
    ).rejects.toThrow(ApiRequestError);

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_data_load_failed',
    );
    expect(calls).toHaveLength(0);
  });

  it('NEGATIVE: error_data_load_failed payload never contains user_id', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      statusText: 'Not Found',
      json: async () => ({ error: 'Not found' }),
    });

    await expect(get('/decks')).rejects.toThrow();

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_data_load_failed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0].properties).not.toHaveProperty('user_id');
  });

  it('different endpoints are throttled independently', async () => {
    const fail = (status: number) => ({
      ok: false,
      status,
      statusText: 'Error',
      json: async () => ({ error: 'Error' }),
    });

    mockFetch.mockResolvedValueOnce(fail(500));
    await expect(get('/matches')).rejects.toThrow();

    mockFetch.mockResolvedValueOnce(fail(500));
    await expect(get('/decks')).rejects.toThrow();

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_data_load_failed',
    );
    expect(calls).toHaveLength(2);
    expect(calls[0][0].properties.endpoint).toBe('/matches');
    expect(calls[1][0].properties.endpoint).toBe('/decks');
  });
});

// ── error_auth_failed analytics ───────────────────────────────────────────────

describe('error_auth_failed analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    configureApi({ baseUrl: 'http://localhost:8080/api/v1', timeout: 30000 });
    localStorage.clear();
    resetErrorThrottle();
  });

  afterEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    setClerkTokenProvider(null);
    resetErrorThrottle();
  });

  it('fires error_auth_failed with reason_class network when Clerk token provider throws', async () => {
    setClerkTokenProvider(async () => {
      throw new Error('network error');
    });
    // With a throwing provider and no API key, authHeaders falls back; but
    // getClerkToken also catches and should emit.
    // We verify the event was fired after a request that exercises getClerkToken.
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ data: {} }),
    });

    await get('/test');

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_auth_failed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0]).toEqual({
      name: 'error_auth_failed',
      properties: { reason_class: 'network' },
    });
  });

  it('NEGATIVE: error_auth_failed payload never contains user_id', async () => {
    setClerkTokenProvider(async () => {
      throw new Error('network error');
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ data: {} }),
    });

    await get('/test');

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_auth_failed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0].properties).not.toHaveProperty('user_id');
  });

  it('does NOT fire error_auth_failed when token provider returns successfully', async () => {
    setClerkTokenProvider(async () => 'valid-token');
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ data: {} }),
    });

    await get('/test');

    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'error_auth_failed',
    );
    expect(calls).toHaveLength(0);
  });
});
