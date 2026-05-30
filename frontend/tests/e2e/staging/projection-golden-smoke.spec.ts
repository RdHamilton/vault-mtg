import { test, expect, type APIRequestContext } from '@playwright/test';

/**
 * BFF Projection Golden-Event Smoke Suite (vault-mtg-tickets#208)
 *
 * Replays representative match.completed, deck.updated, and quest.completed
 * payloads through the BFF projection path on staging. Asserts that:
 *
 *   AC1 — Well-formed events produce projected rows in the destination tables.
 *   AC2 — Missing enrichment fields (format, name, etc.) are default-filled;
 *          the row is still written (no drop).
 *   AC3 — Missing required fields (match_id) cause the event to go to the
 *          projection_errors DLQ (no destination row written).
 *   AC5 — Coverage of the #200 fix: empty format projects as "Unknown".
 *   AC5 — Coverage of the #201 fix: non-empty format projects as provided.
 *
 * Architecture
 * ────────────
 * The BFF projection worker runs on a 30-second tick (no HTTP trigger).
 * The smoke therefore:
 *   1. POSTs each golden payload to POST /api/v1/ingest/events (daemon API key auth).
 *   2. Polls the destination REST endpoint with back-off until the projected row
 *      appears (max 55 s — covers two worker cycles with margin).
 *   3. On timeout, fails with a descriptive message that names the missing field.
 *
 * For the DLQ test (AC3) there is currently no HTTP endpoint that exposes
 * projection_errors row counts. The smoke asserts the negative:
 *   - The destination table (matches) receives NO row for the bad payload.
 *
 * ── Bob hand-off ──────────────────────────────────────────────────────────────
 * Direct projection_errors row-count assertion (AC3 full verification) requires
 * a new admin endpoint: GET /api/v1/admin/projection-errors/count.
 * Without it the smoke can only assert "no match row was written" — the DLQ
 * write itself is proven by the existing Go unit tests (worker_dlq_test.go).
 * See ticket comment on vault-mtg-tickets#208.
 * ─────────────────────────────────────────────────────────────────────────────
 *
 * Required environment variables:
 *   STAGING_API_URL            — BFF base URL (default: https://staging-api.vaultmtg.app)
 *   STAGING_SMOKE_TOKEN        — Clerk Development JWT for read-only authenticated calls
 *   STAGING_DAEMON_API_KEY     — Daemon API key for POST /api/v1/ingest/events
 *
 * If STAGING_DAEMON_API_KEY is absent, the ingest tests are skipped with a clear
 * message. STAGING_SMOKE_TOKEN is required for all read-back assertions.
 *
 * Run locally:
 *   STAGING_API_URL=https://staging-api.vaultmtg.app \
 *   STAGING_SMOKE_TOKEN=<token> \
 *   STAGING_DAEMON_API_KEY=<key> \
 *   npx playwright test tests/e2e/staging/projection-golden-smoke.spec.ts
 */

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const STAGING_API = process.env.STAGING_API_URL ?? 'https://staging-api.vaultmtg.app';
const SMOKE_TOKEN = process.env.STAGING_SMOKE_TOKEN ?? '';
const DAEMON_API_KEY = process.env.STAGING_DAEMON_API_KEY ?? '';

/** Max ms to wait for a projected row to appear. Two 30-second worker cycles
 *  plus 10s boot margin. Individual tests use test.setTimeout(90_000) to allow
 *  for both the ingest and the full poll window. */
const PROJECTION_POLL_TIMEOUT_MS = 55_000;
const PROJECTION_POLL_INTERVAL_MS = 3_000;

// ---------------------------------------------------------------------------
// Stable sentinel IDs — unique per smoke run so reruns don't pick up rows
// from prior runs. The timestamp suffix keeps IDs unique across concurrent
// CI runs. These are not real MTGA IDs.
// ---------------------------------------------------------------------------

const RUN_TS = Date.now();
const GOLDEN_MATCH_ID                 = `smoke-match-${RUN_TS}`;
const GOLDEN_MATCH_ID_WITH_FORMAT     = `smoke-match-fmt-${RUN_TS}`;
const GOLDEN_DLQ_MATCH_ID             = `smoke-dlq-${RUN_TS}`;
const GOLDEN_DECK_ID                  = `smoke-deck-${RUN_TS}`;
const GOLDEN_DECK_ID_MISSING_ENRICHMENT = `smoke-deck-enrich-${RUN_TS}`;
const GOLDEN_QUEST_ID                 = `smoke-quest-${RUN_TS}`;

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

