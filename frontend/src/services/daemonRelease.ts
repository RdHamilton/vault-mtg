/**
 * Daemon Release Adapter
 *
 * Queries the GitHub Releases API at runtime to resolve the latest daemon
 * release tag. This eliminates the VITE_DAEMON_VERSION build-time bake-in that
 * caused a stale download URL in staging during v0.3.1 (post-mortem A7).
 *
 * The adapter is a plain async function — no React state, no fetch inside
 * components — so it can be stubbed cleanly in both vitest component tests and
 * Playwright E2E tests.
 *
 * Release channel selection (env-aware):
 * - staging (VITE_SENTRY_ENV === "staging"): includes prerelease builds so RC
 *   tags such as daemon/v0.3.3-rc1 are served on the staging Download page.
 * - production / any other value: excludes prereleases — stable channel only.
 *   This is the fail-safe default: an unknown env value behaves like production
 *   so a daemon RC can never leak onto the prod Download page.
 */

const GITHUB_REPO = 'RdHamilton/vault-mtg';
const RELEASES_BASE = `https://github.com/${GITHUB_REPO}/releases/download`;
const LATEST_RELEASE_URL = `https://api.github.com/repos/${GITHUB_REPO}/releases`;

/**
 * Returns true only when the build was produced by deploy-spa-staging.yml.
 * Any value other than "staging" (including undefined / "production") is treated
 * as prod-safe: exclude prereleases.
 *
 * Evaluated at call time (not module load time) so that vi.stubEnv() works
 * correctly in unit tests without requiring module re-imports.
 */
function isStaging(): boolean {
  return import.meta.env.VITE_SENTRY_ENV === 'staging';
}

export interface DaemonReleaseInfo {
  /** Full tag name, e.g. "daemon/v0.3.2" */
  tag: string;
  /** Base download URL including the tag segment, ready for artifact filename appended with "/". */
  downloadBase: string;
}

/**
 * Fetch the most-recent release whose tag starts with "daemon/v".
 *
 * Channel behaviour:
 * - staging env  → accepts prereleases (daemon/v*-rc* are valid targets).
 * - prod env     → excludes prereleases (stable channel only).
 *
 * @param signal  Optional AbortSignal for cancellation.
 * @returns Resolved release info, or null if the fetch failed or no matching
 *          release was found (caller should fall back gracefully).
 */
export async function fetchLatestDaemonRelease(
  signal?: AbortSignal
): Promise<DaemonReleaseInfo | null> {
  try {
    const response = await fetch(
      `${LATEST_RELEASE_URL}?per_page=20`,
      {
        signal,
        headers: {
          Accept: 'application/vnd.github+json',
          'X-GitHub-Api-Version': '2022-11-28',
        },
      }
    );

    if (!response.ok) {
      console.warn(
        `[daemonRelease] GitHub Releases API returned ${response.status} — falling back`
      );
      return null;
    }

    const releases: Array<{ tag_name: string; draft: boolean; prerelease: boolean }> =
      await response.json();

    const match = releases.find((r) => {
      if (r.draft) return false;
      if (!r.tag_name.startsWith('daemon/v')) return false;
      // On prod, skip prereleases so an RC tag never leaks to the prod download page.
      if (!isStaging() && r.prerelease) return false;
      return true;
    });

    if (!match) {
      console.warn('[daemonRelease] No daemon/v* release found — falling back');
      return null;
    }

    return {
      tag: match.tag_name,
      downloadBase: `${RELEASES_BASE}/${match.tag_name}`,
    };
  } catch (err) {
    if ((err as Error).name === 'AbortError') {
      return null;
    }
    console.warn('[daemonRelease] Failed to fetch release info:', err);
    return null;
  }
}

/**
 * Fallback download base used when the runtime fetch fails or is still in
 * flight. GitHub's /releases/latest/download/ redirects to the most-recent
 * non-prerelease, so it is safe for production. For staging environments that
 * need a specific pre-release pinned, the runtime fetch will resolve first.
 */
export const FALLBACK_DOWNLOAD_BASE = `https://github.com/${GITHUB_REPO}/releases/latest/download`;
