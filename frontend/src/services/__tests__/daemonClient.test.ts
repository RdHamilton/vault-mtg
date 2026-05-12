import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get, post } from '../daemonClient';

// We need the real getApiKey/setApiKey from apiClient for the auth header tests
import { setApiKey } from '../apiClient';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('daemonClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  afterEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
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
    it('should throw ApiRequestError with 408 status on AbortError', async () => {
      mockFetch.mockImplementationOnce(() => {
        const err = new Error('The operation was aborted');
        err.name = 'AbortError';
        return Promise.reject(err);
      });

      await expect(get('/slow')).rejects.toMatchObject({
        status: 408,
        message: 'Request timeout',
      });
    });
  });
});
