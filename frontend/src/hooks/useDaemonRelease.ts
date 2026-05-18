/**
 * useDaemonRelease — resolves the daemon download base URL at runtime.
 *
 * On mount, queries the GitHub Releases API for the latest `daemon/v*` tag and
 * returns the corresponding download base.  While the fetch is in flight (or if
 * it fails) the hook falls back to GitHub's `/releases/latest/download` redirect
 * so the buttons are never broken.
 *
 * State shape:
 *   downloadBase  — URL prefix to which "/artifact.ext" is appended
 *   loading       — true while the API call is in flight
 *   error         — set when the fetch failed (non-null while fallback is active)
 */

import { useEffect, useState } from 'react';
import {
  fetchLatestDaemonRelease,
  FALLBACK_DOWNLOAD_BASE,
} from '@/services/daemonRelease';

export interface DaemonReleaseState {
  downloadBase: string;
  loading: boolean;
  error: string | null;
}

export function useDaemonRelease(): DaemonReleaseState {
  const [state, setState] = useState<DaemonReleaseState>({
    downloadBase: FALLBACK_DOWNLOAD_BASE,
    loading: true,
    error: null,
  });

  useEffect(() => {
    const controller = new AbortController();

    fetchLatestDaemonRelease(controller.signal).then((info) => {
      if (controller.signal.aborted) return;

      if (info) {
        setState({ downloadBase: info.downloadBase, loading: false, error: null });
      } else {
        setState({
          downloadBase: FALLBACK_DOWNLOAD_BASE,
          loading: false,
          error: 'Could not resolve latest daemon release — using latest stable redirect',
        });
      }
    });

    return () => {
      controller.abort();
    };
  }, []);

  return state;
}
