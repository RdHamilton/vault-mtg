import { useEffect } from 'react';
import { useAuth } from '@clerk/react';
import { initializeSse, disconnectSse } from '../services/adapter';

/**
 * Defers the SSE connection until Clerk reports isLoaded && isSignedIn.
 *
 * Mounting this component before Clerk is ready causes the first SSE request
 * to go out without an Authorization header, triggering a 401 on every cold
 * load (issue #1922). By gating on isLoaded && isSignedIn we guarantee the
 * Clerk token provider is already registered before the stream opens.
 */
export function SseInitializer() {
  const { isLoaded, isSignedIn } = useAuth();

  useEffect(() => {
    if (!isLoaded || !isSignedIn) return;

    let active = true;
    initializeSse().catch((err) => {
      if (active) {
        console.error('[SseInitializer] SSE connection failed:', err);
      }
    });

    return () => {
      active = false;
      disconnectSse();
    };
  }, [isLoaded, isSignedIn]);

  return null;
}
