import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Unmock so we test the real SSE implementation
vi.unmock('@/services/websocketClient');

// Mock apiClient — websocketClient now reads the Clerk JWT via getClerkToken()
// (the legacy getApiKey path was retired with issue #1904).  Default returns
// a stable test JWT so all tests get an Authorization header by default;
// individual tests can override via `vi.mocked(getClerkToken).mockResolvedValueOnce(...)`.
vi.mock('@/services/apiClient', () => ({
  getApiKey: vi.fn(() => 'test-api-key'),
  setApiKey: vi.fn(),
  getClerkToken: vi.fn(() => Promise.resolve('test-clerk-jwt')),
  configureApi: vi.fn(),
  getApiConfig: vi.fn(() => ({ baseUrl: 'http://localhost:8080/api/v1' })),
  healthCheck: vi.fn(() => Promise.resolve(true)),
  ApiRequestError: class ApiRequestError extends Error {},
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  patch: vi.fn(),
  del: vi.fn(),
}));

import {
  configureWebSocket,
  getWebSocketConfig,
  connect,
  disconnect,
  EventsOn,
  EventsOnce,
  EventsOff,
  EventsEmit,
  getListenerCount,
  getRegisteredEventTypes,
} from '../websocketClient';

describe('SSE client (websocketClient)', () => {
  beforeEach(() => {
    configureWebSocket({
      url: 'http://localhost:8080/api/v1/events',
      reconnectInterval: 3000,
      maxReconnectAttempts: 10,
    });

    const eventTypes = getRegisteredEventTypes();
    if (eventTypes.length > 0) {
      EventsOff(eventTypes[0], ...eventTypes.slice(1));
    }
  });

  afterEach(() => {
    disconnect();
    vi.clearAllMocks();
  });

  describe('configureWebSocket', () => {
    it('updates the SSE URL', () => {
      configureWebSocket({ url: 'http://example.com/api/v1/events' });
      expect(getWebSocketConfig().url).toBe('http://example.com/api/v1/events');
    });

    it('merges with existing config', () => {
      configureWebSocket({ reconnectInterval: 5000 });
      const cfg = getWebSocketConfig();
      expect(cfg.reconnectInterval).toBe(5000);
      expect(cfg.url).toBe('http://localhost:8080/api/v1/events');
    });
  });

  describe('EventsOn', () => {
    it('registers a listener', () => {
      EventsOn('test:event', vi.fn());
      expect(getListenerCount('test:event')).toBe(1);
    });

    it('allows multiple listeners for the same event', () => {
      EventsOn('test:event', vi.fn());
      EventsOn('test:event', vi.fn());
      expect(getListenerCount('test:event')).toBe(2);
    });

    it('returns an unsubscribe function', () => {
      const cb = vi.fn();
      const unsub = EventsOn('test:event', cb);
      expect(getListenerCount('test:event')).toBe(1);
      unsub();
      expect(getListenerCount('test:event')).toBe(0);
    });

    it('invokes callback when event is emitted', () => {
      const cb = vi.fn();
      EventsOn('test:event', cb);
      EventsEmit('test:event', { value: 42 });
      expect(cb).toHaveBeenCalledWith({ value: 42 });
    });

    it('invokes all listeners when event is emitted', () => {
      const cb1 = vi.fn();
      const cb2 = vi.fn();
      EventsOn('test:event', cb1);
      EventsOn('test:event', cb2);
      EventsEmit('test:event', { data: 'test' });
      expect(cb1).toHaveBeenCalledWith({ data: 'test' });
      expect(cb2).toHaveBeenCalledWith({ data: 'test' });
    });
  });

  describe('EventsOnce', () => {
    it('calls callback only once', () => {
      const cb = vi.fn();
      EventsOnce('test:event', cb);
      EventsEmit('test:event', { first: true });
      EventsEmit('test:event', { second: true });
      expect(cb).toHaveBeenCalledTimes(1);
      expect(cb).toHaveBeenCalledWith({ first: true });
    });

    it('auto-unsubscribes after first event', () => {
      const cb = vi.fn();
      EventsOnce('test:event', cb);
      expect(getListenerCount('test:event')).toBe(1);
      EventsEmit('test:event', {});
      expect(getListenerCount('test:event')).toBe(0);
    });

    it('returned unsubscribe prevents the callback from firing', () => {
      const cb = vi.fn();
      const unsub = EventsOnce('test:event', cb);
      unsub();
      EventsEmit('test:event', {});
      expect(cb).not.toHaveBeenCalled();
    });
  });

  describe('EventsOff', () => {
    it('removes all listeners for an event', () => {
      EventsOn('test:event', vi.fn());
      EventsOn('test:event', vi.fn());
      EventsOff('test:event');
      expect(getListenerCount('test:event')).toBe(0);
    });

    it('removes listeners for multiple events at once', () => {
      EventsOn('event1', vi.fn());
      EventsOn('event2', vi.fn());
      EventsOn('event3', vi.fn());
      EventsOff('event1', 'event2');
      expect(getListenerCount('event1')).toBe(0);
      expect(getListenerCount('event2')).toBe(0);
      expect(getListenerCount('event3')).toBe(1);
    });
  });

  describe('EventsEmit', () => {
    it('does not throw when no listeners exist', () => {
      expect(() => EventsEmit('nonexistent', { data: 'test' })).not.toThrow();
    });

    it('handles listener errors gracefully', () => {
      const errorCb = vi.fn(() => {
        throw new Error('Listener error');
      });
      const normalCb = vi.fn();
      EventsOn('test:event', errorCb);
      EventsOn('test:event', normalCb);
      expect(() => EventsEmit('test:event', {})).not.toThrow();
      expect(errorCb).toHaveBeenCalled();
      expect(normalCb).toHaveBeenCalled();
    });
  });

  describe('wildcard listener (*)', () => {
    it('receives all events as { type, data }', () => {
      const cb = vi.fn();
      EventsOn('*', cb);
      EventsEmit('event1', { a: 1 });
      EventsEmit('event2', { b: 2 });
      expect(cb).toHaveBeenCalledTimes(2);
      expect(cb).toHaveBeenCalledWith({ type: 'event1', data: { a: 1 } });
      expect(cb).toHaveBeenCalledWith({ type: 'event2', data: { b: 2 } });
    });
  });

  describe('getRegisteredEventTypes', () => {
    it('returns all registered types', () => {
      EventsOn('event1', vi.fn());
      EventsOn('event2', vi.fn());
      EventsOn('event3', vi.fn());
      const types = getRegisteredEventTypes();
      expect(types).toContain('event1');
      expect(types).toContain('event2');
      expect(types).toContain('event3');
      expect(types).toHaveLength(3);
    });

    it('returns empty array when no listeners are registered', () => {
      expect(getRegisteredEventTypes()).toEqual([]);
    });
  });

  describe('real-world event scenarios', () => {
    it('handles stats:updated', () => {
      const cb = vi.fn();
      EventsOn('stats:updated', cb);
      EventsEmit('stats:updated', { matches: 10, games: 25 });
      expect(cb).toHaveBeenCalledWith({ matches: 10, games: 25 });
    });

    it('handles draft:updated', () => {
      const cb = vi.fn();
      EventsOn('draft:updated', cb);
      EventsEmit('draft:updated', { count: 5, picks: 42 });
      expect(cb).toHaveBeenCalledWith({ count: 5, picks: 42 });
    });

    it('handles replay:progress', () => {
      const cb = vi.fn();
      EventsOn('replay:progress', cb);
      EventsEmit('replay:progress', {
        current: 50,
        total: 100,
        percentage: 50.0,
        currentFile: 'log1.log',
      });
      expect(cb).toHaveBeenCalledWith({
        current: 50,
        total: 100,
        percentage: 50.0,
        currentFile: 'log1.log',
      });
    });

    it('handles task:progress', () => {
      const cb = vi.fn();
      EventsOn('task:progress', cb);
      EventsEmit('task:progress', { id: 'sync-1', progress: 50, title: 'Syncing cards' });
      expect(cb).toHaveBeenCalledWith({ id: 'sync-1', progress: 50, title: 'Syncing cards' });
    });

    it('handles download:progress', () => {
      const cb = vi.fn();
      EventsOn('download:progress', cb);
      EventsEmit('download:progress', { id: 'dl-1', progress: 75, description: 'Downloading set' });
      expect(cb).toHaveBeenCalledWith({
        id: 'dl-1',
        progress: 75,
        description: 'Downloading set',
      });
    });
  });

  describe('connect() — fetch-based SSE', () => {
    it('sends Accept: text/event-stream header', async () => {
      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        body: {
          getReader: () => ({
            read: vi.fn().mockResolvedValue({ done: true, value: undefined }),
          }),
        },
      });
      vi.stubGlobal('fetch', fetchMock);

      await connect();

      expect(fetchMock).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            Accept: 'text/event-stream',
          }),
        }),
      );

      vi.unstubAllGlobals();
    });

    it('sends Authorization: Bearer header with the Clerk JWT', async () => {
      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        body: {
          getReader: () => ({
            read: vi.fn().mockResolvedValue({ done: true, value: undefined }),
          }),
        },
      });
      vi.stubGlobal('fetch', fetchMock);

      await connect();

      expect(fetchMock).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: 'Bearer test-clerk-jwt',
          }),
        }),
      );

      vi.unstubAllGlobals();
    });

    it('omits Authorization header when getClerkToken returns null', async () => {
      const { getClerkToken } = await import('@/services/apiClient');
      vi.mocked(getClerkToken).mockResolvedValueOnce(null);

      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        body: {
          getReader: () => ({
            read: vi.fn().mockResolvedValue({ done: true, value: undefined }),
          }),
        },
      });
      vi.stubGlobal('fetch', fetchMock);

      await connect();

      const callArgs = fetchMock.mock.calls[0];
      const headers = callArgs[1].headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();

      vi.unstubAllGlobals();
    });

    it('connects to the configured SSE URL', async () => {
      configureWebSocket({ url: 'http://localhost:8080/api/v1/events' });

      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        body: {
          getReader: () => ({
            read: vi.fn().mockResolvedValue({ done: true, value: undefined }),
          }),
        },
      });
      vi.stubGlobal('fetch', fetchMock);

      await connect();

      expect(fetchMock).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/events',
        expect.any(Object),
      );

      vi.unstubAllGlobals();
    });
  });
});
