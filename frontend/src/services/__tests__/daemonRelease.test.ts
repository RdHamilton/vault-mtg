/**
 * daemonRelease service tests — post-mortem A7
 *
 * Verifies that fetchLatestDaemonRelease correctly queries the GitHub Releases
 * API at runtime, filters to `daemon/v*` tags, and falls back gracefully when
 * the API is unavailable.
 *
 * Env-channel tests (added for the env-aware prerelease feature):
 * - staging env (VITE_SENTRY_ENV=staging): resolves the newest daemon/v* tag
 *   regardless of whether it is a prerelease.
 * - prod env (VITE_SENTRY_ENV=production, or any non-staging value): skips
 *   prerelease tags and resolves the newest stable daemon/v* tag only.
 *
 * Pagination tests (added for vault-mtg-tickets#179):
 * - When page 1 contains no daemon/v* stable releases (crowded by non-daemon
 *   tags or RCs), the resolver paginates to page 2+ and finds the matching tag.
 * - Stops paginating when a page returns fewer than per_page results (last page).
 * - Returns null when all pages are exhausted with no match (respects MAX_PAGES cap).
 *
 * These tests run in the Node environment (matched by the vitest environmentMatchGlobs
 * for service test files) — fetch is mocked with vi.fn().
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  fetchLatestDaemonRelease,
  FALLBACK_DOWNLOAD_BASE,
} from '../daemonRelease';

const GITHUB_REPO = 'RdHamilton/vault-mtg';
const RELEASES_API = `https://api.github.com/repos/${GITHUB_REPO}/releases`;

const mockFetch = vi.fn();
global.fetch = mockFetch;

/** Build a minimal GitHub release payload entry. */
function makeRelease(tag: string, opts: { draft?: boolean; prerelease?: boolean } = {}) {
  return {
    tag_name: tag,
    draft: opts.draft ?? false,
    prerelease: opts.prerelease ?? false,
  };
}

/** Build a successful GitHub API Response mock. */
function githubResponse(releases: ReturnType<typeof makeRelease>[]) {
  return {
    ok: true,
    status: 200,
    json: () => Promise.resolve(releases),
  };
}

