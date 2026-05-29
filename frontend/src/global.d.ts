/**
 * Global type augmentations for VaultMTG.
 *
 * __VAULTMTG_DESKTOP__ is injected by the desktop app shell at startup.
 * It is `true` when the SPA is running inside the native desktop wrapper
 * and `undefined` in all browser/staging contexts.
 *
 * Always guard daemon-specific code with:
 *   if (!window.__VAULTMTG_DESKTOP__) return;
 *
 * __POSTHOG_TEST_FLAGS__ is injected by Playwright tests via addInitScript()
 * to override feature flag values without requiring PostHog to be initialized.
 * Keys are flag names; values are boolean flag states. When a key is present
 * the value takes precedence over posthog.isFeatureEnabled(). This window
 * global is undefined in all non-test environments.
 */
declare interface Window {
  __VAULTMTG_DESKTOP__?: true;
  __POSTHOG_TEST_FLAGS__?: Record<string, boolean>;
}
