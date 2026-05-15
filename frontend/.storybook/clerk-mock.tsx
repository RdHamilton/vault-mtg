/**
 * Clerk mock for Storybook.
 *
 * Storybook aliases `@clerk/react` to this module (see `.storybook/main.ts`
 * `viteFinal`). This keeps every story fully offline and deterministic:
 * components that call `useUser()`, `useAuth()`, or render `<UserButton />`
 * never reach Clerk's network or require a real publishable key.
 *
 * Component-Driven Development principle: a component is developed in
 * isolation. Auth is application context, not component behaviour — so we
 * supply a fixed, signed-in mock session and stub the visual primitives.
 *
 * Default session: a signed-in user named "Planeswalker". To exercise a
 * signed-out state in a specific story, use the `clerk` story parameter:
 *
 *   export const SignedOut: Story = {
 *     parameters: { clerk: { signedIn: false } },
 *   }
 *
 * The `withClerkSession` decorator in `.storybook/decorators.tsx` reads that
 * parameter. When no parameter is set, the signed-in default applies.
 *
 * This mock intentionally implements only the surface the VaultMTG SPA uses.
 * If a component starts using a new Clerk export, add it here.
 */
import type { ReactNode } from 'react';

/** Mutable session state, toggled per-story by the `withClerkSession` decorator. */
export const clerkMockState = {
  signedIn: true,
};

const MOCK_USER = {
  id: 'user_storybook_mock',
  firstName: 'Planeswalker',
  lastName: 'Mock',
  fullName: 'Planeswalker Mock',
  primaryEmailAddress: { emailAddress: 'planeswalker@vaultmtg.test' },
  imageUrl: '',
};

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

export function useAuth() {
  return {
    isLoaded: true,
    isSignedIn: clerkMockState.signedIn,
    userId: clerkMockState.signedIn ? MOCK_USER.id : null,
    sessionId: clerkMockState.signedIn ? 'sess_storybook_mock' : null,
    getToken: async () => (clerkMockState.signedIn ? 'storybook-mock-token' : null),
    signOut: async () => {},
  };
}

export function useUser() {
  return {
    isLoaded: true,
    isSignedIn: clerkMockState.signedIn,
    user: clerkMockState.signedIn ? MOCK_USER : null,
  };
}

export function useSession() {
  return {
    isLoaded: true,
    isSignedIn: clerkMockState.signedIn,
    session: clerkMockState.signedIn ? { id: 'sess_storybook_mock' } : null,
  };
}

export function useClerk() {
  return {
    signOut: async () => {},
    openSignIn: () => {},
    openSignUp: () => {},
  };
}

// ---------------------------------------------------------------------------
// Components
// ---------------------------------------------------------------------------

export function ClerkProvider({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

export function ClerkLoaded({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

/** `<SignedIn>` / `<SignedOut>` render their children based on mock state. */
export function SignedIn({ children }: { children: ReactNode }) {
  return clerkMockState.signedIn ? <>{children}</> : null;
}

export function SignedOut({ children }: { children: ReactNode }) {
  return clerkMockState.signedIn ? null : <>{children}</>;
}

/** `<Show>` is the Clerk v6 conditional-render primitive used by AuthBar. */
export function Show({ children }: { children: ReactNode; when?: unknown }) {
  return <>{children}</>;
}

export function SignInButton({ children }: { children?: ReactNode }) {
  return <>{children ?? <button type="button">Sign in</button>}</>;
}

export function SignUpButton({ children }: { children?: ReactNode }) {
  return <>{children ?? <button type="button">Sign up</button>}</>;
}

export function RedirectToSignIn() {
  return null;
}

/** Stubbed `<UserButton />` — a static avatar so organism stories render cleanly. */
export function UserButton() {
  return (
    <div
      aria-label="User menu (Storybook mock)"
      style={{
        width: 32,
        height: 32,
        borderRadius: '50%',
        background: '#6c5ce7',
        color: '#fff',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: 14,
        fontWeight: 600,
      }}
    >
      P
    </div>
  );
}
