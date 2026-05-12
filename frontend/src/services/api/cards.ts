/**
 * Cards API service.
 *
 * Phase 2 PR #8: cloud-data card metadata, set catalog, 17Lands ratings,
 * and ChannelFireball ratings now hit the BFF directly via apiClient.
 * /collection-quantities and /search-with-collection are the two
 * account-scoped endpoints. refresh-ratings is a documented BFF stub
 * pending the scrape pipeline. URL paths unchanged in this file.
 *
 * Plan tracker: .claude/plans/spa-route-migration.md
 */

import { get, post, del, getRaw } from '../apiClient';
import { models, gui, seventeenlands } from '@/types/models';

// Re-export types for convenience
export type SetCard = models.SetCard;
export type CardRatingWithTier = gui.CardRatingWithTier;
export type SetInfo = gui.SetInfo;

/**
 * Search request for cards.
 */
export interface CardSearchRequest {
  query: string;
  set_code?: string;
  colors?: string[];
  types?: string[];
  rarity?: string;
  limit?: number;
}

/**
 * Search for cards.
 * Uses GET with query parameters as the backend expects.
 */
export async function searchCards(request: CardSearchRequest): Promise<SetCard[]> {
  const params = new URLSearchParams();
  params.set('q', request.query);
  if (request.set_code) {
    params.set('set', request.set_code);
  }
  if (request.limit) {
    params.set('limit', request.limit.toString());
  }
  return get<SetCard[]>(`/cards?${params.toString()}`);
}

/**
 * Get a card by Arena ID.
 */
export async function getCardByArenaId(arenaId: number): Promise<SetCard> {
  return get<SetCard>(`/cards/${arenaId}`);
}

/**
 * Get all set information.
 */
export async function getAllSetInfo(): Promise<SetInfo[]> {
  return get<SetInfo[]>('/cards/sets');
}

/**
 * Get cards for a specific set.
 */
export async function getSetCards(setCode: string): Promise<SetCard[]> {
  return get<SetCard[]>(`/cards/sets/${setCode}/cards`);
}

/**
 * Get card ratings for a set and format.
 */
export async function getCardRatings(
  setCode: string,
  format: string
): Promise<CardRatingWithTier[]> {
  return get<CardRatingWithTier[]>(`/cards/ratings/${setCode}/${format}`);
}

/**
 * Result from fetching card ratings, including cache degraded status from
 * the X-Cache-Degraded and X-Cache-Age-Hours response headers.
 */
export interface CardRatingsResult {
  ratings: CardRatingWithTier[];
  /** True when the BFF returned X-Cache-Degraded: true — data may be stale. */
  cacheDegraded: boolean;
  /**
   * Number of hours since cached ratings were last refreshed from upstream
   * (17Lands). Parsed from X-Cache-Age-Hours header.
   * Undefined when the header is absent or non-numeric.
   */
  cacheAgeHours?: number;
}

/**
 * Get card ratings for a set and format, exposing the X-Cache-Degraded and
 * X-Cache-Age-Hours headers so the UI can show a degraded-mode notice.
 */
export async function getCardRatingsWithDegradedFlag(
  setCode: string,
  format: string
): Promise<CardRatingsResult> {
  const { data, headers } = await getRaw<CardRatingWithTier[]>(`/cards/ratings/${setCode}/${format}`);
  const cacheDegraded = headers.get('x-cache-degraded') === 'true';
  const rawAge = headers.get('x-cache-age-hours');
  const parsedAge = rawAge !== null ? parseFloat(rawAge) : undefined;
  const cacheAgeHours =
    parsedAge !== undefined && !isNaN(parsedAge) ? parsedAge : undefined;
  return { ratings: data ?? [], cacheDegraded, cacheAgeHours };
}

/**
 * Get collection quantities for cards.
 */
export async function getCollectionQuantities(
  arenaIds: number[]
): Promise<Record<number, number>> {
  return post<Record<number, number>>('/cards/collection-quantities', { arenaIDs: arenaIds });
}

/**
 * Get color ratings for a set and format.
 */
export async function getColorRatings(
  setCode: string,
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  _format: string // Format parameter preserved for API compatibility but not used in backend path
): Promise<seventeenlands.ColorRating[]> {
  return get<seventeenlands.ColorRating[]>(`/cards/ratings/${setCode}/colors`);
}

/**
 * Card with collection quantity info.
 */
export interface CardWithCollection extends SetCard {
  quantity: number;
}

/**
 * Search cards with collection info.
 */
export async function searchCardsWithCollection(
  query: string,
  sets?: string[],
  limit?: number
): Promise<CardWithCollection[]> {
  return post<CardWithCollection[]>('/cards/search-with-collection', {
    query,
    setCodes: sets,
    limit,
  });
}

/**
 * Ratings staleness information.
 */
export interface RatingsStaleness {
  cachedAt: string;
  isStale: boolean;
  cardCount: number;
}

/**
 * Check if ratings for a set are stale.
 */
