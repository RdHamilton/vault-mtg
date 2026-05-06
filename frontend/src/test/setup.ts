import '@testing-library/jest-dom';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import { mockWailsRuntime, mockEventEmitter } from './mocks/websocketMock';
import { mockApi, resetMocks } from './mocks/apiMock';

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
  useAuth: () => ({ isLoaded: true, isSignedIn: true, getToken: () => Promise.resolve('clerk-test-token-stub') }),
  useUser: () => ({ isLoaded: true, isSignedIn: true, user: { id: 'user_test_123', emailAddresses: [{ emailAddress: 'test@example.com' }] } }),
}));

// Mock WebSocket client globally
vi.mock('@/services/websocketClient', () => mockWailsRuntime);

// Mock the REST API modules globally
vi.mock('@/services/api', () => mockApi);

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
afterEach(() => {
  cleanup();
  mockEventEmitter.clear();
  resetMocks();
});

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

// Mock IntersectionObserver
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
