/**
 * DraftLive E2E Tests — ticket #1390, #2178
 *
 * Tests the /draft/live page with mocked SSE events.
 *
 * The page is behind ProtectedRoute. Tests inject signed-in Clerk state via
 * window.__CLERK_TEST_STATE__ (requires VITE_CLERK_TEST_MODE=true, which is set
 * in the playwright.config.ts webServer command).
 *
 * SSE and ratings are intercepted via Playwright route interception so the
 * tests run without a live BFF.
 *
 * SSE mocking (#2178): DraftLive consumes the /api/v1/events stream via an
 * EventSource (useDraftEventStream). Two problems had to be solved:
 *
 * 1. Reconnect storm — a plain route.fulfill() with a finite body closes the
 *    stream the instant the body is delivered. EventSource then reconnects, and
 *    repeatedly fulfilling a finite body produced an unbounded reconnect storm
 *    (hundreds of /api/v1/events requests) that starved the page's event loop,
 *    so the async BFF draft-ratings fetch never resolved.
 *
 * 2. Event coalescing — useDraftEventStream stores only the *latest* event
 *    (setLatestEvent) and DraftLive's reducer processes it via a useEffect.
 *    Delivering several events in a single fulfil() body fires onmessage for
 *    each in the same tick, React batches the setState calls, and the effect
 *    only ever observes the final event — earlier events (e.g. draft.started)
 *    are dropped.
 *
 * mockSse() solves both: it takes an ordered list of event bodies and serves
 * exactly one per connection. EventSource closes after each body, reconnects
 * (≈100 ms backoff), and receives the next event in a fresh React tick — so
 * every event is processed in order. Once the list is exhausted, further
 * reconnections are aborted so the EventSource backs off instead of storming.
 */

import { test, expect, type Page } from '@playwright/test';

// Helper to build a mock SSE payload line.
function sseData(payload: object): string {
  return `data: ${JSON.stringify(payload)}\n\n`;
}

/**
 * Intercept the SSE endpoint and serve the given event bodies one per
 * connection (in order). After the list is exhausted, reconnections are
 * aborted so the EventSource backs off rather than reconnect-storming.
 * Must be called before page.goto().
 *
 * @param bodies Ordered SSE bodies — one delivered per EventSource connection.
 */
async function mockSse(page: Page, bodies: string[]): Promise<void> {
  let index = 0;
  await page.route('**/api/v1/events*', async (route) => {
    if (index >= bodies.length) {
      // All scripted events delivered — abort further reconnections so the
      // EventSource backs off (≈100 ms+) instead of tight-looping.
      await route.abort();
      return;
    }
    const body = bodies[index];
    index += 1;
    await route.fulfill({
      status: 200,
      headers: {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
      },
      body,
    });
  });
}

