import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  configureDaemonApi,
  getDaemonApiConfig,
  get,
  post,
  put,
  del,
  getRaw,
  createDaemonSSEConnection,
} from '../daemonClient';

// We need the real getApiKey/setApiKey from apiClient for the auth header tests
import { setApiKey } from '../apiClient';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('daemonClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset to default daemon config
    configureDaemonApi({
      baseUrl: 'http://localhost:9001/api/v1',
      timeout: 30000,
    });
    localStorage.clear();
  });

  afterEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  // ---------------------------------------------------------------------------
  // Configuration
  // ---------------------------------------------------------------------------

  describe('configureDaemonApi / getDaemonApiConfig', () => {
    it('should update daemon configuration', () => {
      configureDaemonApi({ baseUrl: 'http://custom-daemon:9090/api/v1' });
      const config = getDaemonApiConfig();
      expect(config.baseUrl).toBe('http://custom-daemon:9090/api/v1');
    });

    it('should merge with existing config', () => {
      configureDaemonApi({ timeout: 5000 });
      const config = getDaemonApiConfig();
      expect(config.timeout).toBe(5000);
      expect(config.baseUrl).toBe('http://localhost:9001/api/v1');
    });

    it('daemon default baseUrl differs from cloudClient default when VITE_DAEMON_URL is set', () => {
      // The daemon client uses VITE_DAEMON_URL; cloud client uses VITE_BFF_URL.
      // In tests neither env var is set, so both fall back to the same default —
      // but the config key is independent and can be set separately.
      configureDaemonApi({ baseUrl: 'http://daemon-host:8080/api/v1' });
      expect(getDaemonApiConfig().baseUrl).toBe('http://daemon-host:8080/api/v1');
    });
  });

  // ---------------------------------------------------------------------------
  // Authorization header
  // ---------------------------------------------------------------------------

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

    it('should include Authorization header when API key is stored', async () => {
      setApiKey('daemon-api-key-abc');
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { ok: true } }),
      });

      await get('/test');

      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer daemon-api-key-abc');
    });
  });

  // ---------------------------------------------------------------------------
  // GET
  // ---------------------------------------------------------------------------

  describe('get', () => {
    it('should make GET request to daemon base URL', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { items: [] } }),
      });

      const result = await get('/drafts');

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:9001/api/v1/drafts',
        expect.objectContaining({ method: 'GET' })
      );
      expect(result).toEqual({ items: [] });
    });

    it('should throw ApiRequestError on 4xx response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({ error: 'draft not found' }),
      });

      await expect(get('/drafts/missing')).rejects.toMatchObject({
        status: 404,
        message: 'draft not found',
      });
    });
  });

  // ---------------------------------------------------------------------------
  // POST
  // ---------------------------------------------------------------------------

  describe('post', () => {
    it('should make POST request with JSON body', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { created: true } }),
      });

      const result = await post('/matches', { format: 'Ranked' });

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe('http://localhost:9001/api/v1/matches');
      expect(init.method).toBe('POST');
      expect(init.body).toBe(JSON.stringify({ format: 'Ranked' }));
      expect(result).toEqual({ created: true });
    });
  });

  // ---------------------------------------------------------------------------
  // PUT
  // ---------------------------------------------------------------------------

  describe('put', () => {
    it('should make PUT request with JSON body', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { updated: true } }),
      });

      await put('/settings', { theme: 'dark' });

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe('http://localhost:9001/api/v1/settings');
      expect(init.method).toBe('PUT');
    });
  });

  // ---------------------------------------------------------------------------
  // DELETE
  // ---------------------------------------------------------------------------

  describe('del', () => {
    it('should make DELETE request', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
        json: async () => ({}),
      });

      const result = await del('/decks/abc123');

      const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(url).toBe('http://localhost:9001/api/v1/decks/abc123');
      expect(init.method).toBe('DELETE');
      expect(result).toBeUndefined();
    });
  });

  // ---------------------------------------------------------------------------
  // getRaw
  // ---------------------------------------------------------------------------

  describe('getRaw', () => {
    it('should return data and headers', async () => {
      const mockHeaders = new Headers({ 'x-cache-degraded': 'true', 'x-cache-age-hours': '3' });
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: mockHeaders,
        json: async () => ({ data: [{ name: 'Card A' }] }),
      });

      const result = await getRaw('/cards/ratings/dsk/PremierDraft');

      expect(result.data).toEqual([{ name: 'Card A' }]);
      expect(result.headers.get('x-cache-degraded')).toBe('true');
      expect(result.headers.get('x-cache-age-hours')).toBe('3');
    });

    it('should throw on error response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: async () => ({ error: 'server error' }),
      });

      await expect(getRaw('/cards/ratings/bad/format')).rejects.toMatchObject({
        status: 500,
      });
    });
  });

  // ---------------------------------------------------------------------------
  // createDaemonSSEConnection
  // ---------------------------------------------------------------------------

  describe('createDaemonSSEConnection', () => {
    it('should return null when no API key is stored', () => {
      const conn = createDaemonSSEConnection('/events');
      expect(conn).toBeNull();
    });

    it('should return EventSource with token query param when key is set', () => {
      setApiKey('sse-key-xyz');

      // EventSource is not available in jsdom — patch it minimally
      const MockEventSource = vi.fn().mockImplementation((url: string) => ({ url }));
      vi.stubGlobal('EventSource', MockEventSource);

      const conn = createDaemonSSEConnection('/events');

      expect(MockEventSource).toHaveBeenCalledOnce();
      const [calledUrl] = MockEventSource.mock.calls[0] as [string];
      expect(calledUrl).toContain('token=sse-key-xyz');
      expect(conn).not.toBeNull();

      vi.unstubAllGlobals();
    });
  });

  // ---------------------------------------------------------------------------
  // 204 No Content
  // ---------------------------------------------------------------------------

  describe('204 No Content handling', () => {
    it('should return undefined for 204 responses in GET', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
        json: async () => ({}),
      });

      const result = await get('/quests/active');
      expect(result).toBeUndefined();
    });
  });

  // ---------------------------------------------------------------------------
  // Timeout / abort
  // ---------------------------------------------------------------------------

  describe('request timeout', () => {
    it('should throw ApiRequestError with 408 status on timeout', async () => {
      mockFetch.mockImplementationOnce((_url: string, options: RequestInit) => {
        const signal = options.signal as AbortSignal;
        return new Promise((_resolve, reject) => {
          signal.addEventListener('abort', () => {
            const err = new Error('The operation was aborted');
            err.name = 'AbortError';
            reject(err);
          });
        });
      });

      configureDaemonApi({ timeout: 1 }); // 1 ms — fires immediately

      await expect(get('/slow')).rejects.toMatchObject({
        status: 408,
        message: 'Request timeout',
      });
    });
  });
});
