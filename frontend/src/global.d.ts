/**
 * Global type augmentations for VaultMTG.
 *
 * __VAULTMTG_DESKTOP__ is injected by the desktop app shell at startup.
 * It is `true` when the SPA is running inside the native desktop wrapper
 * and `undefined` in all browser/staging contexts.
 *
 * Always guard daemon-specific code with:
 *   if (!window.__VAULTMTG_DESKTOP__) return;
 */
declare interface Window {
  __VAULTMTG_DESKTOP__?: true;
}
