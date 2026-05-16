import { test, expect } from '@playwright/test';

/**
 * Staging Smoke Suite (#1444)
 *
 * Targets the live staging BFF at staging-api.vaultmtg.app.
 * No browser UI is loaded — all assertions are made against raw HTTP responses
 * using Playwright's APIRequestContext (the `request` fixture).
 *
 * This suite is triggered by the INFRA-7 staging deploy workflow after a
 * successful deploy. A failure here must cause the workflow post-step to fail.
 *
 * Authentication approach:
 *   The staging BFF uses Clerk's Development instance (pk_test_* / sk_test_*
 *   per the staging ADR). Rather than a full browser sign-in flow, we use a
 *   pre-issued Clerk Development JWT supplied via the STAGING_SMOKE_TOKEN
 *   environment variable. The INFRA-7 workflow is responsible for providing
 *   this secret. This approach is explicitly permitted by the ticket:
 *   "Sign-in stub (Clerk Development instance sign-in flow or a test token)".
 *
 * Test cases:
 *   1. Sign-in stub        — pre-issued Clerk test token is accepted by the BFF
 *   2. Authenticated GET   — GET /api/v1/matches with token returns valid JSON
 *   3. SSE connect         — GET /api/v1/events with token connects without error
 *                            (receives 200 or at least does not 5xx/network-fail)
 *   4. Health check        — GET /healthz returns 200 with expected shape (unauthenticated)
 *   5. Auth guard          — auth-gated routes return 401 without a token
 *
 * The suite must complete in under 60 seconds total.
 *
 * Required environment variables:
 *   STAGING_API_URL        — override the staging BFF base URL (optional)
 *   STAGING_SMOKE_TOKEN    — Clerk Development JWT for authenticated calls (required
 *                            for the authenticated test cases; see below)
 */

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

/** Base URL for the staging BFF. Set STAGING_API_URL in the deploy workflow. */
const STAGING_API = process.env.STAGING_API_URL ?? 'https://staging-api.vaultmtg.app';

/**
 * Pre-issued Clerk Development JWT supplied by the INFRA-7 deploy workflow.
 * If absent, the authenticated test cases are skipped with a clear message so
 * that developers running the suite locally don't hit failures they cannot fix.
 */
const SMOKE_TOKEN = process.env.STAGING_SMOKE_TOKEN ?? '';

/** Authorization header value for authenticated requests. */
const authHeader = (): Record<string, string> =>
  SMOKE_TOKEN ? { Authorization: `Bearer ${SMOKE_TOKEN}` } : {};

// ---------------------------------------------------------------------------
// 1. Health check (unauthenticated)
// ---------------------------------------------------------------------------

test.describe('Staging smoke: health check', () => {
  test('GET /healthz returns 200', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/healthz`);
    expect(res.status()).toBe(200);
  });

  test('GET /healthz response body contains status field', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/healthz`);
    expect(res.status()).toBe(200);

    const body = await res.json() as Record<string, unknown>;

    // The BFF health endpoint must return at minimum a "status" field.
    expect(body).toHaveProperty('status');
    expect(typeof body.status).toBe('string');
    expect((body.status as string).length).toBeGreaterThan(0);
  });

  test('GET /healthz Content-Type is application/json', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/healthz`);
    const contentType = res.headers()['content-type'] ?? '';
    expect(contentType).toContain('application/json');
  });
});

// ---------------------------------------------------------------------------
// 2. Auth guard — unauthenticated requests must return 401
// ---------------------------------------------------------------------------

test.describe('Staging smoke: auth-gated routes return 401', () => {
  test('GET /api/v1/decks returns 401 without Authorization header', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/api/v1/decks`);
    expect(res.status()).toBe(401);
  });
});

// ---------------------------------------------------------------------------
// 3. Sign-in stub — Clerk test token is accepted by the BFF
//
// The INFRA-7 workflow supplies STAGING_SMOKE_TOKEN, a pre-issued JWT from
// Clerk's Development instance. This test verifies the token is accepted (i.e.
// the BFF's Clerk middleware correctly validates Development instance tokens).
// ---------------------------------------------------------------------------