export async function getRatingsStaleness(
  setCode: string,
  format: string
): Promise<RatingsStaleness> {
  return get<RatingsStaleness>(`/cards/ratings/${setCode}/${format}/staleness`);
}

/**
 * Refresh ratings for a set (force re-download from 17Lands).
 */
export async function refreshSetRatings(
  setCode: string,
  format: string = 'PremierDraft'
): Promise<void> {
  await post(`/cards/ratings/${setCode}/refresh`, { format });
}

// ============================================================================
// ChannelFireball (CFB) Ratings
// ============================================================================

/**
 * CFB rating grades for Limited (display purposes).
 */
export type CFBLimitedGrade = 'A+' | 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'C-' | 'D' | 'F';

/**
 * CFB constructed playability ratings.
 */
export type CFBConstructedRating = 'Staple' | 'Playable' | 'Fringe' | 'Unplayable';

/**
 * Convert a numerical rating (0-5 scale) to a letter grade for display.
 */
export function ratingToGrade(rating: number): CFBLimitedGrade {
  if (rating >= 4.75) return 'A+';
  if (rating >= 4.25) return 'A';
  if (rating >= 3.75) return 'A-';
  if (rating >= 3.25) return 'B+';
  if (rating >= 2.75) return 'B';
  if (rating >= 2.25) return 'B-';
  if (rating >= 1.75) return 'C+';
  if (rating >= 1.25) return 'C';
  if (rating >= 0.75) return 'C-';
  if (rating >= 0.25) return 'D';
  return 'F';
}

/**
 * ChannelFireball card rating.
 */
export interface CFBRating {
  id: number;
  cardName: string;
  setCode: string;
  arenaId?: number;
  /** Numerical rating on 0.0-5.0 scale (matching TCGPlayer/MTG Arena Zone format) */
  limitedRating: number;
  /** Normalized score (0.0-1.0) for internal calculations */
  limitedScore: number;
  constructedRating?: CFBConstructedRating;
  constructedScore?: number;
  archetypeFit?: string;
  commentary?: string;
  sourceUrl?: string;
  author?: string;
  importedAt: string;
  updatedAt: string;
}

/**
 * CFB rating import data structure.
 */
export interface CFBRatingImport {
  card_name: string;
  set_code: string;
  /** Numerical rating on 0.0-5.0 scale */
  limited_rating: number;
  constructed_rating?: CFBConstructedRating;
  archetype_fit?: string;
  commentary?: string;
  source_url?: string;
  author?: string;
}

/**
 * Get CFB ratings for a set.
 */
export async function getCFBRatings(setCode: string): Promise<CFBRating[]> {
  return get<CFBRating[]>(`/cards/cfb/${setCode}`);
}

/**
 * Get CFB rating count for a set.
 */
export async function getCFBRatingsCount(setCode: string): Promise<{ set_code: string; count: number }> {
  return get<{ set_code: string; count: number }>(`/cards/cfb/${setCode}/count`);
}

/**
 * Get CFB rating for a specific card.
 */
export async function getCFBRatingByCard(setCode: string, cardName: string): Promise<CFBRating> {
  return get<CFBRating>(`/cards/cfb/${setCode}/card/${encodeURIComponent(cardName)}`);
}

/**
 * Import CFB ratings.
 */
export async function importCFBRatings(
  ratings: CFBRatingImport[]
): Promise<{ status: string; imported: number; message: string }> {
  return post<{ status: string; imported: number; message: string }>('/cards/cfb/import', { ratings });
}

/**
 * Link CFB ratings to Arena IDs for a set.
 */
export async function linkCFBArenaIds(
  setCode: string
): Promise<{ status: string; set_code: string; linked: number; message: string }> {
  return post<{ status: string; set_code: string; linked: number; message: string }>(
    `/cards/cfb/${setCode}/link-arena-ids`,
    {}
  );
}

/**
 * Delete CFB ratings for a set.
 */
export async function deleteCFBRatings(setCode: string): Promise<void> {
  await del(`/cards/cfb/${setCode}`);
}

/**
 * Convert CFB limited grade to display color.
 */
export function getCFBGradeColor(grade: CFBLimitedGrade): string {
  const colors: Record<CFBLimitedGrade, string> = {
    'A+': '#ffd700', // Gold
    'A': '#c0c0c0',  // Silver
    'A-': '#b8b8b8', // Light silver
    'B+': '#cd7f32', // Bronze
    'B': '#b87333',  // Copper
    'B-': '#a0522d', // Sienna
    'C+': '#4a9eff', // Blue
    'C': '#3a8eef',  // Darker blue
    'C-': '#2a7edf', // Even darker blue
    'D': '#888888',  // Gray
    'F': '#ff4444',  // Red
  };
  return colors[grade] || '#888888';
}

/**
 * Get display color for a numerical CFB rating (0-5 scale).
 */
export function getCFBRatingColor(rating: number): string {
  return getCFBGradeColor(ratingToGrade(rating));
}
