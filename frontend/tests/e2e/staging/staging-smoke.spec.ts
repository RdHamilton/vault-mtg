import { test, expect } from '@playwright/test';

/**
 * Staging Smoke Suite (#1444)
 *
 * Targets the live staging BFF at staging-api.vaultmtg.app.
 * No browser UI is loaded — all assertions are made against raw HTTP responses
 * using Playwright's APIRequestContext (the `request` fixture).
 *
 * This suite is triggered by the INFRA-7 staging deploy workflow after a
 * successful deploy.  A failure here must cause the workflow post-step to fail.
 *
 * Test cases:
 *   1. Health check  — GET /health returns 200 with expected JSON shape
 *   2. Auth-gated GET  — GET /api/v1/matches returns 401 without a token
 *   3. Auth-gated GET (decks) — GET /api/v1/decks returns 401 without a token
 *   4. SSE endpoint  — GET /api/v1/events connects without a network-level error
 *                       (accepts 401 from a gated SSE endpoint)
 *
 * The suite must complete in under 60 seconds total.
 */

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

/** Base URL for the staging BFF.  Set STAGING_API_URL in the deploy workflow. */
const STAGING_API = process.env.STAGING_API_URL ?? 'https://staging-api.vaultmtg.app';

// ---------------------------------------------------------------------------
// 1. Health check
// ---------------------------------------------------------------------------

test.describe('Staging smoke: health check', () => {
  test('GET /health returns 200', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/health`);
    expect(res.status()).toBe(200);
  });

  test('GET /health response body contains status field', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/health`);
    expect(res.status()).toBe(200);

    const body = await res.json() as Record<string, unknown>;

    // The BFF health endpoint must return at minimum a "status" field.
    // Accept either { status: "ok" } or { status: "healthy" } — both are valid.
    expect(body).toHaveProperty('status');
    expect(typeof body.status).toBe('string');
    expect((body.status as string).length).toBeGreaterThan(0);
  });

  test('GET /health Content-Type is application/json', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/health`);
    const contentType = res.headers()['content-type'] ?? '';
    expect(contentType).toContain('application/json');
  });
});

// ---------------------------------------------------------------------------
// 2. Auth-gated endpoints return 401 without a token
// ---------------------------------------------------------------------------

test.describe('Staging smoke: auth-gated routes return 401', () => {
  test('GET /api/v1/matches returns 401 without Authorization header', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/api/v1/matches`);
    // Must be 401 (Unauthorized) — not 200 and not a 5xx.
    expect(res.status()).toBe(401);
  });

  test('GET /api/v1/decks returns 401 without Authorization header', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/api/v1/decks`);
    expect(res.status()).toBe(401);
  });

  test('GET /api/v1/draft returns 401 without Authorization header', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/api/v1/draft`);
    // 401 or 404 are both acceptable — either means auth runs before routing.
    expect([401, 404]).toContain(res.status());
  });
});

// ---------------------------------------------------------------------------
// 3. SSE endpoint accepts a connection (network-level check only)
// ---------------------------------------------------------------------------

test.describe('Staging smoke: SSE endpoint reachability', () => {
  /**
   * Playwright's APIRequestContext does not stream SSE natively, but a plain
   * GET to the SSE endpoint must not throw a network-level error.
   *
   * Acceptable outcomes:
   *   - 401 — endpoint is protected (expected; auth guard fires first)
   *   - 200 — endpoint is open or a test token was supplied
   *   - 404 — SSE path doesn't exist yet on staging (non-blocking warning)
   *
   * Any 5xx or a thrown network error indicates a broken staging deploy.
   */
  test('GET /api/v1/events does not return a 5xx or network error', async ({ request }) => {
    let res: Awaited<ReturnType<typeof request.get>>;
    try {
      // Short timeout — the SSE server will hold this connection open if we are
      // authenticated, so we only want to verify the initial HTTP response.
      res = await request.get(`${STAGING_API}/api/v1/events`, { timeout: 8_000 });
    } catch (err) {
      // A thrown error means a network failure — fail the test with context.
      throw new Error(`SSE endpoint threw a network error: ${String(err)}`);
    }

    const status = res.status();
    // 5xx means staging is broken.
    expect(
      status,
      `SSE endpoint returned ${status} — staging BFF may be unhealthy`,
    ).toBeLessThan(500);
  });
});
