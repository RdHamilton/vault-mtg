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
 *
 * Pagination (fix for vault-mtg-tickets#179):
 * - Uses per_page=100 and paginates up to MAX_PAGES pages.
 * - Stops as soon as a matching daemon/v* release is found.
 * - Prevents a daemon release from being missed when non-daemon tags or RCs
 *   crowd the first page at high release volume.
 */

const GITHUB_REPO = 'RdHamilton/vault-mtg';
const RELEASES_BASE = `https://github.com/${GITHUB_REPO}/releases/download`;
const LATEST_RELEASE_URL = `https://api.github.com/repos/${GITHUB_REPO}/releases`;

/** Number of results fetched per page. 100 is the GitHub API maximum. */
const PER_PAGE = 100;

/** Maximum pages to paginate before giving up. Caps at 500 releases checked. */
const MAX_PAGES = 5;

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

type GitHubRelease = { tag_name: string; draft: boolean; prerelease: boolean };

/**
 * Returns true when a release entry is a candidate for the current environment.
 */
function isCandidate(r: GitHubRelease): boolean {
  if (r.draft) return false;
  if (!r.tag_name.startsWith('daemon/v')) return false;
  // On prod, skip prereleases so an RC tag never leaks to the prod download page.
  if (!isStaging() && r.prerelease) return false;
  return true;
}

/**
 * Fetch the most-recent release whose tag starts with "daemon/v".
 *
 * Paginates through the GitHub Releases API (per_page=100, up to MAX_PAGES
 * pages) so the resolver remains correct even when non-daemon tags or release
 * candidates push the newest stable daemon release off page 1.
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
    for (let page = 1; page <= MAX_PAGES; page++) {
      const response = await fetch(
        `${LATEST_RELEASE_URL}?per_page=${PER_PAGE}&page=${page}`,
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

      const releases: GitHubRelease[] = await response.json();

      // Empty page means we have exhausted all releases.
      if (releases.length === 0) {
        break;
      }

      const match = releases.find(isCandidate);
      if (match) {
        return {
          tag: match.tag_name,
          downloadBase: `${RELEASES_BASE}/${match.tag_name}`,
        };
      }

      // If the page was not full, there are no more pages to fetch.
      if (releases.length < PER_PAGE) {
        break;
      }
    }

    console.warn('[daemonRelease] No daemon/v* release found — falling back');
    return null;
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
