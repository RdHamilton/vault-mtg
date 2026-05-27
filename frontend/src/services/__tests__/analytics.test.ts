import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock posthog-js before importing the analytics module.
vi.mock('posthog-js', () => ({
  default: {
    init: vi.fn(),
    capture: vi.fn(),
    identify: vi.fn(),
    reset: vi.fn(),
    register: vi.fn(),
    startSessionRecording: vi.fn(),
    stopSessionRecording: vi.fn(),
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

  // ── Funnel event taxonomy declarations ───────────────────────────────────────

  it('Events.FUNNEL_SIGN_UP_STARTED is declared in the taxonomy', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const { Events } = await import('../analytics');
    vi.unstubAllEnvs();
    expect(Events.FUNNEL_SIGN_UP_STARTED).toBe('funnel_sign_up_started');
  });

  it('Events.FUNNEL_FIRST_FEATURE_USED is declared in the taxonomy', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const { Events } = await import('../analytics');
    vi.unstubAllEnvs();
    expect(Events.FUNNEL_FIRST_FEATURE_USED).toBe('funnel_first_feature_used');
  });

  it('Events.FUNNEL_DAEMON_PAIRED is declared in the taxonomy (declaration-only — BFF emits)', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const { Events } = await import('../analytics');
    vi.unstubAllEnvs();
    expect(Events.FUNNEL_DAEMON_PAIRED).toBe('funnel_daemon_paired');
  });

  it('trackEvent handles funnel_sign_up_started with entry_point protected_route_redirect', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({
      name: 'funnel_sign_up_started',
      properties: { entry_point: 'protected_route_redirect' },
    });

    expect(posthog.capture).toHaveBeenCalledWith('funnel_sign_up_started', {
      entry_point: 'protected_route_redirect',
    });
    vi.unstubAllEnvs();
  });

  it('NEGATIVE: funnel_sign_up_started payload does not contain user_id', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({
      name: 'funnel_sign_up_started',
      properties: { entry_point: 'protected_route_redirect' },
    });

    const capturedProps = (posthog.capture as ReturnType<typeof vi.fn>).mock.calls[0][1] as Record<string, unknown>;
    expect(capturedProps).not.toHaveProperty('user_id');
    vi.unstubAllEnvs();
  });

  it('trackEvent handles funnel_first_feature_used with feature draft', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({
      name: 'funnel_first_feature_used',
      properties: { feature: 'draft' },
    });

    expect(posthog.capture).toHaveBeenCalledWith('funnel_first_feature_used', {
      feature: 'draft',
    });
    vi.unstubAllEnvs();
  });

  it('NEGATIVE: funnel_first_feature_used payload does not contain user_id', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({
      name: 'funnel_first_feature_used',
      properties: { feature: 'charts' },
    });

    const capturedProps = (posthog.capture as ReturnType<typeof vi.fn>).mock.calls[0][1] as Record<string, unknown>;
    expect(capturedProps).not.toHaveProperty('user_id');
    vi.unstubAllEnvs();
  });

  // ── trackEvent typed API ──────────────────────────────────────────────────

  it('trackEvent calls posthog.capture with correct event name and typed properties', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({ name: 'page_viewed', properties: { page: 'match_history', previous_page: null } });

    expect(posthog.capture).toHaveBeenCalledWith('page_viewed', {
      page: 'match_history',
      previous_page: null,
    });
    vi.unstubAllEnvs();
  });

  it('trackEvent is a no-op when PostHog was not initialized', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({ name: 'page_viewed', properties: { page: 'match_history', previous_page: null } });

    expect(posthog.capture).not.toHaveBeenCalled();
    vi.unstubAllEnvs();
  });

  it('trackEvent handles funnel_daemon_download_started with correct shape', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({
      name: 'funnel_daemon_download_started',
      properties: { os: 'mac', download_source: 'download_page' },
    });

    expect(posthog.capture).toHaveBeenCalledWith('funnel_daemon_download_started', {
      os: 'mac',
      download_source: 'download_page',
    });
    vi.unstubAllEnvs();
  });

  it('trackEvent handles funnel_daemon_connected with optional properties', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({ name: 'funnel_daemon_connected' });

    expect(posthog.capture).toHaveBeenCalledWith('funnel_daemon_connected', undefined);
    vi.unstubAllEnvs();
  });

  it('trackEvent handles funnel_first_data_loaded with match_count', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({ name: 'funnel_first_data_loaded', properties: { match_count: 42 } });

    expect(posthog.capture).toHaveBeenCalledWith('funnel_first_data_loaded', { match_count: 42 });
    vi.unstubAllEnvs();
  });

  it('trackEvent handles error_daemon_never_connected with optional source', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({
      name: 'error_daemon_never_connected',
      properties: { source: 'onboarding_modal' },
    });

    expect(posthog.capture).toHaveBeenCalledWith('error_daemon_never_connected', {
      source: 'onboarding_modal',
    });
    vi.unstubAllEnvs();
  });

  it('trackEvent handles funnel_sign_up_completed with required properties', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({
      name: 'funnel_sign_up_completed',
      properties: { auth_method: 'google', user_id: 'user_xyz' },
    });

    expect(posthog.capture).toHaveBeenCalledWith('funnel_sign_up_completed', {
      auth_method: 'google',
      user_id: 'user_xyz',
    });
    vi.unstubAllEnvs();
  });

  it('trackEvent handles app_user_signed_out with no properties', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, trackEvent } = await import('../analytics');

    initAnalytics();
    trackEvent({ name: 'app_user_signed_out' });

    expect(posthog.capture).toHaveBeenCalledWith('app_user_signed_out', undefined);
    vi.unstubAllEnvs();
  });

  // ── Session Replay ────────────────────────────────────────────────────────

  it('initAnalytics passes session_recording config with maskAllInputs and disable_session_recording', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics } = await import('../analytics');

    initAnalytics();

    expect(posthog.init).toHaveBeenCalledWith(
      'phc_testkey',
      expect.objectContaining({
        disable_session_recording: true,
        session_recording: expect.objectContaining({
          maskAllInputs: true,
        }),
      }),
    );
    vi.unstubAllEnvs();
  });

  it('startSessionReplay calls posthog.startSessionRecording after init', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, startSessionReplay } = await import('../analytics');

    initAnalytics();
    startSessionReplay();

    expect(posthog.startSessionRecording).toHaveBeenCalledOnce();
    vi.unstubAllEnvs();
  });

  it('startSessionReplay is a no-op when PostHog was not initialized', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, startSessionReplay } = await import('../analytics');

    initAnalytics();
    startSessionReplay();

    expect(posthog.startSessionRecording).not.toHaveBeenCalled();
    vi.unstubAllEnvs();
  });

  it('stopSessionReplay calls posthog.stopSessionRecording after init', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', 'phc_testkey');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, stopSessionReplay } = await import('../analytics');

    initAnalytics();
    stopSessionReplay();

    expect(posthog.stopSessionRecording).toHaveBeenCalledOnce();
    vi.unstubAllEnvs();
  });

  it('stopSessionReplay is a no-op when PostHog was not initialized', async () => {
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    const posthog = (await import('posthog-js')).default;
    const { initAnalytics, stopSessionReplay } = await import('../analytics');

    initAnalytics();
    stopSessionReplay();

    expect(posthog.stopSessionRecording).not.toHaveBeenCalled();
    vi.unstubAllEnvs();
  });
});