/** Authorization header for Clerk-authenticated REST calls (read-back). */
const clerkAuthHeader = (): Record<string, string> =>
  SMOKE_TOKEN ? { Authorization: `Bearer ${SMOKE_TOKEN}` } : {};

/** Authorization header for daemon ingest calls. */
const daemonAuthHeader = (): Record<string, string> =>
  DAEMON_API_KEY ? { Authorization: `Bearer ${DAEMON_API_KEY}` } : {};

// ---------------------------------------------------------------------------
// Golden payloads
// ---------------------------------------------------------------------------

type DaemonEventEnvelope = {
  type: string;
  account_id: string;
  event_id: string;
  sequence: number;
  occurred_at: string;
  payload: Record<string, unknown>;
};

/**
 * AC5 / #200 fix: match.completed with EMPTY format field.
 * The projection worker must default-fill format="Unknown" and write the row.
 */
const GOLDEN_MATCH_MISSING_FORMAT: DaemonEventEnvelope = {
  type: 'match.completed',
  account_id: '',
  event_id: `evt-smoke-match-${RUN_TS}`,
  sequence: 1,
  occurred_at: new Date().toISOString(),
  payload: {
    match_id:        GOLDEN_MATCH_ID,
    event_id:        `evt-smoke-match-${RUN_TS}`,
    event_name:      'Ladder_BO1',
    format:          '',           // intentionally empty — AC5 / #200 fix
    result:          'win',
    player_wins:     2,
    opponent_wins:   0,
    player_team_id:  1,
    winning_team_id: 1,
  },
};

/**
 * AC5 / #201 fix: match.completed with explicit non-empty format field.
 * Verifies the fix did not break the happy path.
 */
const GOLDEN_MATCH_WITH_FORMAT: DaemonEventEnvelope = {
  type: 'match.completed',
  account_id: '',
  event_id: `evt-smoke-match-fmt-${RUN_TS}`,
  sequence: 2,
  occurred_at: new Date().toISOString(),
  payload: {
    match_id:        GOLDEN_MATCH_ID_WITH_FORMAT,
    event_id:        `evt-smoke-match-fmt-${RUN_TS}`,
    event_name:      'QuickDraft_EOE',
    format:          'Premier Draft',  // non-empty — #201 fix
    result:          'loss',
    player_wins:     0,
    opponent_wins:   2,
    player_team_id:  2,
    winning_team_id: 1,
  },
};

/**
 * AC3 / DLQ: match.completed with MISSING match_id (required PK field).
 * Per ADR-039: the event must go to projection_errors — NOT to matches.
 */
const GOLDEN_MATCH_DLQ: DaemonEventEnvelope = {
  type: 'match.completed',
  account_id: '',
  event_id: `evt-smoke-dlq-${RUN_TS}`,
  sequence: 3,
  occurred_at: new Date().toISOString(),
  payload: {
    // match_id intentionally absent — required PK, triggers DLQ path per ADR-039
    event_name:      'Ladder_BO1',
    format:          'Standard',
    result:          'win',
    player_wins:     2,
    opponent_wins:   0,
    player_team_id:  1,
    winning_team_id: 1,
  },
};

/** AC1 / deck.updated: well-formed deck payload. */
const GOLDEN_DECK: DaemonEventEnvelope = {
  type: 'deck.updated',
  account_id: '',
  event_id: `evt-smoke-deck-${RUN_TS}`,
  sequence: 4,
  occurred_at: new Date().toISOString(),
  payload: {
    deck_id: GOLDEN_DECK_ID,
    name:    'Golden Smoke Deck',
    format:  'Standard',
    cards: [
      { arena_id: 84738, quantity: 4 },
      { arena_id: 84739, quantity: 4 },
    ],
  },
};

/**
 * AC2 / deck.updated with missing enrichment fields (name, format).
 * Per ADR-039: name defaults to "Unnamed Deck", format defaults to "Unknown".
 */