describe('fetchLatestDaemonRelease', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
  });

  // ---------------------------------------------------------------------------
  // Happy path
  // ---------------------------------------------------------------------------

  it('returns the first daemon/v* release in the list', async () => {
    mockFetch.mockResolvedValueOnce(
      githubResponse([
        makeRelease('daemon/v0.3.1'),
        makeRelease('daemon/v0.3.1'),
        makeRelease('app/v1.0.0'),
      ])
    );

    const result = await fetchLatestDaemonRelease();

    expect(result).not.toBeNull();
    expect(result!.tag).toBe('daemon/v0.3.1');
    expect(result!.downloadBase).toBe(
      `https://github.com/${GITHUB_REPO}/releases/download/daemon/v0.3.1`
    );
  });

  it('skips non-daemon releases and returns the first matching one', async () => {
    mockFetch.mockResolvedValueOnce(
      githubResponse([
        makeRelease('app/v2.0.0'),
        makeRelease('sync/v1.5.0'),
        makeRelease('daemon/v0.3.1'),
      ])
    );

    const result = await fetchLatestDaemonRelease();

    expect(result).not.toBeNull();
    expect(result!.tag).toBe('daemon/v0.3.1');
  });

  it('sends the correct GitHub API version header', async () => {
    mockFetch.mockResolvedValueOnce(githubResponse([makeRelease('daemon/v0.3.1')]));

    await fetchLatestDaemonRelease();

    expect(mockFetch).toHaveBeenCalledOnce();
    const [, options] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect((options.headers as Record<string, string>)['X-GitHub-Api-Version']).toBe('2022-11-28');
    expect((options.headers as Record<string, string>)['Accept']).toBe(
      'application/vnd.github+json'
    );
  });

  it('queries page 1 with per_page=100', async () => {
    mockFetch.mockResolvedValueOnce(githubResponse([makeRelease('daemon/v0.3.1')]));

    await fetchLatestDaemonRelease();

    const [url] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('per_page=100');
    expect(url).toContain('page=1');
  });

  // ---------------------------------------------------------------------------
  // Draft / pre-release filtering
  // ---------------------------------------------------------------------------

  it('skips draft releases', async () => {
    mockFetch.mockResolvedValueOnce(
      githubResponse([
        makeRelease('daemon/v0.3.3', { draft: true }),
        makeRelease('daemon/v0.3.1'),
      ])
    );

    const result = await fetchLatestDaemonRelease();

    expect(result!.tag).toBe('daemon/v0.3.1');
  });

  it('does NOT skip prerelease releases in staging env (prereleases are valid staging targets)', async () => {
    vi.stubEnv('VITE_SENTRY_ENV', 'staging');
    mockFetch.mockResolvedValueOnce(
      githubResponse([makeRelease('daemon/v0.4.0-rc1', { prerelease: true })])
    );

    const result = await fetchLatestDaemonRelease();

    expect(result).not.toBeNull();
    expect(result!.tag).toBe('daemon/v0.4.0-rc1');
  });

  // ---------------------------------------------------------------------------
  // Fallback / error cases
  // ---------------------------------------------------------------------------

  it('returns null when the API response is not ok', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 403 });

    const result = await fetchLatestDaemonRelease();

    expect(result).toBeNull();
  });

  it('returns null when no daemon/v* release exists in the list', async () => {
    mockFetch.mockResolvedValueOnce(
      githubResponse([makeRelease('app/v1.0.0'), makeRelease('sync/v0.5.0')])
    );

    const result = await fetchLatestDaemonRelease();

    expect(result).toBeNull();
  });

  it('returns null when the releases list is empty', async () => {
    mockFetch.mockResolvedValueOnce(githubResponse([]));

    const result = await fetchLatestDaemonRelease();

    expect(result).toBeNull();
  });

  it('returns null and does not throw when fetch throws a network error', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'));

    const result = await fetchLatestDaemonRelease();

    expect(result).toBeNull();
  });

  it('returns null when the request is aborted', async () => {
    const controller = new AbortController();
    mockFetch.mockImplementationOnce(() => {
      controller.abort();
      const err = new Error('The operation was aborted.');
      err.name = 'AbortError';
      return Promise.reject(err);
    });

    const result = await fetchLatestDaemonRelease(controller.signal);

    expect(result).toBeNull();
  });

  // ---------------------------------------------------------------------------
  // Env-channel: staging vs prod prerelease filtering
  //
  // Both suites use the same mock release list:
  //   - daemon/v0.3.3-rc1  (prerelease: true)   ← newest tag in list
  //   - daemon/v0.3.2      (prerelease: false)   ← latest stable
  //   - app/v1.0.0                               ← non-daemon (always ignored)
  //
  // staging  → resolves daemon/v0.3.3-rc1 (RC is a valid staging target)
  // prod     → resolves daemon/v0.3.2     (RC is excluded from stable channel)
  // ---------------------------------------------------------------------------

  describe('env-channel: staging (VITE_SENTRY_ENV=staging)', () => {
    beforeEach(() => {
      vi.stubEnv('VITE_SENTRY_ENV', 'staging');
    });

    it('resolves the RC prerelease when staging env is set', async () => {
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.3.3-rc1', { prerelease: true }),
          makeRelease('daemon/v0.3.2'),
          makeRelease('app/v1.0.0'),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.3.3-rc1');
    });

    it('still skips draft releases even in staging env', async () => {
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.3.3-rc1', { draft: true, prerelease: true }),
          makeRelease('daemon/v0.3.2'),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.3.2');
    });
  });

  describe('env-channel: prod (VITE_SENTRY_ENV=production)', () => {
    beforeEach(() => {
      vi.stubEnv('VITE_SENTRY_ENV', 'production');
    });

    it('skips the RC prerelease and resolves the stable release', async () => {
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.3.3-rc1', { prerelease: true }),
          makeRelease('daemon/v0.3.2'),
          makeRelease('app/v1.0.0'),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.3.2');
    });

    it('returns null when only a prerelease is available in prod env', async () => {
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.3.3-rc1', { prerelease: true }),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).toBeNull();
    });
  });

  describe('env-channel: fail-safe default (unknown / undefined VITE_SENTRY_ENV)', () => {
    it('excludes prereleases when VITE_SENTRY_ENV is undefined (safe default = prod behaviour)', async () => {
      // No vi.stubEnv call — env var is absent, IS_STAGING evaluates to false.
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.3.3-rc1', { prerelease: true }),
          makeRelease('daemon/v0.3.2'),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.3.2');
    });

    it('excludes prereleases when VITE_SENTRY_ENV is an unrecognised value', async () => {
      vi.stubEnv('VITE_SENTRY_ENV', 'preview');
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.3.3-rc1', { prerelease: true }),
          makeRelease('daemon/v0.3.2'),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.3.2');
    });
  });

  // ---------------------------------------------------------------------------
  // FALLBACK_DOWNLOAD_BASE constant
  // ---------------------------------------------------------------------------

  it('FALLBACK_DOWNLOAD_BASE points to /releases/latest/download', () => {
    expect(FALLBACK_DOWNLOAD_BASE).toBe(
      `https://github.com/${GITHUB_REPO}/releases/latest/download`
    );
  });

  // ---------------------------------------------------------------------------
  // Pagination: latest stable is beyond page 1 / page 1 crowded by non-daemon tags
  //
  // These tests verify the fix for vault-mtg-tickets#179: when page 1 is full
  // of non-daemon tags or RC tags, the resolver must paginate to find the
  // matching daemon/v* stable release.
  // ---------------------------------------------------------------------------

  describe('pagination: daemon release beyond page 1', () => {
    /**
     * Build a full page of non-daemon releases (simulates a crowded page 1).
     * Count must equal PER_PAGE (100) so the resolver knows there may be more pages.
     */
    function makeFullPageOfNonDaemonReleases(): ReturnType<typeof makeRelease>[] {
      return Array.from({ length: 100 }, (_, i) => makeRelease(`app/v1.${i}.0`));
    }

    it('paginates to page 2 when page 1 has no daemon/v* releases', async () => {
      // Page 1: 100 non-daemon releases (full page → resolver should request page 2)
      mockFetch.mockResolvedValueOnce(githubResponse(makeFullPageOfNonDaemonReleases()));
      // Page 2: daemon release is present
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.3.2'),
          makeRelease('app/v2.0.0'),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.3.2');
      expect(mockFetch).toHaveBeenCalledTimes(2);

      const [page1Url] = mockFetch.mock.calls[0] as [string, RequestInit];
      const [page2Url] = mockFetch.mock.calls[1] as [string, RequestInit];
      expect(page1Url).toContain('page=1');
      expect(page2Url).toContain('page=2');
    });

    it('resolves the stable release when page 1 is crowded with non-daemon and RC tags (prod env)', async () => {
      vi.stubEnv('VITE_SENTRY_ENV', 'production');

      // Page 1: mix of non-daemon tags and RC daemon tags (full page)
      const page1Releases = [
        ...Array.from({ length: 50 }, (_, i) => makeRelease(`app/v${i}.0.0`)),
        ...Array.from({ length: 49 }, (_, i) =>
          makeRelease(`daemon/v0.4.${i}-rc1`, { prerelease: true })
        ),
        makeRelease('sync/v1.0.0'),
      ];
      mockFetch.mockResolvedValueOnce(githubResponse(page1Releases));

      // Page 2: stable daemon release
      mockFetch.mockResolvedValueOnce(
        githubResponse([makeRelease('daemon/v0.3.2'), makeRelease('app/v0.9.0')])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.3.2');
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    it('finds stable release on page 2 in staging env when page 1 only has RCs', async () => {
      vi.stubEnv('VITE_SENTRY_ENV', 'staging');

      // Page 1: only RC daemon releases — staging should pick these up immediately
      mockFetch.mockResolvedValueOnce(
        githubResponse([
          makeRelease('daemon/v0.4.0-rc1', { prerelease: true }),
        ])
      );

      const result = await fetchLatestDaemonRelease();

      // Staging accepts RC — found on page 1, no page 2 needed
      expect(result).not.toBeNull();
      expect(result!.tag).toBe('daemon/v0.4.0-rc1');
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it('stops paginating when an empty page is returned (no more releases)', async () => {
      // Page 1: full page of non-daemon (resolver will request page 2)
      mockFetch.mockResolvedValueOnce(githubResponse(makeFullPageOfNonDaemonReleases()));
      // Page 2: empty — no more releases
      mockFetch.mockResolvedValueOnce(githubResponse([]));

      const result = await fetchLatestDaemonRelease();

      expect(result).toBeNull();
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    it('stops paginating when a partial page is returned (last page)', async () => {
      // Page 1: full page of non-daemon
      mockFetch.mockResolvedValueOnce(githubResponse(makeFullPageOfNonDaemonReleases()));
      // Page 2: partial page with no daemon release → resolver stops here
      mockFetch.mockResolvedValueOnce(
        githubResponse([makeRelease('app/v5.0.0'), makeRelease('sync/v2.0.0')])
      );

      const result = await fetchLatestDaemonRelease();

      expect(result).toBeNull();
      // Only 2 fetches despite no match — partial page signals end of releases
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    it('respects MAX_PAGES cap and returns null when all pages are exhausted', async () => {
      // Mock MAX_PAGES (5) full pages of non-daemon releases — no daemon tag anywhere
      for (let i = 0; i < 5; i++) {
        mockFetch.mockResolvedValueOnce(githubResponse(makeFullPageOfNonDaemonReleases()));
      }

      const result = await fetchLatestDaemonRelease();

      expect(result).toBeNull();
      // Stopped after exactly MAX_PAGES=5 fetches, not more
      expect(mockFetch).toHaveBeenCalledTimes(5);
    });

    it('returns null when the API returns non-ok on page 2', async () => {
      // Page 1: full page of non-daemon
      mockFetch.mockResolvedValueOnce(githubResponse(makeFullPageOfNonDaemonReleases()));
      // Page 2: API error
      mockFetch.mockResolvedValueOnce({ ok: false, status: 500 });

      const result = await fetchLatestDaemonRelease();

      expect(result).toBeNull();
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });
  });
});
