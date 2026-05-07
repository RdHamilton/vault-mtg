import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock posthog-js before importing the analytics module.
vi.mock('posthog-js', () => ({
  default: {
    init: vi.fn(),
    capture: vi.fn(),
    identify: vi.fn(),
    reset: vi.fn(),
    register: vi.fn(),
  },
}));

// Reset module registry between tests so initAnalytics state is fresh.
describe('analytics', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  it('skips posthog.init when VITE_POSTHOG_KEY is absent', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics } = await import('../analytics');

    initAnalytics();

    expect(posthog.init).not.toHaveBeenCalled();
    vi.unstubAllEnvs();
  });

  it('calls posthog.init with key and host when VITE_POSTHOG_KEY is present', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    vi.stubEnv('VITE_POSTHOG_HOST', 'https://eu.posthog.com');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics } = await import('../analytics');

    initAnalytics();

    expect(posthog.init).toHaveBeenCalledWith(
      'phc_testkey',
      expect.objectContaining({
        api_host: 'https://eu.posthog.com',
        capture_pageview: false,
      }),
    );
    vi.unstubAllEnvs();
  });

  it('captureEvent calls posthog.capture with event name and properties after init', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, captureEvent, Events } = await import(
      '../analytics'
    );

    initAnalytics();
    captureEvent(Events.PAGE_VIEWED, { page: 'match_history' });

    expect(posthog.capture).toHaveBeenCalledWith('page_viewed', {
      page: 'match_history',
    });
    vi.unstubAllEnvs();
  });

  it('captureEvent is a no-op when PostHog was not initialized', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, captureEvent, Events } = await import(
      '../analytics'
    );

    initAnalytics(); // key absent → no init
    captureEvent(Events.PAGE_VIEWED, { page: 'match_history' });

    expect(posthog.capture).not.toHaveBeenCalled();
    vi.unstubAllEnvs();
  });

  it('identifyUser calls posthog.identify with the given user id after init', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, identifyUser } = await import('../analytics');

    initAnalytics();
    identifyUser('user_abc123');

    expect(posthog.identify).toHaveBeenCalledWith('user_abc123');
    vi.unstubAllEnvs();
  });

  it('resetIdentity calls posthog.reset after init', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, resetIdentity } = await import('../analytics');

    initAnalytics();
    resetIdentity();

    expect(posthog.reset).toHaveBeenCalledOnce();
    vi.unstubAllEnvs();
  });

  it('registerSuperProperties calls posthog.register after init', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, registerSuperProperties } = await import(
      '../analytics'
    );

    initAnalytics();
    registerSuperProperties({ app_version: '1.0.0', is_signed_in: true });

    expect(posthog.register).toHaveBeenCalledWith({
      app_version: '1.0.0',
      is_signed_in: true,
    });
    vi.unstubAllEnvs();
  });

  it('Events object contains all taxonomy event names', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const { Events } = await import('../analytics');
    vi.unstubAllEnvs();

    expect(Events.FUNNEL_SIGN_UP_COMPLETED).toBe('funnel_sign_up_completed');
    expect(Events.PAGE_VIEWED).toBe('page_viewed');
    expect(Events.APP_USER_IDENTIFIED).toBe('app_user_identified');
    expect(Events.ERROR_AUTH_FAILED).toBe('error_auth_failed');
    expect(Events.APP_USER_SIGNED_OUT).toBe('app_user_signed_out');
  });
});
