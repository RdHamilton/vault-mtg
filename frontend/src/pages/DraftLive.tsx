/**
 * DraftLive — /draft/live
 *
 * Real-time draft assistant page. Consumes the SSE event stream via
 * useDraftEventStream, feeds events into useDraftSession state machine,
 * and fetches card ratings from the BFF for top-pick highlighting.
 *
 * Ticket: #1390
 */

import { useEffect, useMemo, useRef, useState } from 'react';
import { useAuth } from '@clerk/react';
import { useDraftEventStream, useDraftSession } from '@/hooks';
import type { DraftPackPayload } from '@/hooks';
import { getDraftRatings } from '@/services/api/bffDraftRatings';
import type { BffCardRating } from '@/services/api/bffDraftRatings';
import { trackEvent } from '@/services/analytics';
import LoadingSpinner from '../components/LoadingSpinner';
import EmptyState from '../components/EmptyState';
import './DraftLive.css';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Map draft_type / CourseName strings to human-readable format labels.
 * Handles both the flat `draft_type` field and the nested `CourseName` field
 * from the daemon pack payload.
 */
function formatLabel(raw: string | undefined): string {
  if (!raw) return 'Draft';
  const upper = raw.toUpperCase();
  if (upper.includes('PREMIER') || upper.includes('TRADITIONAL')) return 'Premier Draft';
  if (upper.includes('QUICK')) return 'Quick Draft';
  if (upper.includes('SEALED')) return 'Sealed';
  return raw;
}

/** Best-effort set code extractor from a CourseName like "Quilinor_QuickDraft". */
function setCodeFromCourseName(courseName: string | undefined): string | null {
  if (!courseName) return null;
  // CourseName format: "<SetCode>_<DraftType>", e.g. "ONE_PremierDraft"
  const parts = courseName.split('_');
  if (parts.length >= 2) return parts[0].toUpperCase();
  return null;
}

/** Grade letter for a card's GIHWR (Game-In-Hand Win Rate). */
function gradeFromGihwr(gihwr: number | undefined): string {
  if (gihwr === undefined || gihwr === 0) return '—';
  if (gihwr >= 65) return 'A+';
  if (gihwr >= 62) return 'A';
  if (gihwr >= 59) return 'A-';
  if (gihwr >= 57) return 'B+';
  if (gihwr >= 55) return 'B';
  if (gihwr >= 53) return 'B-';
  if (gihwr >= 51) return 'C+';
  if (gihwr >= 49) return 'C';
  if (gihwr >= 47) return 'C-';
  if (gihwr >= 45) return 'D';
  return 'F';
}

