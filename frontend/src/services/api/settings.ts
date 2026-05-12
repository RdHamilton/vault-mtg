/**
 * Settings API service.
 *
 * Phase 2 PR #12: account-scoped settings now hit the BFF via apiClient.
 * Backing store is the user_settings table (JSONB key/value, scoped by
 * account_id). The SPA's AppSettings constructor applies defaults for
 * missing keys, so a brand-new account just receives an empty object
 * on GET /settings and renders defaults locally.
 *
 * Plan tracker: .claude/plans/spa-route-migration.md
 */

import { get, put } from '../apiClient';
import { gui } from '@/types/models';

// Re-export types for convenience
export type AppSettings = gui.AppSettings;

/**
 * Get all settings for the authenticated account.
 */
export async function getSettings(): Promise<AppSettings> {
  return get<AppSettings>('/settings');
}

/**
 * Replace all settings for the authenticated account.
 */
export async function updateSettings(settings: AppSettings): Promise<void> {
  await put('/settings', settings);
}

/**
 * Get a single setting by key. The BFF returns `{ value: ... }`; callers
 * that want the raw value can read `.value` off the result.
 */
export async function getSetting(key: string): Promise<unknown> {
  return get<unknown>(`/settings/${encodeURIComponent(key)}`);
}

/**
 * Update a single setting. The BFF accepts `{ value: ... }` and stores
 * the value verbatim as JSONB.
 */
export async function updateSetting(key: string, value: unknown): Promise<void> {
  await put(`/settings/${encodeURIComponent(key)}`, { value });
}