test.describe('Staging smoke: sign-in stub (Clerk test token)', () => {
  test('Clerk test token is accepted — BFF returns non-401 on authenticated route', async ({ request }) => {
    if (!SMOKE_TOKEN) {
      test.skip(true, 'STAGING_SMOKE_TOKEN not set — skipping authenticated tests (required in CI)');
    }

    const res = await request.get(`${STAGING_API}/api/v1/matches`, {
      headers: authHeader(),
    });

    // Token must be accepted — any 2xx or 404 is fine (account may have no data).
    // 401 means the token was rejected — fail the smoke.
    expect(
      res.status(),
      `Clerk test token was rejected (got ${res.status()}). The BFF Clerk middleware may be misconfigured for the Development instance.`,
    ).not.toBe(401);

    // Staging must not 5xx either.
    expect(
      res.status(),
      `Authenticated GET /api/v1/matches returned ${res.status()} — staging BFF may be unhealthy`,
    ).toBeLessThan(500);
  });
});

// ---------------------------------------------------------------------------
// 4. Authenticated GET — one protected endpoint returns valid JSON
// ---------------------------------------------------------------------------

test.describe('Staging smoke: authenticated GET returns valid response', () => {
  test('GET /api/v1/matches with Clerk token returns JSON', async ({ request }) => {
    if (!SMOKE_TOKEN) {
      test.skip(true, 'STAGING_SMOKE_TOKEN not set — skipping authenticated tests (required in CI)');
    }

    const res = await request.get(`${STAGING_API}/api/v1/matches`, {
      headers: authHeader(),
    });

    // Must not 401 (token rejected) or 5xx (server error).
    expect(res.status()).not.toBe(401);
    expect(res.status()).toBeLessThan(500);

    // Response body must be valid JSON.
    const body = await res.json() as unknown;
    expect(body).not.toBeNull();

    // Accept either an array (has matches) or an object (envelope / empty data).
    // Both are valid shapes — the smoke just verifies the body is parseable JSON.
    expect(typeof body === 'object' || Array.isArray(body)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// 5. SSE endpoint — connection opens without a network-level error
// ---------------------------------------------------------------------------

test.describe('Staging smoke: SSE endpoint reachability', () => {
  /**
   * Playwright's APIRequestContext does not stream SSE, but a plain GET to the
   * SSE endpoint must not throw a network-level error.
   *
   * Acceptable outcomes with token:
   *   - 200 — endpoint accepted the connection
   *   - 404 — SSE path not yet live on staging (non-fatal warning)
   *
   * Acceptable outcomes without token (when SMOKE_TOKEN is absent):
   *   - 401 — auth guard fires before SSE upgrade (expected)
   *
   * Any 5xx or thrown network error indicates a broken staging deploy.
   */
  test('GET /api/v1/events does not return 5xx or network error', async ({ request }) => {
    let res: Awaited<ReturnType<typeof request.get>>;
    try {
      // Short timeout — the SSE server holds the connection open for
      // authenticated callers; we only verify the initial HTTP response.
      res = await request.get(`${STAGING_API}/api/v1/events`, {
        headers: authHeader(),
        timeout: 8_000,
      });
    } catch (err) {
      // A thrown error means a network failure — staging is unreachable.
      throw new Error(`SSE endpoint threw a network error: ${String(err)}`);
    }

    const status = res.status();

    // 5xx = staging BFF is unhealthy.
    expect(
      status,
      `SSE endpoint returned ${status} — staging BFF may be unhealthy`,
    ).toBeLessThan(500);

    if (SMOKE_TOKEN) {
      // With a token, the connection must be accepted (200) or route not found (404).
      // A 401 here means the token was rejected — fail loudly.
      expect(
        status,
        `SSE endpoint rejected Clerk test token (got ${status}). The BFF Clerk middleware may be misconfigured.`,
      ).not.toBe(401);
    }
  });
});
