/**
 * daemonRelease service tests — post-mortem A7
 *
 * Verifies that fetchLatestDaemonRelease correctly queries the GitHub Releases
 * API at runtime, filters to `daemon/v*` tags, and falls back gracefully when
 * the API is unavailable.
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

  it('queries with per_page=20', async () => {
    mockFetch.mockResolvedValueOnce(githubResponse([makeRelease('daemon/v0.3.1')]));

    await fetchLatestDaemonRelease();

    const [url] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toContain(`${RELEASES_API}?per_page=20`);
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

  it('does NOT skip prerelease releases (prereleases are valid download targets)', async () => {
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
  // FALLBACK_DOWNLOAD_BASE constant
  // ---------------------------------------------------------------------------

  it('FALLBACK_DOWNLOAD_BASE points to /releases/latest/download', () => {
    expect(FALLBACK_DOWNLOAD_BASE).toBe(
      `https://github.com/${GITHUB_REPO}/releases/latest/download`
    );
  });
});
