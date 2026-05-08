/**
 * Smoke tests for clerkMock.tsx — assert every named export exists so a
 * missing export is caught at vitest time, not 30+ minutes into an E2E run.
 *
 * ProtectedRoute imports RedirectToSignIn; the absence of that export caused
 * a Vite optimizer crash in CI (fix: see git history for this file).
 */

import { describe, it, expect } from 'vitest';
import * as clerkMock from './clerkMock';

describe('clerkMock exports', () => {
  const requiredExports: (keyof typeof clerkMock)[] = [
    'ClerkProvider',
    'Show',
    'SignInButton',
    'SignUpButton',
    'UserButton',
    'RedirectToSignIn',
    'useAuth',
    'useUser',
    'useClerk',
    'APIKeys',
    'withAuth',
  ];

  it.each(requiredExports)('exports %s', (name) => {
    expect(clerkMock[name]).toBeDefined();
  });

  it('RedirectToSignIn renders null (returns null from call)', () => {
    const result = clerkMock.RedirectToSignIn();
    expect(result).toBeNull();
  });

  it('useAuth returns isLoaded=true', () => {
    const auth = clerkMock.useAuth();
    expect(auth.isLoaded).toBe(true);
  });

  it('useUser returns isLoaded=true', () => {
    const user = clerkMock.useUser();
    expect(user.isLoaded).toBe(true);
  });

  it('useClerk signOut returns a promise', async () => {
    const clerk = clerkMock.useClerk();
    await expect(clerk.signOut()).resolves.toBeUndefined();
  });
});
