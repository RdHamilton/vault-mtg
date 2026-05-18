import { afterEach, vi } from 'vitest';
import { mockWailsRuntime, mockEventEmitter } from './mocks/websocketMock';
import { mockApi, resetMocks } from './mocks/apiMock';

// In DOM environments (jsdom) load jest-dom matchers and React testing utils.
// In node environments (integration tests) these are not available.
if (typeof document !== 'undefined') {
  // Dynamic imports avoid import errors when running in node environment
  await import('@testing-library/jest-dom');
}

// Mock @clerk/react globally so components that use Clerk work in tests
// without a real ClerkProvider or publishable key.
// Default behaviour: signed-in so route tests reach protected pages.
// Override per-test with vi.mocked(@clerk/react).useAuth for signed-out scenarios.
vi.mock('@clerk/react', () => ({
  ClerkProvider: ({ children }: { children: unknown }) => children,
  Show: ({ when, children }: { when: string; children: unknown }) =>
    when === 'signed-in' ? children : null,
  SignInButton: ({ children }: { children: unknown }) => children,
  SignUpButton: ({ children }: { children: unknown }) => children,
  UserButton: () => null,
  RedirectToSignIn: () => null,
  useAuth: () => ({ isLoaded: true, isSignedIn: true, getToken: () => Promise.resolve('clerk-test-token-stub') }),
  useUser: () => ({ isLoaded: true, isSignedIn: true, user: { id: 'user_test_123', emailAddresses: [{ emailAddress: 'test@example.com' }] } }),
}));

// Mock WebSocket client globally
vi.mock('@/services/websocketClient', () => mockWailsRuntime);

// Mock the REST API modules globally
vi.mock('@/services/api', () => mockApi);

// Mock bffHealth globally — returns connected by default so pages render normally.
// Tests that need daemon-disconnected behaviour can override this mock per-test.
vi.mock('@/services/api/bffHealth', () => ({
  getDaemonHealth: vi.fn(() => Promise.resolve({ status: 'connected' })),
}));

// Mock useDaemonStatus globally — returns connected+checked by default so page
// tests that predate Wave 5 do not need to be updated to account for the hook.
// Tests that need disconnected state can override: vi.mocked(useDaemonStatus).mockReturnValue(...)
vi.mock('@/hooks/useDaemonStatus', () => ({
  useDaemonStatus: vi.fn(() => ({ daemonConnected: true, daemonChecked: true })),
}));

// Mock useDaemonRelease globally — returns the fallback/latest download base by
// default so component tests that predate A7 do not need to mock this hook.
// Tests that need runtime-resolved URLs can override per-test:
//   vi.mocked(useDaemonRelease).mockReturnValue({ downloadBase: '...', loading: false, error: null })
vi.mock('@/hooks/useDaemonRelease', () => ({
  useDaemonRelease: vi.fn(() => ({
    downloadBase: 'https://github.com/RdHamilton/vault-mtg/releases/latest/download',
    loading: false,
    error: null,
  })),
}));

// Mock individual API modules that are imported directly
vi.mock('@/services/api/standard', () => ({
  validateDeckStandard: vi.fn(() => Promise.resolve({ isLegal: true, errors: [], warnings: [], setBreakdown: [] })),
  getStandardSets: vi.fn(() => Promise.resolve([])),
  getUpcomingRotation: vi.fn(() => Promise.resolve({})),
  getRotationAffectedDecks: vi.fn(() => Promise.resolve([])),
  getStandardConfig: vi.fn(() => Promise.resolve({})),
  getCardLegality: vi.fn(() => Promise.resolve({})),
}));

// Cleanup after each test
afterEach(async () => {
  // cleanup() is only available in DOM environments
  if (typeof document !== 'undefined') {
    const { cleanup } = await import('@testing-library/react');
    cleanup();
  }
  mockEventEmitter.clear();
  resetMocks();
});

// DOM-specific setup: only runs in jsdom environment
if (typeof window !== 'undefined') {
  // Mock window.matchMedia
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {}, // deprecated
      removeListener: () => {}, // deprecated
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => true,
    }),
  });
}

// Mock IntersectionObserver
if (typeof global !== 'undefined') {
  global.IntersectionObserver = class IntersectionObserver {
    constructor() {}
    disconnect() {}
    observe() {}
    takeRecords() {
      return [];
    }
    unobserve() {}
  } as unknown as typeof IntersectionObserver;

  // Mock ResizeObserver
  global.ResizeObserver = class ResizeObserver {
    constructor() {}
    disconnect() {}
    observe() {}
    unobserve() {}
  } as unknown as typeof ResizeObserver;
}
