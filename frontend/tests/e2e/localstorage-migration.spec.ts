/**
 * E2E tests for localStorage migration shim — ADR-022 Phase 2 (#1749)
 *
 * Verifies that when a user has legacy `mtga-companion-*` keys in localStorage,
 * the app migrates their values to the new `vaultmtg-*` keys on mount and
 * removes the old keys.
 *
 * The migration runs synchronously (before React renders) via main.tsx so the
 * first page.goto() is sufficient to trigger it. We use page.addInitScript to
 * seed the old keys before the page JS runs.
 *
 * Routes used: / (Home) — simplest page that still causes main.tsx to execute
 * and the migration shim to run. The migration is not gated on auth, so we
 * do not need a signed-in Clerk state for these tests.
 */

import { test, expect } from '@playwright/test';

/** All legacy→new key pairs exercised by the migration shim. */
const MIGRATION_PAIRS: [string, string][] = [
  ['mtga-companion-api-key', 'vaultmtg-api-key'],
  ['mtga-companion-settings-expanded', 'vaultmtg-settings-expanded'],
  ['mtga-companion-developer-mode', 'vaultmtg-developer-mode'],
  ['mtga-companion-filters', 'vaultmtg-filters'],
];

/** The sentinel key written when migration completes. */
const MIGRATION_SENTINEL = 'vaultmtg-migration-v1';

/** Retrieve a localStorage value from the browser. */
async function getStorageItem(page: Parameters<typeof test.describe>[1] extends never ? never : never, key: string): Promise<string | null> {
  // Use page.evaluate inline at the call site instead.
  void key;
  return null;
}
void getStorageItem;

