/**
 * BFF match history adapter.
 *
 * Targets GET /api/v1/history/matches on the BFF.
 * The endpoint is Clerk-protected and returns a paginated list of matches.
 *
 * Response shape:
 *   { matches: MatchHistoryItem[], total: number, limit: number, offset: number }
 */

import { getApiConfig, ApiRequestError } from '../apiClient';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface MatchHistoryItem {
  id: number;
  opponent_deck: string;
  result: string;
  format: string;
  played_at: string;
}

export interface MatchHistoryParams {
  limit?: number;
  offset?: number;
}

export interface MatchHistoryResponse {
  matches: MatchHistoryItem[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

/**
 * Fetch paginated match history from the BFF.
 *
 * Targets: GET /api/v1/history/matches
 *
 * The endpoint is Clerk-protected — pass the Clerk session token obtained via
 * `useAuth().getToken()` as the `clerkToken` parameter.
 *
 * @param clerkToken  Clerk session JWT returned by useAuth().getToken()
 * @param params      Optional pagination params (limit, offset)
 */
export async function getMatchHistory(
  clerkToken: string,
  params: MatchHistoryParams = {}
): Promise<MatchHistoryResponse> {
  const { baseUrl } = getApiConfig();
  const url = new URL(`${baseUrl}/history/matches`);

  if (params.limit !== undefined) {
    url.searchParams.set('limit', String(params.limit));
  }
  if (params.offset !== undefined) {
    url.searchParams.set('offset', String(params.offset));
  }

  const response = await fetch(url.toString(), {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${clerkToken}`,
    },
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

  return response.json() as Promise<MatchHistoryResponse>;
}
