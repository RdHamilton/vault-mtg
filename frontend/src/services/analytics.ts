/**
 * PostHog analytics module for VaultMTG.
 *
 * Initialization is guarded by VITE_POSTHOG_KEY — if the env var is absent
 * (local dev, test) all calls are no-ops so components need no branching.
 *
 * Event taxonomy: docs/analytics/event-taxonomy.md
 */
import posthog from 'posthog-js';

// ── Initialization ────────────────────────────────────────────────────────────

const POSTHOG_KEY = import.meta.env.VITE_POSTHOG_KEY as string | undefined;
const POSTHOG_HOST =
  (import.meta.env.VITE_POSTHOG_HOST as string | undefined) ??
  'https://app.posthog.com';

let initialized = false;

export function initAnalytics(): void {
  if (!POSTHOG_KEY) return;
  posthog.init(POSTHOG_KEY, {
    api_host: POSTHOG_HOST,
    // We fire page_viewed manually so we can attach properties.
    capture_pageview: false,
    // Disable autocapture to keep event taxonomy clean.
    autocapture: false,
  });
  initialized = true;
}

// ── Event name constants (locked taxonomy) ────────────────────────────────────

export const Events = {
  // Activation funnel
  FUNNEL_LANDING_PAGE_VIEWED: 'funnel_landing_page_viewed',
  FUNNEL_SIGN_UP_STARTED: 'funnel_sign_up_started',
  FUNNEL_SIGN_UP_COMPLETED: 'funnel_sign_up_completed',
  FUNNEL_DAEMON_DOWNLOAD_STARTED: 'funnel_daemon_download_started',
  FUNNEL_DAEMON_CONNECTED: 'funnel_daemon_connected',
  FUNNEL_FIRST_DATA_LOADED: 'funnel_first_data_loaded',
  FUNNEL_FIRST_FEATURE_USED: 'funnel_first_feature_used',

  // Page views
  PAGE_VIEWED: 'page_viewed',

  // Feature usage
  FEATURE_MATCH_HISTORY_FILTERED: 'feature_match_history_filtered',
  FEATURE_MATCH_DETAILS_OPENED: 'feature_match_details_opened',
  FEATURE_DRAFT_ADVISOR_PICK_VIEWED: 'feature_draft_advisor_pick_viewed',
  FEATURE_DRAFT_ANALYTICS_VIEWED: 'feature_draft_analytics_viewed',
  FEATURE_DECK_BUILDER_OPENED: 'feature_deck_builder_opened',
  FEATURE_DECK_BUILD_AROUND_STARTED: 'feature_deck_build_around_started',
  FEATURE_COLLECTION_VIEWED: 'feature_collection_viewed',
  FEATURE_META_VIEWED: 'feature_meta_viewed',
  FEATURE_ML_SUGGESTIONS_VIEWED: 'feature_ml_suggestions_viewed',
  FEATURE_CHART_INTERACTED: 'feature_chart_interacted',
  FEATURE_OPPONENT_ANALYSIS_VIEWED: 'feature_opponent_analysis_viewed',
  FEATURE_COMMUNITY_COMPARISON_VIEWED: 'feature_community_comparison_viewed',
  FEATURE_SETTINGS_CHANGED: 'feature_settings_changed',
  FEATURE_REPLAY_STARTED: 'feature_replay_started',
  FEATURE_REPLAY_COMPLETED: 'feature_replay_completed',

  // Errors
  ERROR_DAEMON_CONNECTION_FAILED: 'error_daemon_connection_failed',
  ERROR_DAEMON_NEVER_CONNECTED: 'error_daemon_never_connected',
  ERROR_DATA_LOAD_FAILED: 'error_data_load_failed',
  ERROR_AUTH_FAILED: 'error_auth_failed',
  ERROR_EMPTY_STATE_SHOWN: 'error_empty_state_shown',

  // Engagement
  APP_SESSION_STARTED: 'app_session_started',
  APP_USER_IDENTIFIED: 'app_user_identified',
  APP_USER_SIGNED_OUT: 'app_user_signed_out',
} as const;

export type EventName = (typeof Events)[keyof typeof Events];

// ── Core helpers ──────────────────────────────────────────────────────────────

/**
 * Capture a PostHog event. No-op when PostHog is not initialized.
 * Never include PII — use opaque Clerk user_id only.
 */
export function captureEvent(
  name: EventName,
  properties?: Record<string, unknown>,
): void {
  if (!initialized) return;
  posthog.capture(name, properties);
}

/**
 * Identify the current user by their opaque Clerk user ID.
 * Must only be called once per session after Clerk confirms isSignedIn.
 */
export function identifyUser(userId: string): void {
  if (!initialized) return;
  posthog.identify(userId);
}

/**
 * Reset PostHog identity on sign-out.
 */
export function resetIdentity(): void {
  if (!initialized) return;
  posthog.reset();
}

/**
 * Register super-properties sent on every subsequent event.
 */
export function registerSuperProperties(
  properties: Record<string, unknown>,
): void {
  if (!initialized) return;
  posthog.register(properties);
}
