import type { Decorator } from '@storybook/react';
import { MemoryRouter } from 'react-router-dom';
import { clerkMockState } from './clerk-mock';

/**
 * Storybook decorators that provide the application context a component needs
 * to render in isolation — without booting the full app, its router, or a live
 * Clerk session.
 *
 * Component-Driven Development principle: a story develops a component free of
 * application business logic. These decorators supply *just enough* mock
 * context for components that depend on the router or Clerk auth, so the
 * component under test renders correctly on its own.
 *
 * Usage in a story file:
 *
 *   import { withRouter, withClerkMock } from '../../.storybook/decorators'
 *
 *   const meta: Meta<typeof MyComponent> = {
 *     title: 'Organisms/MyComponent',
 *     component: MyComponent,
 *     decorators: [withRouter],          // component renders <Link> / uses router hooks
 *   }
 *
 * Most atoms and molecules need neither decorator and should not opt into them.
 */

/**
 * withRouter — wraps a story in a MemoryRouter so components that use
 * `<Link>`, `<Outlet />`, `useNavigate()`, or `useLocation()` render without
 * a real browser history. MemoryRouter keeps history in memory, which is
 * exactly what an isolated story needs.
 *
 * Apply this decorator only to stories whose component touches the router.
 */
export const withRouter: Decorator = (Story) => (
  <MemoryRouter>
    <Story />
  </MemoryRouter>
);

/**
 * withRouterAt — factory variant of `withRouter` that seeds the router with a
 * specific initial path. Use when a component's rendering depends on the
 * current route (e.g. ProtectedRoute deriving a page name from the pathname).
 *
 *   decorators: [withRouterAt('/collection')]
 */
export const withRouterAt =
  (initialPath: string): Decorator =>
  (Story) => (
    <MemoryRouter initialEntries={[initialPath]}>
      <Story />
    </MemoryRouter>
  );

/**
 * withClerkSession — global decorator (registered in `preview.ts`) that syncs
 * the Clerk mock's session state from the per-story `clerk` parameter before
 * each story renders.
 *
 * `@clerk/react` is aliased to `.storybook/clerk-mock` for all stories, so the
 * mock is always active. By default the mock reports a signed-in user; a story
 * can flip to signed-out via:
 *
 *   parameters: { clerk: { signedIn: false } }
 *
 * Atoms and molecules that never touch Clerk are unaffected — this decorator
 * is a no-op for them.
 */
export const withClerkSession: Decorator = (Story, context) => {
  const clerkParam = context.parameters.clerk as { signedIn?: boolean } | undefined;
  clerkMockState.signedIn = clerkParam?.signedIn ?? true;
  return <Story />;
};
