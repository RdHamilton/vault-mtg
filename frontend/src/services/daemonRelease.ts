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
 */

const GITHUB_REPO = 'RdHamilton/vault-mtg';
const RELEASES_BASE = `https://github.com/${GITHUB_REPO}/releases/download`;
const LATEST_RELEASE_URL = `https://api.github.com/repos/${GITHUB_REPO}/releases`;

export interface DaemonReleaseInfo {
  /** Full tag name, e.g. "daemon/v0.3.2" */
  tag: string;
  /** Base download URL including the tag segment, ready for artifact filename appended with "/". */
  downloadBase: string;
}

/**
 * Fetch the most-recent release whose tag starts with "daemon/v".
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

    const match = releases.find(
      (r) => !r.draft && r.tag_name.startsWith('daemon/v')
    );

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