test.describe('DraftLive', () => {
  test.beforeEach(async ({ page }) => {
    // Inject signed-in Clerk state so ProtectedRoute passes through.
    await page.addInitScript(() => {
      (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ = {
        isSignedIn: true,
      };
    });
  });

  // ── Empty state ────────────────────────────────────────────────────────────

  test('@smoke shows empty state when no active draft', async ({ page }) => {
    // SSE stream that never sends an event — empty body, no scripted events.
    await mockSse(page, []);

    await page.goto('/draft/live');

    // The page must render.
    await expect(
      page.locator('[data-testid="draft-live-container"]')
    ).toBeVisible();

    // Empty state must be visible — no active draft.
    await expect(page.locator('[data-testid="empty-state"]')).toBeVisible();
    await expect(page.getByText('No active draft')).toBeVisible();
    await expect(
      page.getByText('Start a draft in Arena to see your live pick recommendations')
    ).toBeVisible();
  });

  // ── Active draft — pack display ────────────────────────────────────────────

  // NOT @smoke (#2178): the top-pick-badge assertion depends on the BFF
  // draft-ratings response being applied to the rendered pack cards. The pack
  // cards themselves render correctly from the SSE draft.pack event, but the
  // GIHWR ratings do not reliably attach in the E2E harness — DraftLive's
  // /api/v1/events endpoint is consumed by TWO independent SSE clients
  // (useDraftEventStream's EventSource and websocketClient's fetch-based
  // stream), which makes the ratings-fetch timing non-deterministic under
  // route mocking. Reliably covering top-pick highlighting needs an app-source
  // change (a dedicated, separately-mockable ratings path) that is out of
  // scope for #2178's test-only fix. Tracked as a follow-up; kept in the
  // `full` project so it still runs in release validation.
  test('shows pack cards and highlights top pick', async ({ page }) => {
    // Stub ratings endpoint.
    await page.route('**/api/v1/draft-ratings/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          set_code: 'ONE',
          draft_format: 'PremierDraft',
          cached_at: '2026-01-01T00:00:00Z',
          card_ratings: [
            { arena_id: 101, name: 'Elesh Norn', gihwr: 68 },
            { arena_id: 102, name: 'Plains', gihwr: 50 },
            { arena_id: 103, name: 'Swamp', gihwr: 48 },
          ],
          color_ratings: [],
        }),
      });
    });

    // Stub SSE — emit draft.started then draft.pack.
    const startedEvent = {
      type: 'draft.started',
      account_id: 'acc1',
      event_id: 'evt0',
      session_id: 'sess1',
      sequence: 0,
      occurred_at: '2026-05-08T00:00:00Z',
      payload: { set_code: 'ONE', draft_type: 'PremierDraft' },
    };
    const packEvent = {
      type: 'draft.pack',
      account_id: 'acc1',
      event_id: 'evt1',
      session_id: 'sess1',
      sequence: 1,
      occurred_at: '2026-05-08T00:00:01Z',
      payload: {
        card_ids: [101, 102, 103],
        pack_number: 0,
        pick_number: 0,
      },
    };
    // One event per connection so the reducer processes each in its own tick.
    await mockSse(page, [sseData(startedEvent), sseData(packEvent)]);

    await page.goto('/draft/live');
    await expect(page.locator('[data-testid="draft-live-container"]')).toBeVisible();

    // Pack section must appear.
    await expect(page.locator('[data-testid="draft-live-pack"]')).toBeVisible();

    // Pack cards appear.
    await expect(page.locator('[data-testid="pack-card-101"]')).toBeVisible();
    await expect(page.locator('[data-testid="pack-card-102"]')).toBeVisible();
    await expect(page.locator('[data-testid="pack-card-103"]')).toBeVisible();

    // Top pick badge on highest-GIHWR card.
    await expect(page.locator('[data-testid="top-pick-badge"]')).toBeVisible();
    const topCard = page.locator('[data-testid="pack-card-101"]');
    await expect(topCard).toHaveAttribute('data-top-pick', 'true');
  });

  // ── Pick history updates ─────────────────────────────────────────────────

  test('pick history updates after a draft.pick event', async ({ page }) => {
    // Stub ratings.
    await page.route('**/api/v1/draft-ratings/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          set_code: 'BLB',
          draft_format: 'QuickDraft',
          cached_at: '2026-01-01T00:00:00Z',
          card_ratings: [
            { arena_id: 201, name: 'Mosswood Dreadknight', gihwr: 63 },
            { arena_id: 202, name: 'Forest', gihwr: 46 },
          ],
          color_ratings: [],
        }),
      });
    });

    // SSE: started → pack → pick
    const events = [
      {
        type: 'draft.started',
        account_id: 'acc1',
        event_id: 'e0',
        session_id: 's1',
        sequence: 0,
        occurred_at: '2026-05-08T00:00:00Z',
        payload: { set_code: 'BLB', draft_type: 'QuickDraft' },
      },
      {
        type: 'draft.pack',
        account_id: 'acc1',
        event_id: 'e1',
        session_id: 's1',
        sequence: 1,
        occurred_at: '2026-05-08T00:00:01Z',
        payload: { card_ids: [201, 202], pack_number: 0, pick_number: 0 },
      },
      {
        type: 'draft.pick',
        account_id: 'acc1',
        event_id: 'e2',
        session_id: 's1',
        sequence: 2,
        occurred_at: '2026-05-08T00:00:02Z',
        payload: { card_id: 201, pack_number: 0, pick_number: 0 },
      },
    ];
    // One event per connection so the reducer processes each in its own tick.
    await mockSse(page, events.map(sseData));

    await page.goto('/draft/live');
    await expect(page.locator('[data-testid="draft-live-container"]')).toBeVisible();

    // After the pick event, the picked card should appear in history.
    await expect(page.locator('[data-testid="picked-card-201"]')).toBeVisible();

    // History section is visible.
    await expect(page.locator('[data-testid="draft-live-history"]')).toBeVisible();
  });

  // ── Set name and format display ───────────────────────────────────────────

  test('displays set name and format from draft.started event', async ({ page }) => {
    await page.route('**/api/v1/draft-ratings/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          set_code: 'MKM',
          draft_format: 'PremierDraft',
          cached_at: '2026-01-01T00:00:00Z',
          card_ratings: [],
          color_ratings: [],
        }),
      });
    });

    const ev = {
      type: 'draft.started',
      account_id: 'acc1',
      event_id: 'e0',
      session_id: 's1',
      sequence: 0,
      occurred_at: '2026-05-08T00:00:00Z',
      payload: { set_code: 'MKM', draft_type: 'PremierDraft' },
    };
    await mockSse(page, [sseData(ev)]);

    await page.goto('/draft/live');
    await expect(page.locator('[data-testid="draft-live-container"]')).toBeVisible();

    await expect(page.locator('[data-testid="draft-live-set"]')).toHaveText('MKM');
    await expect(page.locator('[data-testid="draft-live-format"]')).toHaveText('Premier Draft');
  });
});