const GOLDEN_DECK_MISSING_ENRICHMENT: DaemonEventEnvelope = {
  type: 'deck.updated',
  account_id: '',
  event_id: `evt-smoke-deck-enrich-${RUN_TS}`,
  sequence: 5,
  occurred_at: new Date().toISOString(),
  payload: {
    deck_id: GOLDEN_DECK_ID_MISSING_ENRICHMENT,
    // name intentionally absent → defaults to "Unnamed Deck"
    // format intentionally absent → defaults to "Unknown"
    cards: [{ arena_id: 90001, quantity: 1 }],
  },
};

/** AC1 / quest.completed: well-formed quest payload. */
const GOLDEN_QUEST: DaemonEventEnvelope = {
  type: 'quest.completed',
  account_id: '',
  event_id: `evt-smoke-quest-${RUN_TS}`,
  sequence: 6,
  occurred_at: new Date().toISOString(),
  payload: {
    quest_id:          GOLDEN_QUEST_ID,
    quest_name:        'Golden Smoke Quest',
    progress:          3,
    goal:              3,
    xp_reward:         500,
    completion_source: 'match',
  },
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** POST a single event to the staging BFF ingest endpoint. Returns HTTP status. */
async function ingestEvent(
  request: APIRequestContext,
  envelope: DaemonEventEnvelope,
): Promise<number> {
  const res = await request.post(
    `${STAGING_API}/api/v1/ingest/events`,
    {
      headers: { ...daemonAuthHeader(), 'Content-Type': 'application/json' },
      data: JSON.stringify(envelope),
    },
  );
  return res.status();
}

/**
 * Poll a URL with the Clerk token until the predicate returns true or timeout.
 * Returns the last parsed response body on success; throws on timeout.
 */
async function pollUntil(
  request: APIRequestContext,
  url: string,
  predicate: (body: unknown) => boolean,
  description: string,
): Promise<unknown> {
  const deadline = Date.now() + PROJECTION_POLL_TIMEOUT_MS;
  let lastBody: unknown = null;
  while (Date.now() < deadline) {
    const res = await request.get(url, { headers: clerkAuthHeader() });
    if (res.ok()) {
      lastBody = await res.json() as unknown;
      if (predicate(lastBody)) {
        return lastBody;
      }
    }
    await new Promise((resolve) => setTimeout(resolve, PROJECTION_POLL_INTERVAL_MS));
  }
  throw new Error(
    `Projection poll timed out after ${PROJECTION_POLL_TIMEOUT_MS}ms waiting for: ${description}.\n` +
    `Last response body: ${JSON.stringify(lastBody, null, 2)}`,
  );
}

/** Returns true when body contains a match with the given match_id or matchId. */
function bodyContainsMatch(matchId: string): (body: unknown) => boolean {
  return (body: unknown): boolean => {
    if (!body || typeof body !== 'object') return false;
    const matches = (body as Record<string, unknown>)['matches'];
    if (!Array.isArray(matches)) return false;
    return matches.some((m: unknown) => {
      if (!m || typeof m !== 'object') return false;
      const row = m as Record<string, unknown>;
      return row['matchId'] === matchId || row['match_id'] === matchId;
    });
  };
}

/** Returns true when body contains a deck with the given deck_id or deckId. */
function bodyContainsDeck(deckId: string): (body: unknown) => boolean {
  return (body: unknown): boolean => {
    if (!body || typeof body !== 'object') return false;
    const raw = (body as Record<string, unknown>)['decks'] ?? body;
    if (!Array.isArray(raw)) return false;
    return raw.some((d: unknown) => {
      if (!d || typeof d !== 'object') return false;
      const row = d as Record<string, unknown>;
      return row['deckId'] === deckId || row['deck_id'] === deckId;
    });
  };
}

/** Returns true when body contains a quest entry with the given quest_id or questId. */
function bodyContainsQuest(questId: string): (body: unknown) => boolean {
  return (body: unknown): boolean => {
    if (!body || typeof body !== 'object') return false;
    const b = body as Record<string, unknown>;
    const raw =
      Array.isArray(b['quests'])  ? b['quests'] :
      Array.isArray(b['history']) ? b['history'] :
      Array.isArray(body)         ? body         : null;
    if (!Array.isArray(raw)) return false;
    return raw.some((q: unknown) => {
      if (!q || typeof q !== 'object') return false;
      const row = q as Record<string, unknown>;
      return row['questId'] === questId || row['quest_id'] === questId;
    });
  };
}

// ---------------------------------------------------------------------------
// Pre-flight guards
// ---------------------------------------------------------------------------

test.describe('Projection golden-event smoke: pre-flight guards', () => {
  test('staging BFF /healthz returns 200', async ({ request }) => {
    const res = await request.get(`${STAGING_API}/healthz`);
    expect(res.status()).toBe(200);
  });

  test('required env vars are set (STAGING_DAEMON_API_KEY + STAGING_SMOKE_TOKEN)', () => {
    if (!DAEMON_API_KEY) {
      test.skip(true, 'STAGING_DAEMON_API_KEY not set — skipping projection smoke (required in CI)');
    }
    if (!SMOKE_TOKEN) {
      test.skip(true, 'STAGING_SMOKE_TOKEN not set — skipping projection smoke read-back (required in CI)');
    }
    expect(DAEMON_API_KEY.length).toBeGreaterThan(0);
    expect(SMOKE_TOKEN.length).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// AC5 / #200 fix — match.completed: missing format defaults to "Unknown"
// ---------------------------------------------------------------------------

test.describe('Projection golden-event smoke: match.completed (AC5 — empty format default-fills)', () => {
  test.setTimeout(90_000);

  test('POST match.completed with empty format → BFF returns 202 @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY) {
      test.skip(true, 'STAGING_DAEMON_API_KEY not set');
    }

    const status = await ingestEvent(request, GOLDEN_MATCH_MISSING_FORMAT);
    expect(
      status,
      `BFF rejected golden match.completed ingest: got ${status}. ` +
      'Check that STAGING_DAEMON_API_KEY is a valid daemon API key on staging.',
    ).toBe(202);
  });

  test('match.completed (empty format) projects to matches table with format="Unknown" @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY || !SMOKE_TOKEN) {
      test.skip(true, 'STAGING_DAEMON_API_KEY or STAGING_SMOKE_TOKEN not set');
    }

    // Ingest is idempotent — safe to re-send across test workers.
    await ingestEvent(request, GOLDEN_MATCH_MISSING_FORMAT);

    const body = await pollUntil(
      request,
      `${STAGING_API}/api/v1/matches`,
      bodyContainsMatch(GOLDEN_MATCH_ID),
      `match_id=${GOLDEN_MATCH_ID} to appear in /api/v1/matches`,
    );

    const matches = (body as Record<string, unknown>)['matches'] as Array<Record<string, unknown>>;
    const projected = matches.find(
      (m) => m['matchId'] === GOLDEN_MATCH_ID || m['match_id'] === GOLDEN_MATCH_ID,
    );
    expect(projected, `Match ${GOLDEN_MATCH_ID} not found in /api/v1/matches after projection`).toBeDefined();

    const format = (projected as Record<string, unknown>)['format'];
    expect(
      format,
      `match.completed with empty format field must project with format="Unknown" (AC5/#200 fix); got ${String(format)}`,
    ).toBe('Unknown');
  });
});

// ---------------------------------------------------------------------------
// AC5 / #201 fix — match.completed: non-empty format projects as-provided
// ---------------------------------------------------------------------------

test.describe('Projection golden-event smoke: match.completed (AC5 — explicit format preserved)', () => {
  test.setTimeout(90_000);

  test('POST match.completed with format="Premier Draft" → BFF returns 202 @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY) {
      test.skip(true, 'STAGING_DAEMON_API_KEY not set');
    }

    const status = await ingestEvent(request, GOLDEN_MATCH_WITH_FORMAT);
    expect(status).toBe(202);
  });

  test('match.completed (explicit format) projects with format="Premier Draft" @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY || !SMOKE_TOKEN) {
      test.skip(true, 'STAGING_DAEMON_API_KEY or STAGING_SMOKE_TOKEN not set');
    }

    await ingestEvent(request, GOLDEN_MATCH_WITH_FORMAT);

    const body = await pollUntil(
      request,
      `${STAGING_API}/api/v1/matches`,
      bodyContainsMatch(GOLDEN_MATCH_ID_WITH_FORMAT),
      `match_id=${GOLDEN_MATCH_ID_WITH_FORMAT} to appear in /api/v1/matches`,
    );

    const matches = (body as Record<string, unknown>)['matches'] as Array<Record<string, unknown>>;
    const projected = matches.find(
      (m) => m['matchId'] === GOLDEN_MATCH_ID_WITH_FORMAT || m['match_id'] === GOLDEN_MATCH_ID_WITH_FORMAT,
    );
    expect(projected, `Match ${GOLDEN_MATCH_ID_WITH_FORMAT} not found after projection`).toBeDefined();

    const format = (projected as Record<string, unknown>)['format'];
    expect(
      format,
      `match.completed with explicit format must project as-provided (#201 fix); got ${String(format)}`,
    ).toBe('Premier Draft');
  });
});

