/**
 * Tests for PostHogRouteTracker — router-level page_viewed tracking.
 *
 * Covers:
 * - Fires page_viewed on route change (after initial mount).
 * - Skips firing on initial mount (first render).
 * - Normalizes dynamic route segments to slugs (e.g. /deck-builder/:id → deck_builder).
 * - No user_id is ever included in any page_viewed payload.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router-dom';
import { act } from 'react';
import { PostHogRouteTracker } from './PostHogRouteTracker';

// ── Analytics mock ────────────────────────────────────────────────────────────

const mockTrackEvent = vi.fn();

vi.mock('@/services/analytics', () => ({
  trackEvent: (...args: unknown[]) => mockTrackEvent(...args),
  identifyUser: vi.fn(),
  resetIdentity: vi.fn(),
  startSessionReplay: vi.fn(),
  stopSessionReplay: vi.fn(),
  registerSuperProperties: vi.fn(),
  initAnalytics: vi.fn(),
}));

// ── Helpers ───────────────────────────────────────────────────────────────────

/**
 * Renders PostHogRouteTracker inside a MemoryRouter starting at `initialPath`.
 * Returns a `navigate` handle so tests can drive route changes.
 */
function renderTracker(initialPath = '/home') {
  let navRef: ReturnType<typeof useNavigate> | null = null;

  function NavCapture() {
    navRef = useNavigate();
    return null;
  }

  render(
    <MemoryRouter initialEntries={[initialPath]}>
      <NavCapture />
      <PostHogRouteTracker />
      <Routes>
        <Route path="*" element={null} />
      </Routes>
    </MemoryRouter>,
  );

  return {
    navigate: (path: string) =>
      act(() => {
        navRef!(path);
      }),
  };
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('PostHogRouteTracker', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does NOT fire page_viewed on the initial mount (skip-first rule)', () => {
    renderTracker('/home');
    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'page_viewed',
    );
    expect(calls).toHaveLength(0);
  });

  it('fires page_viewed when the route changes away from the initial path', async () => {
    const { navigate } = renderTracker('/home');
    await navigate('/match-history');
    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'page_viewed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0]).toEqual({
      name: 'page_viewed',
      properties: {
        page: 'match_history',
        previous_page: 'home',
      },
    });
  });

  it('fires page_viewed again on a second route change', async () => {
    const { navigate } = renderTracker('/home');
    await navigate('/draft');
    await navigate('/decks');
    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'page_viewed',
    );
    expect(calls).toHaveLength(2);
    expect(calls[1][0].properties.previous_page).toBe('draft_advisor');
  });

  it('normalizes /deck-builder/:id to "deck_builder"', async () => {
    const { navigate } = renderTracker('/home');
    await navigate('/deck-builder/abc-123');
    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'page_viewed',
    );
    expect(calls).toHaveLength(1);
    expect(calls[0][0].properties.page).toBe('deck_builder');
  });

  it('normalizes /charts/win-rate-trend to "chart_win_rate"', async () => {
    const { navigate } = renderTracker('/home');
    await navigate('/charts/win-rate-trend');
    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'page_viewed',
    );
    expect(calls[0][0].properties.page).toBe('chart_win_rate');
  });

  it('normalizes /charts/deck-performance to "chart_deck_performance"', async () => {
    const { navigate } = renderTracker('/home');
    await navigate('/charts/deck-performance');
    const calls = mockTrackEvent.mock.calls.filter(
      ([e]: [{ name: string }]) => e.name === 'page_viewed',
    );
    expect(calls[0][0].properties.page).toBe('chart_deck_performance');
  });

  it('NEGATIVE: page_viewed payload never contains user_id', async () => {
    const { navigate } = renderTracker('/home');
    await navigate('/match-history');
    mockTrackEvent.mock.calls
      .filter(([e]: [{ name: string }]) => e.name === 'page_viewed')
      .forEach(([e]: [{ properties: Record<string, unknown> }]) => {
        expect(e.properties).not.toHaveProperty('user_id');
      });
  });

  it('returns null (renders nothing)', () => {
    const { container } = render(
      <MemoryRouter>
        <PostHogRouteTracker />
      </MemoryRouter>,
    );
    expect(container.firstChild).toBeNull();
  });
});
