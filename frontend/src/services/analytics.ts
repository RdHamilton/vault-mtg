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
    // Session replay: disabled by default; enabled per-user after auth.
    // maskAllInputs prevents any typed text from appearing in replays.
    // maskAllText: false — we use .ph-no-capture class for selective masking.
    // disable_session_recording: true until startSessionReplay() is called.
    session_recording: {
      maskAllInputs: true,
      maskTextSelector: '.sensitive, .ph-no-capture',
    },
    disable_session_recording: true,
  });
  initialized = true;
}

/**
 * Start PostHog session replay for the current user.
 * Must only be called once Clerk has confirmed isSignedIn — never for
 * unauthenticated sessions.
 */
export function startSessionReplay(): void {
  if (!initialized) return;
  posthog.startSessionRecording();
}

/**
 * Stop PostHog session replay (e.g. on sign-out).
 */
export function stopSessionReplay(): void {
  if (!initialized) return;
  posthog.stopSessionRecording();
}

// ── Event name constants (locked taxonomy) ────────────────────────────────────

export const Events = {
  // Activation funnel
  FUNNEL_LANDING_PAGE_VIEWED: 'funnel_landing_page_viewed',
  FUNNEL_SIGN_UP_STARTED: 'funnel_sign_up_started',
  FUNNEL_SIGN_UP_COMPLETED: 'funnel_sign_up_completed',
  FUNNEL_DAEMON_DOWNLOAD_STARTED: 'funnel_daemon_download_started',
  FUNNEL_DAEMON_CONNECTED: 'funnel_daemon_connected',
  FUNNEL_DAEMON_INSTALLED: 'funnel_daemon_installed',
  FUNNEL_FIRST_GAME_PLAYED: 'funnel_first_game_played',
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

// ── Typed property shapes per event ──────────────────────────────────────────
//
// Every entry in the discriminated union covers one event from the taxonomy.
// Adding a new event requires adding a new branch here — the compiler will
// enforce completeness at every trackEvent call site.

export type AnalyticsEvent =
  // Activation funnel
  | {
      name: 'funnel_landing_page_viewed';
      properties: {
        referrer: string;
        utm_source: string;
        utm_medium: string;
        utm_campaign: string;
      };
    }
  | {
      name: 'funnel_sign_up_started';
      properties: {
        entry_point: 'landing_page' | 'auth_bar' | 'protected_route_redirect';
      };
    }
  | {
      name: 'funnel_sign_up_completed';
      properties: {
        auth_method: 'email' | 'google' | 'apple' | 'facebook';
        user_id: string;
      };
    }
  | {
      name: 'funnel_daemon_download_started';
      properties: {
        os: string;
        download_source: 'download_page' | 'prompt_modal' | 'onboarding_modal';
      };
    }
  | {
      name: 'funnel_daemon_connected';
      properties?: {
        time_since_signup_seconds?: number;
        source?: string;
      };
    }
  | {
      name: 'funnel_daemon_installed';
      properties?: {
        /** Daemon version string if known */
        daemon_version?: string;
        /** Source page where the event fired */
        source?: string;
      };
    }
  | {
      name: 'funnel_first_game_played';
      properties?: {
        /** Format of the first game (e.g. "Standard", "Limited") */
        format?: string;
        /** Source page where the event fired */
        source?: string;
      };
    }
  | {
      name: 'funnel_first_data_loaded';
      properties: { match_count: number };
    }
  | {
      name: 'funnel_first_feature_used';
      properties: {
        feature:
          | 'draft'
          | 'draft_analytics'
          | 'decks'
          | 'collection'
          | 'meta'
          | 'charts'
          | 'quests';
      };
    }
  // Page views
  | {
      name: 'page_viewed';
      properties: { page: string; previous_page: string | null };
    }
  // Feature usage
  | {
      name: 'feature_match_history_filtered';
      properties: {
        filter_type: 'format' | 'deck' | 'date_range' | 'result';
        filter_value: string;
      };
    }
  | {
      name: 'feature_match_details_opened';
      properties: {
        match_result: 'win' | 'loss' | 'draw';
        format: string;
      };
    }
  | {
      name: 'feature_draft_advisor_pick_viewed';
      properties: {
        set_code: string;
        pack_number: number;
        pick_number: number;
      };
    }
  | {
      name: 'feature_draft_analytics_viewed';
      properties: { draft_count: number };
    }
  | {
      name: 'feature_deck_builder_opened';
      properties: {
        entry_point: 'decks_list' | 'draft_build_around' | 'direct_link';
      };
    }
  | {
      name: 'feature_deck_build_around_started';
      properties: { seed_type: 'card' | 'archetype' | 'color_pair' };
    }
  | {
      name: 'feature_collection_viewed';
      properties: { card_count: number };
    }
  | { name: 'feature_meta_viewed'; properties?: Record<string, never> }
  | {
      name: 'feature_ml_suggestions_viewed';
      properties: {
        suggestion_count: number;
        context: 'deck_builder' | 'draft';
      };
    }
  | {
      name: 'feature_chart_interacted';
      properties: {
        chart: string;
        interaction: 'filter_applied' | 'time_range_changed' | 'format_changed';
      };
    }
  | {
      name: 'feature_opponent_analysis_viewed';
      properties: { opponent_match_count: number };
    }
  | {
      name: 'feature_community_comparison_viewed';
      properties?: Record<string, never>;
    }
  | {
      name: 'feature_settings_changed';
      properties: {
        setting_section: 'daemon_connection' | 'preferences' | 'display';
        setting_key: string;
      };
    }
  | { name: 'feature_replay_started'; properties?: Record<string, never> }
  | { name: 'feature_replay_completed'; properties?: Record<string, never> }
  // Errors
  | {
      name: 'error_daemon_connection_failed';
      properties: {
        previous_status: 'connected' | 'reconnecting';
        duration_connected_seconds: number;
      };
    }
  | {
      name: 'error_daemon_never_connected';
      properties: {
        time_since_signin_seconds?: number;
        source?: string;
      };
    }
  | {
      name: 'error_data_load_failed';
      properties: { page: string; endpoint: string; status_code: number };
    }
  | {
      name: 'error_auth_failed';
      properties: { context: string };
    }
  | {
      name: 'error_empty_state_shown';
      properties: { page: string };
    }
  // Engagement
  | {
      name: 'app_session_started';
      properties: { services_init_ms: number };
    }
  | {
      name: 'app_user_identified';
      properties: {
        user_id: string;
        auth_method: 'email' | 'google' | 'apple' | 'facebook';
      };
    }
  | { name: 'app_user_signed_out'; properties?: Record<string, never> };

// ── Typed capture entry point ─────────────────────────────────────────────────

/**
 * Typed PostHog event capture. Property shapes are enforced per event name.
 * No-op when PostHog is not initialized (key absent).
 * Never include PII — use opaque Clerk user_id only.
 */
export function trackEvent(event: AnalyticsEvent): void {
  if (!initialized) return;
  posthog.capture(event.name, event.properties);
}

// ── Core helpers ──────────────────────────────────────────────────────────────

/**
 * Capture a PostHog event. No-op when PostHog is not initialized.
 * Never include PII — use opaque Clerk user_id only.
 *
 * @deprecated Prefer `trackEvent` which enforces typed property shapes.
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