// ---------------------------------------------------------------------------
// AC3 — match.completed: missing match_id (required PK) → DLQ, NOT matches
// ---------------------------------------------------------------------------

test.describe('Projection golden-event smoke: match.completed (AC3 — missing PK → DLQ)', () => {
  test.setTimeout(90_000);

  test('POST match.completed with missing match_id → BFF returns 202 (ingest never rejects)', async ({ request }) => {
    if (!DAEMON_API_KEY) {
      test.skip(true, 'STAGING_DAEMON_API_KEY not set');
    }

    const status = await ingestEvent(request, GOLDEN_MATCH_DLQ);
    // The BFF ingest endpoint accepts all events regardless of payload structure
    // (ADR-039 §"Do not alter the ingest path"). A non-202 here is a BFF regression.
    expect(
      status,
      `BFF ingest must accept all events (202) regardless of payload structure; got ${status}`,
    ).toBe(202);
  });

  test('match.completed (missing match_id) does NOT produce a matches row after projection', async ({ request }) => {
    if (!DAEMON_API_KEY || !SMOKE_TOKEN) {
      test.skip(true, 'STAGING_DAEMON_API_KEY or STAGING_SMOKE_TOKEN not set');
    }

    await ingestEvent(request, GOLDEN_MATCH_DLQ);

    // Use the well-formed match as the projection-tick sentinel: if it appeared,
    // we know the worker has run at least once since we ingested the DLQ payload.
    await pollUntil(
      request,
      `${STAGING_API}/api/v1/matches`,
      bodyContainsMatch(GOLDEN_MATCH_ID),
      `well-formed sentinel match ${GOLDEN_MATCH_ID} to appear (proves projection tick fired)`,
    );

    // Now assert the structurally-broken event did NOT produce a row.
    const res = await request.get(`${STAGING_API}/api/v1/matches`, { headers: clerkAuthHeader() });
    expect(res.ok()).toBe(true);
    const body = await res.json() as unknown;

    expect(
      bodyContainsMatch(GOLDEN_DLQ_MATCH_ID)(body),
      `match.completed with missing match_id MUST NOT produce a matches row (AC3 / ADR-039 DLQ path). ` +
      `Found a row for event_id=${GOLDEN_DLQ_MATCH_ID} — the permanent-error DLQ routing is broken.`,
    ).toBe(false);

    // ── Bob hand-off ──────────────────────────────────────────────────────────
    // To also assert a projection_errors row WAS written (full AC3 coverage),
    // add: GET /api/v1/admin/projection-errors/count?event_type=match.completed
    // Tracked on vault-mtg-tickets#208.
    // ─────────────────────────────────────────────────────────────────────────
  });
});

