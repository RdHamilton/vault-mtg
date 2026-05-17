/**
 * Clerk mock for E2E (Playwright) test mode.
 *
 * Loaded via Vite alias when VITE_CLERK_TEST_MODE=true. Reads auth state from
 * window.__CLERK_TEST_STATE__ so Playwright tests can control signed-in/out
 * state via page.addInitScript() without needing a real Clerk publishable key.
 *
 * Default state (no window.__CLERK_TEST_STATE__): signed-out.
 */

import React from 'react';

type ClerkTestState = {
  isSignedIn?: boolean;
  userId?: string;
  firstName?: string;
  lastName?: string;
  email?: string;
};

function getTestState(): ClerkTestState {
  if (typeof window !== 'undefined' && (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__) {
    return (window as unknown as Record<string, unknown>).__CLERK_TEST_STATE__ as ClerkTestState;
  }
  return { isSignedIn: false };
}

// ClerkProvider — just renders children; no real Clerk context needed
export const ClerkProvider = ({ children }: { children: React.ReactNode }) => {
  return React.createElement(React.Fragment, null, children);
};

// Show — conditionally renders based on test state
export const Show = ({ when, children }: { when: 'signed-in' | 'signed-out'; children: React.ReactNode }) => {
  const { isSignedIn } = getTestState();
  if (when === 'signed-in' && isSignedIn) return React.createElement(React.Fragment, null, children);
  if (when === 'signed-out' && !isSignedIn) return React.createElement(React.Fragment, null, children);
  return null;
};

// SignInButton — renders children as-is (just a wrapper)
export const SignInButton = ({ children }: { children: React.ReactNode }) => {
  return React.createElement(React.Fragment, null, children);
};

// SignUpButton — renders children as-is (just a wrapper)
export const SignUpButton = ({ children }: { children: React.ReactNode }) => {
  return React.createElement(React.Fragment, null, children);
};

// UserButton — renders a placeholder avatar when signed in
export const UserButton = ({ afterSignOutUrl: _afterSignOutUrl }: { afterSignOutUrl?: string } = {}) => {
  const { isSignedIn } = getTestState();
  if (!isSignedIn) return null;
  return React.createElement(
    'button',
    { 'data-testid': 'clerk-user-button', className: 'cl-userButton', 'aria-label': 'User menu' },
    React.createElement('span', { className: 'cl-userButtonAvatarBox' })
  );
};

// useAuth — returns auth state based on test state.
// getToken() returns a deterministic test JWT so components that call
// useAuth().getToken() receive a non-null value in test mode.
export const useAuth = () => {
  const { isSignedIn } = getTestState();
  return {
    isLoaded: true,
    isSignedIn: isSignedIn ?? false,
    userId: isSignedIn ? 'user_test_123' : null,
    getToken: () => Promise.resolve(isSignedIn ? 'clerk-test-token-stub' : null),
  };
};

// useUser — returns user info based on test state.
//
// The user object exposes update() and setProfileImage() stubs that resolve
// immediately so Profile.tsx's name-edit / avatar-edit save flows run through
// their happy path in E2E tests without a real Clerk session (#2178).
export const useUser = () => {
  const state = getTestState();
  if (!state.isSignedIn) return { isLoaded: true, isSignedIn: false, user: null };
  const email = state.email ?? 'test@example.com';
  return {
    isLoaded: true,
    isSignedIn: true,
    user: {
      id: 'user_test_123',
      firstName: state.firstName ?? 'Test',
      lastName: state.lastName ?? 'User',
      fullName: `${state.firstName ?? 'Test'} ${state.lastName ?? 'User'}`,
      imageUrl: '',
      primaryEmailAddress: { emailAddress: email },
      update: (_params: { firstName?: string; lastName?: string }) => Promise.resolve(),
      setProfileImage: (_params: { file: File | null }) => Promise.resolve(),
    },
  };
};

// useClerk — minimal stub
export const useClerk = () => ({
  signOut: () => Promise.resolve(),
  openSignIn: () => {},
  openSignUp: () => {},
});

// APIKeys — renders a test stub that represents the Clerk API Keys UI.
// In E2E tests, Playwright can interact with these data-testid elements.
export const APIKeys = (_props?: Record<string, unknown>) => {
  return React.createElement(
    'div',
    { 'data-testid': 'clerk-api-keys-component', className: 'cl-apiKeys' },
    React.createElement(
      'button',
      { 'data-testid': 'clerk-create-api-key-btn', className: 'cl-button' },
      'Create API key'
    ),
    React.createElement(
      'ul',
      { 'data-testid': 'clerk-api-key-list', className: 'cl-apiKeysList' }
    )
  );
};

// RedirectToSignIn — renders nothing in test mode; ProtectedRoute uses this
// to redirect unauthenticated users. The mock prevents a Vite optimizer crash
// when @clerk/react is aliased to this file via VITE_CLERK_TEST_MODE=true.
export const RedirectToSignIn = () => null;

// withAuth HOC stub
export const withAuth = (Component: React.ComponentType) => Component;
