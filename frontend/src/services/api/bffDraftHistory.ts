/**
 * BFF draft history adapter.
 *
 * Targets GET /api/v1/history/drafts on the BFF.
 * The endpoint is Clerk-protected and returns a paginated list of drafts.
 *
 * Response shape:
 *   { drafts: DraftHistoryItem[], total: number, limit: number, offset: number }
 */

import { getApiConfig, ApiRequestError } from '../apiClient';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface DraftHistoryItem {
  id: number;
  set_code: string;
  wins: number;
  losses: number;
  drafted_at: string;
}

export interface DraftHistoryParams {
  limit?: number;
  offset?: number;
}

export interface DraftHistoryResponse {
  drafts: DraftHistoryItem[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

/**
 * Fetch paginated draft history from the BFF.
 *
 * Targets: GET /api/v1/history/drafts
 *
 * The endpoint is Clerk-protected — pass the Clerk session token obtained via
 * `useAuth().getToken()` as the `clerkToken` parameter.
 *
 * @param clerkToken  Clerk session JWT returned by useAuth().getToken()
 * @param params      Optional pagination params (limit, offset)
 */
export async function getDraftHistory(
  clerkToken: string,
  params: DraftHistoryParams = {}
): Promise<DraftHistoryResponse> {
  const { baseUrl } = getApiConfig();
  const url = new URL(`${baseUrl}/history/drafts`);

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

  return response.json() as Promise<DraftHistoryResponse>;
}