function gradeClass(grade: string): string {
  const letter = grade.charAt(0);
  switch (letter) {
    case 'A': return 'grade-a';
    case 'B': return 'grade-b';
    case 'C': return 'grade-c';
    case 'D': return 'grade-d';
    case 'F': return 'grade-f';
    default:  return 'grade-unknown';
  }
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface RatingsState {
  ratings: BffCardRating[];
  loading: boolean;
  error: string | null;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const DraftLive: React.FC = () => {
  const { getToken } = useAuth();
  const getTokenRef = useRef(getToken);
  useEffect(() => { getTokenRef.current = getToken; });

  // ── SSE stream ────────────────────────────────────────────────────────────
  const { latestEvent, status: streamStatus } = useDraftEventStream();

  // ── Session state machine ─────────────────────────────────────────────────
  const { state: session, dispatch } = useDraftSession();

  // ── Draft metadata derived from events ───────────────────────────────────
  const [setCode, setSetCode] = useState<string | null>(null);
  const [draftFormat, setDraftFormat] = useState<string | null>(null);

  // Feed latest SSE event into state machine + derive metadata.
  useEffect(() => {
    if (!latestEvent) return;

    dispatch({ type: latestEvent.type, payload: latestEvent.payload ?? undefined });

    // Extract set code and format from draft.started payload.
    if (latestEvent.type === 'draft.started') {
      const p = latestEvent.payload as { set_code?: string; draft_type?: string } | null;
      if (p?.set_code) setSetCode(p.set_code.toUpperCase());
      if (p?.draft_type) setDraftFormat(formatLabel(p.draft_type));
    }

    // Fallback: also try to extract set code from pack payload CourseName.
    if (latestEvent.type === 'draft.pack' && !setCode) {
      const p = latestEvent.payload as DraftPackPayload | null;
      const extracted = setCodeFromCourseName(p?.CourseName);
      if (extracted) setSetCode(extracted);
      if (p?.CourseName && !draftFormat) setDraftFormat(formatLabel(p.CourseName));
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- dispatch is stable; setCode/draftFormat only needed as guards
  }, [latestEvent, dispatch]);

  // ── BFF ratings fetch ─────────────────────────────────────────────────────
  const [ratingsState, setRatingsState] = useState<RatingsState>({
    ratings: [],
    loading: false,
    error: null,
  });

  const lastFetchedRef = useRef<string | null>(null);

  useEffect(() => {
    if (!setCode || !draftFormat) return;
    const key = `${setCode}/${draftFormat}`;
    if (lastFetchedRef.current === key) return;
    lastFetchedRef.current = key;

    const fetchRatings = async () => {
      setRatingsState((prev) => ({ ...prev, loading: true, error: null }));
      try {
        const result = await getDraftRatings(setCode, draftFormat);
        setRatingsState({
          ratings: result.data.card_ratings ?? [],
          loading: false,
          error: null,
        });
      } catch (err) {
        setRatingsState({
          ratings: [],
          loading: false,
          error: err instanceof Error ? err.message : 'Failed to load ratings',
        });
      }
    };

    void fetchRatings();
  }, [setCode, draftFormat]);

  // ── Derived data: pack cards with ratings ─────────────────────────────────
  const ratingsMap = useMemo(() => {
    const map = new Map<number, BffCardRating>();
    for (const r of ratingsState.ratings) {
      map.set(r.arena_id, r);
    }
    return map;
  }, [ratingsState.ratings]);

  interface PackCard {
    arenaId: number;
    rating: BffCardRating | undefined;
    grade: string;
    gihwr: number | undefined;
  }

  const packCards: PackCard[] = useMemo(() => {
    return session.currentPackCards.map((id) => {
      const rating = ratingsMap.get(id);
      const grade = gradeFromGihwr(rating?.gihwr);
      return { arenaId: id, rating, grade, gihwr: rating?.gihwr };
    });
  }, [session.currentPackCards, ratingsMap]);

  // Analytics: feature_draft_advisor_pick_viewed — fires once per pack when cards are non-empty
  const lastPickKeyRef = useRef<string | null>(null);
  useEffect(() => {
    if (packCards.length === 0 || !setCode) return;
    const key = `${setCode}/${session.packNumber}/${session.pickNumber}`;
    if (lastPickKeyRef.current === key) return;
    lastPickKeyRef.current = key;
    trackEvent({
      name: 'feature_draft_advisor_pick_viewed',
      properties: {
        set_code: setCode,
        pack_number: session.packNumber,
        pick_number: session.pickNumber,
      },
    });
  }, [packCards.length, setCode, session.packNumber, session.pickNumber]);

  // Top pick = highest GIHWR. Undefined when all are unrated.
  const topPickArenaId: number | null = useMemo(() => {
    if (packCards.length === 0) return null;
    let best: PackCard | null = null;
    for (const card of packCards) {
      if (card.gihwr === undefined) continue;
      if (!best || card.gihwr > (best.gihwr ?? 0)) best = card;
    }
    return best?.arenaId ?? null;
  }, [packCards]);

  // Picked cards with names from ratings map.
  const pickedCardsInfo = useMemo(() => {
    return session.pickedCards.map((id) => {
      const rating = ratingsMap.get(id);
      return { arenaId: id, name: rating?.name ?? `Card #${id}`, grade: gradeFromGihwr(rating?.gihwr) };
    });
  }, [session.pickedCards, ratingsMap]);

  // ── Render ─────────────────────────────────────────────────────────────────

  // No active draft.
  if (session.sessionStatus === 'idle') {
    return (
      <div className="draft-live-container" data-testid="draft-live-container">
        <div className="draft-live-header">
          <h1>Live Draft</h1>
          <span className={`stream-status stream-status--${streamStatus}`} data-testid="stream-status">
            {streamStatus}
          </span>
        </div>
        <EmptyState
          icon="🎯"
          heading="No active draft"
          subtext="Start a draft in Arena to see your live pick recommendations"
          variant="no-data"
        />
      </div>
    );
  }

  // Draft complete.
  if (session.sessionStatus === 'complete') {
    return (
      <div className="draft-live-container" data-testid="draft-live-container">
        <div className="draft-live-header">
          <h1>Live Draft</h1>
        </div>
        <EmptyState
          icon="✅"
          heading="Draft complete"
          subtext="Your draft session has ended. View your picks in Draft History."
          variant="no-data"
        />
      </div>
    );
  }

  // Active draft.
  return (
    <div className="draft-live-container" data-testid="draft-live-container">
      {/* Header */}
      <div className="draft-live-header">
        <div className="draft-live-title-row">
          <h1>Live Draft</h1>
          <span
            className={`stream-status stream-status--${streamStatus}`}
            data-testid="stream-status"
          >
            {streamStatus}
          </span>
        </div>
        <div className="draft-live-meta" data-testid="draft-live-meta">
          {setCode && (
            <span className="draft-live-set" data-testid="draft-live-set">
              {setCode}
            </span>
          )}
          {draftFormat && (
            <span className="draft-live-format" data-testid="draft-live-format">
              {draftFormat}
            </span>
          )}
          <span className="draft-live-progress" data-testid="draft-live-progress">
            Pack {session.packNumber} · Pick {session.pickNumber}
          </span>
        </div>
      </div>

      <div className="draft-live-body">
        {/* Current Pack */}
        <section className="draft-live-pack-section" data-testid="draft-live-pack">
          <h2>Current Pack</h2>
          {ratingsState.loading && <LoadingSpinner message="Loading ratings..." />}
          {ratingsState.error && (
            <p className="draft-live-ratings-error" data-testid="ratings-error">
              {ratingsState.error}
            </p>
          )}
          {packCards.length === 0 && !ratingsState.loading && (
            <p className="draft-live-waiting" data-testid="pack-waiting">
              Waiting for next pack…
            </p>
          )}
          <div className="draft-live-pack-grid" data-testid="pack-grid">
            {packCards.map((card) => {
              const isTop = card.arenaId === topPickArenaId;
              return (
                <div
                  key={card.arenaId}
                  className={`draft-live-card${isTop ? ' draft-live-card--top' : ''}`}
                  data-testid={`pack-card-${card.arenaId}`}
                  data-top-pick={isTop ? 'true' : undefined}
                >
                  <span className="draft-live-card-name">
                    {card.rating?.name ?? `#${card.arenaId}`}
                  </span>
                  <span
                    className={`draft-live-grade ${gradeClass(card.grade)}`}
                    data-testid={`card-grade-${card.arenaId}`}
                  >
                    {card.grade}
                  </span>
                  {card.gihwr !== undefined && (
                    <span className="draft-live-gihwr">
                      {card.gihwr.toFixed(1)}%
                    </span>
                  )}
                  {isTop && (
                    <span className="draft-live-top-badge" data-testid="top-pick-badge">
                      Top Pick
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        </section>

        {/* Pick History */}
        <section className="draft-live-history-section" data-testid="draft-live-history">
          <h2>Picks ({pickedCardsInfo.length})</h2>
          {pickedCardsInfo.length === 0 ? (
            <p className="draft-live-no-picks">No picks yet</p>
          ) : (
            <div className="draft-live-history-grid">
              {pickedCardsInfo.map((card, idx) => (
                <div
                  key={`${card.arenaId}-${idx}`}
                  className="draft-live-history-item"
                  data-testid={`picked-card-${card.arenaId}`}
                >
                  <span className="draft-live-history-name">{card.name}</span>
                  <span className={`draft-live-grade ${gradeClass(card.grade)}`}>
                    {card.grade}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
};

export default DraftLive;