test.describe('localStorage migration shim', () => {
  test('migrates all legacy keys to vaultmtg-* equivalents on first load', async ({ page }) => {
    const testValues: Record<string, string> = {
      'mtga-companion-api-key': 'test-api-key-12345',
      'mtga-companion-settings-expanded': JSON.stringify(['connection', 'preferences']),
      'mtga-companion-developer-mode': 'true',
      'mtga-companion-filters': JSON.stringify({ matchHistory: { dateRange: '30days' } }),
    };

    // Seed all legacy keys before the page script executes.
    await page.addInitScript((values: Record<string, string>) => {
      for (const [key, value] of Object.entries(values)) {
        localStorage.setItem(key, value);
      }
    }, testValues);

    await page.goto('/');
    // Wait for the app to mount — the migration runs synchronously in main.tsx
    // before renderApp() so by the time the DOM is ready the migration is done.
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Assert new keys hold the migrated values.
    // Simple scalar/array keys: assert exact string equality.
    const simpleKeys: [string, string][] = MIGRATION_PAIRS.filter(
      ([, newKey]) => newKey !== 'vaultmtg-filters'
    );
    for (const [legacyKey, newKey] of simpleKeys) {
      const newValue = await page.evaluate((k: string) => localStorage.getItem(k), newKey);
      expect(newValue).toBe(testValues[legacyKey]);
    }

    // vaultmtg-filters: AppContext enriches the stored value on mount by spreading
    // defaultFilters over the migrated partial object and writing it back. So the
    // stored JSON will contain more fields than the migrated input. Assert the
    // specific preserved sub-fields instead of exact-string equality.
    const rawFilters = await page.evaluate((k: string) => localStorage.getItem(k), 'vaultmtg-filters');
    expect(rawFilters).not.toBeNull();
    const parsedFilters = JSON.parse(rawFilters!);
    expect(parsedFilters.matchHistory.dateRange).toBe('30days');

    // Assert old keys are gone.
    for (const [legacyKey] of MIGRATION_PAIRS) {
      const oldValue = await page.evaluate((k: string) => localStorage.getItem(k), legacyKey);
      expect(oldValue).toBeNull();
    }

    // Assert migration sentinel is set.
    const sentinel = await page.evaluate(
      (k: string) => localStorage.getItem(k),
      MIGRATION_SENTINEL
    );
    expect(sentinel).toBe('1');
  });

  test('does not overwrite new vaultmtg-* keys that already hold a value', async ({ page }) => {
    // Simulate a user who already has data under the new key (e.g. from a
    // previous partial migration) AND still has the old key present.
    await page.addInitScript(() => {
      localStorage.setItem('vaultmtg-developer-mode', 'false');
      localStorage.setItem('mtga-companion-developer-mode', 'true');
    });

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // New key must keep its pre-existing value.
    const devMode = await page.evaluate((k: string) => localStorage.getItem(k), 'vaultmtg-developer-mode');
    expect(devMode).toBe('false');

    // Legacy key must be removed.
    const legacy = await page.evaluate((k: string) => localStorage.getItem(k), 'mtga-companion-developer-mode');
    expect(legacy).toBeNull();
  });

  test('is idempotent — second page load does not re-run migration', async ({ page }) => {
    const originalFilters = JSON.stringify({ matchHistory: { dateRange: '7days' } });

    // First load — seed legacy key and let migration run.
    await page.addInitScript((v: string) => {
      localStorage.setItem('mtga-companion-filters', v);
    }, originalFilters);

    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // After first migration the new key is set. AppContext enriches the stored
    // value on mount (spreading defaultFilters over the migrated partial), so we
    // assert the specific preserved sub-field rather than exact string equality.
    const afterFirstRaw = await page.evaluate((k: string) => localStorage.getItem(k), 'vaultmtg-filters');
    expect(afterFirstRaw).not.toBeNull();
    const afterFirstParsed = JSON.parse(afterFirstRaw!);
    expect(afterFirstParsed.matchHistory.dateRange).toBe('7days');

    // Inject a stale legacy key again (simulating some unusual scenario).
    await page.evaluate(() => {
      localStorage.setItem('mtga-companion-filters', '{"stale":true}');
    });

    // Reload — migration sentinel is already set so it must NOT re-run.
    await page.reload();
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    const afterSecondRaw = await page.evaluate((k: string) => localStorage.getItem(k), 'vaultmtg-filters');
    // New key must retain the value from the FIRST migration (dateRange '7days'),
    // not the stale legacy value. Assert the preserved sub-field.
    expect(afterSecondRaw).not.toBeNull();
    const afterSecondParsed = JSON.parse(afterSecondRaw!);
    expect(afterSecondParsed.matchHistory.dateRange).toBe('7days');
    // Stale value must not have polluted the stored filters.
    expect(afterSecondParsed.stale).toBeUndefined();
  });

  test('runs cleanly when no legacy keys are present', async ({ page }) => {
    // Fresh user — no legacy keys at all.
    await page.goto('/');
    await expect(page.locator('[data-testid="app-container"]')).toBeVisible();

    // Sentinel should still be set.
    const sentinel = await page.evaluate(
      (k: string) => localStorage.getItem(k),
      MIGRATION_SENTINEL
    );
    expect(sentinel).toBe('1');

    // No legacy vaultmtg-* keys should be created from nothing by the migration shim.
    // Note: AppContext writes vaultmtg-filters on mount (it always persists filter
    // state), so we only verify the non-filter keys are absent. vaultmtg-filters is
    // written by AppContext regardless of migration, which is correct behaviour.
    const nonFilterKeys: [string, string][] = MIGRATION_PAIRS.filter(
      ([, newKey]) => newKey !== 'vaultmtg-filters'
    );
    for (const [, newKey] of nonFilterKeys) {
      const val = await page.evaluate((k: string) => localStorage.getItem(k), newKey);
      expect(val).toBeNull();
    }

    // vaultmtg-filters may be written by AppContext with default values — that is
    // correct and expected. What we verify here is that no legacy data was injected:
    // the stored value should reflect defaultFilters (no stale legacy sub-values).
    const filtersRaw = await page.evaluate((k: string) => localStorage.getItem(k), 'vaultmtg-filters');
    if (filtersRaw !== null) {
      // AppContext wrote defaults — assert the expected default dateRange value.
      const parsedFilters = JSON.parse(filtersRaw);
      expect(parsedFilters.matchHistory.dateRange).toBe('7days');
    }
  });
});
