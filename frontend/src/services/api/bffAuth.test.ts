import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { createAPIKey, bffBaseUrl } from './bffAuth';
import type { CreateAPIKeyResponse } from './bffAuth';
import { ApiRequestError } from '../apiClient';

const BFF_BASE = 'http://localhost:8080';

const mockKeyResponse: CreateAPIKeyResponse = {
  key: 'abc123def456abc123def456abc123def456abc123def456abc123def456abcd',
  created_at: '2024-01-15T12:00:00Z',
};

// MSW server — intercepts fetch calls made by the adapter.
const server = setupServer(
  http.post(`${BFF_BASE}/api/keys`, async ({ request }) => {
    const authHeader = request.headers.get('Authorization');

    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      return HttpResponse.json({ error: 'unauthorized' }, { status: 401 });
    }

    const token = authHeader.replace('Bearer ', '');
    if (token === 'invalid-token') {
      return HttpResponse.json({ error: 'unauthorized' }, { status: 401 });
    }

    return HttpResponse.json(mockKeyResponse, { status: 201 });
  })
);

beforeEach(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  server.resetHandlers();
  server.close();
});

describe('createAPIKey', () => {
  it('calls POST /api/keys on the BFF base URL (not /api/v1)', async () => {
    let capturedUrl = '';
    server.use(
      http.post(`${BFF_BASE}/api/keys`, ({ request }) => {
        capturedUrl = request.url;
        return HttpResponse.json(mockKeyResponse, { status: 201 });
      })
    );

    await createAPIKey({ daemonToken: 'valid-daemon-jwt' });

    expect(capturedUrl).toBe(`${BFF_BASE}/api/keys`);
    // Confirm it is NOT under /api/v1/
    expect(capturedUrl).not.toContain('/api/v1/');
  });

  it('sends Authorization: Bearer header with the daemon token', async () => {
    let capturedAuth = '';
    server.use(
      http.post(`${BFF_BASE}/api/keys`, ({ request }) => {
        capturedAuth = request.headers.get('Authorization') ?? '';
        return HttpResponse.json(mockKeyResponse, { status: 201 });
      })
    );

    await createAPIKey({ daemonToken: 'my-daemon-jwt' });

    expect(capturedAuth).toBe('Bearer my-daemon-jwt');
  });

  it('returns the plaintext key and created_at on success', async () => {
    const result = await createAPIKey({ daemonToken: 'valid-daemon-jwt' });

    expect(result.key).toBe(mockKeyResponse.key);
    expect(result.created_at).toBe(mockKeyResponse.created_at);
  });

  it('throws ApiRequestError on 401 unauthorized', async () => {
    await expect(createAPIKey({ daemonToken: 'invalid-token' })).rejects.toThrow(
      ApiRequestError
    );
  });

  it('throws ApiRequestError with correct status on 401', async () => {
    try {
      await createAPIKey({ daemonToken: 'invalid-token' });
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiRequestError);
      expect((err as ApiRequestError).status).toBe(401);
    }
  });

  it('throws ApiRequestError on 500 server error', async () => {
    server.use(
      http.post(`${BFF_BASE}/api/keys`, () =>
        HttpResponse.json({ error: 'internal server error' }, { status: 500 })
      )
    );

    await expect(createAPIKey({ daemonToken: 'valid-token' })).rejects.toThrow(
      ApiRequestError
    );
  });

  it('sends Content-Type: application/json header', async () => {
    let capturedContentType = '';
    server.use(
      http.post(`${BFF_BASE}/api/keys`, ({ request }) => {
        capturedContentType = request.headers.get('Content-Type') ?? '';
        return HttpResponse.json(mockKeyResponse, { status: 201 });
      })
    );

    await createAPIKey({ daemonToken: 'valid-daemon-jwt' });

    expect(capturedContentType).toContain('application/json');
  });
});

describe('bffBaseUrl', () => {
  it('does not include /api/v1 suffix', () => {
    const base = bffBaseUrl();
    expect(base).not.toMatch(/\/api\/v1$/);
  });

  it('returns a valid URL string', () => {
    const base = bffBaseUrl();
    expect(() => new URL(base)).not.toThrow();
  });
});
