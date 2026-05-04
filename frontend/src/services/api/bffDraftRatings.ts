/**
 * BFF draft-ratings adapter.
 *
 * Targets GET /api/v1/draft-ratings/{setCode}/{format} on the BFF.
 * This is a separate endpoint from the legacy /cards/ratings/ path served by
 * the main API.  It is fully wired once Vercel→BFF connectivity is resolved
 * (see ticket #1025).
 *
 * The response includes X-Cache-Degraded and X-Cache-Age-Hours headers when
 * data is stale; this adapter surfaces them via BffDraftRatingsResult.
 */

import { getApiConfig, ApiRequestError } from '../apiClient';

// ---------------------------------------------------------------------------
// Types matching the BFF draftRatingsResponse JSON envelope (draft_ratings.go)
// ---------------------------------------------------------------------------

export interface BffCardRating {
  arena_id: number;
  name: string;
  color?: string;
  rarity?: string;
  gihwr?: number;
  ohwr?: number;
  alsa?: number;
  ata?: number;
  gih_count?: number;
}

export interface BffColorRating {
  color_combination: string;
  win_rate?: number;
  games_played?: number;
}

export interface BffDraftRatingsResponse {
  set_code: string;
  draft_format: string;
  cached_at: string;
  card_ratings: BffCardRating[];
  color_ratings: BffColorRating[];
}

export interface BffDraftRatingsResult {
  data: BffDraftRatingsResponse;
  /** True when the BFF returned X-Cache-Degraded: true — data may be stale. */
  cacheDegraded: boolean;
  /**
   * Hours since the ratings were last refreshed from 17Lands.
   * Parsed from X-Cache-Age-Hours. Undefined when header is absent/non-numeric.
   */
  cacheAgeHours?: number;
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

/**
 * Fetch draft card and color ratings from the BFF.
 *
 * Targets: GET /api/v1/draft-ratings/{setCode}/{format}
 *
 * Note: The BFF returns the JSON envelope directly (not wrapped in { data: ... }),
 * so we issue a raw fetch rather than using apiClient.getRaw which unwraps a
 * data envelope.
 *
 * @param setCode   Three-letter MTG set code, e.g. "ONE"
 * @param format    Draft format, e.g. "PremierDraft"
 */
export async function getDraftRatings(
  setCode: string,
  format: string
): Promise<BffDraftRatingsResult> {
  const { baseUrl } = getApiConfig();
  const url = `${baseUrl}/draft-ratings/${encodeURIComponent(setCode)}/${encodeURIComponent(format)}`;

  const response = await fetch(url, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  });

  if (!response.ok) {
    let errorMessage = response.statusText || 'Request failed';
    try {
      const body = (await response.json()) as { error?: string; message?: string };
      errorMessage = body.message ?? body.error ?? errorMessage;
    } catch {
      // ignore parse failure — use statusText
    }
    throw new ApiRequestError(errorMessage, response.status);
  }

  const data = (await response.json()) as BffDraftRatingsResponse;

  const cacheDegraded = response.headers.get('x-cache-degraded') === 'true';
  const rawAge = response.headers.get('x-cache-age-hours');
  const parsedAge = rawAge !== null ? parseFloat(rawAge) : undefined;
  const cacheAgeHours =
    parsedAge !== undefined && !isNaN(parsedAge) ? parsedAge : undefined;

  return { data, cacheDegraded, cacheAgeHours };
}