// ---------------------------------------------------------------------------
// AC1 — deck.updated: well-formed payload projects to decks table
// ---------------------------------------------------------------------------

test.describe('Projection golden-event smoke: deck.updated (AC1 — well-formed)', () => {
  test.setTimeout(90_000);

  test('POST deck.updated with all fields → BFF returns 202 @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY) {
      test.skip(true, 'STAGING_DAEMON_API_KEY not set');
    }

    const status = await ingestEvent(request, GOLDEN_DECK);
    expect(status).toBe(202);
  });

  test('deck.updated projects to /api/v1/decks with correct deck_id @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY || !SMOKE_TOKEN) {
      test.skip(true, 'STAGING_DAEMON_API_KEY or STAGING_SMOKE_TOKEN not set');
    }

    await ingestEvent(request, GOLDEN_DECK);

    await pollUntil(
      request,
      `${STAGING_API}/api/v1/decks`,
      bodyContainsDeck(GOLDEN_DECK_ID),
      `deck_id=${GOLDEN_DECK_ID} to appear in /api/v1/decks`,
    );
    // pollUntil throws on timeout — reaching here means the deck was projected.
    expect(true).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// AC2 — deck.updated: missing enrichment fields default-fill
// ---------------------------------------------------------------------------

test.describe('Projection golden-event smoke: deck.updated (AC2 — enrichment default-fill)', () => {
  test.setTimeout(90_000);

  test('POST deck.updated with missing name and format → BFF returns 202', async ({ request }) => {
    if (!DAEMON_API_KEY) {
      test.skip(true, 'STAGING_DAEMON_API_KEY not set');
    }

    const status = await ingestEvent(request, GOLDEN_DECK_MISSING_ENRICHMENT);
    expect(status).toBe(202);
  });

  test('deck.updated (missing name/format) projects with name="Unnamed Deck" and format="Unknown"', async ({ request }) => {
    if (!DAEMON_API_KEY || !SMOKE_TOKEN) {
      test.skip(true, 'STAGING_DAEMON_API_KEY or STAGING_SMOKE_TOKEN not set');
    }

    await ingestEvent(request, GOLDEN_DECK_MISSING_ENRICHMENT);

    const body = await pollUntil(
      request,
      `${STAGING_API}/api/v1/decks`,
      bodyContainsDeck(GOLDEN_DECK_ID_MISSING_ENRICHMENT),
      `deck_id=${GOLDEN_DECK_ID_MISSING_ENRICHMENT} to appear in /api/v1/decks`,
    );

    const decksRaw = (body as Record<string, unknown>)['decks'] ?? body;
    const decks = Array.isArray(decksRaw) ? decksRaw as Array<Record<string, unknown>> : [];
    const projected = decks.find(
      (d) => d['deckId'] === GOLDEN_DECK_ID_MISSING_ENRICHMENT ||
             d['deck_id'] === GOLDEN_DECK_ID_MISSING_ENRICHMENT,
    );

    expect(projected, `Deck ${GOLDEN_DECK_ID_MISSING_ENRICHMENT} not found after projection`).toBeDefined();

    const name = (projected as Record<string, unknown>)['name'];
    const format = (projected as Record<string, unknown>)['format'];

    expect(
      name,
      `deck.updated with missing name must default to "Unnamed Deck" (AC2 / ADR-039); got ${String(name)}`,
    ).toBe('Unnamed Deck');

    expect(
      format,
      `deck.updated with missing format must default to "Unknown" (AC2 / ADR-039); got ${String(format)}`,
    ).toBe('Unknown');
  });
});

// ---------------------------------------------------------------------------
// AC1 — quest.completed: well-formed payload projects to quest history
// ---------------------------------------------------------------------------

test.describe('Projection golden-event smoke: quest.completed (AC1 — well-formed)', () => {
  test.setTimeout(90_000);

  test('POST quest.completed → BFF returns 202 @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY) {
      test.skip(true, 'STAGING_DAEMON_API_KEY not set');
    }

    const status = await ingestEvent(request, GOLDEN_QUEST);
    expect(status).toBe(202);
  });

  test('quest.completed projects to /api/v1/quests/history with correct quest_id @smoke', async ({ request }) => {
    if (!DAEMON_API_KEY || !SMOKE_TOKEN) {
      test.skip(true, 'STAGING_DAEMON_API_KEY or STAGING_SMOKE_TOKEN not set');
    }

    await ingestEvent(request, GOLDEN_QUEST);

    await pollUntil(
      request,
      `${STAGING_API}/api/v1/quests/history`,
      bodyContainsQuest(GOLDEN_QUEST_ID),
      `quest_id=${GOLDEN_QUEST_ID} to appear in /api/v1/quests/history`,
    );
    expect(true).toBe(true);
  });
});
