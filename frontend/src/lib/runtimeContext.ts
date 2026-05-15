/**
 * runtimeContext — centralised runtime environment detection.
 *
 * Use `isDesktopApp()` everywhere you need to gate daemon-specific behaviour.
 * Never write `window.__VAULTMTG_DESKTOP__` inline in components or hooks.
 *
 * `window.__VAULTMTG_DESKTOP__` is injected by the native desktop wrapper at
 * startup and is `undefined` in all browser/staging/test environments.
 */
export function isDesktopApp(): boolean {
  return !!window.__VAULTMTG_DESKTOP__;
}
