package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	posthoglib "github.com/posthog/posthog-go"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/api/sse"
	"github.com/ramonehamilton/mtga-bff/internal/config"
	"github.com/ramonehamilton/mtga-bff/internal/projection"
	"github.com/ramonehamilton/mtga-bff/internal/storage"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	port        = flag.Int("port", 8080, "HTTP server port")
	databaseURL = flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
)

func runMigrationsWithRetry(dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		log.Println("Running database migrations...")
		err := storage.RunMigrations(dsn)
		if err == nil {
			log.Println("Migrations complete.")
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("migration init: %w", err)
		}
		log.Printf("Database not ready, retrying in 1s: %v", err)
		time.Sleep(time.Second)
	}
}

func main() {
	flag.Parse()

	// BFF_PORT env var is used as a fallback when -port is not explicitly
	// provided on the command line.  This lets the staging systemd unit set
	// Environment=BFF_PORT=8081 without hardcoding -port in ExecStart (which
	// gets overwritten on every deploy).  An explicit -port CLI flag always wins.
	portFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			portFlagSet = true
		}
	})
	if !portFlagSet {
		if envPort := os.Getenv("BFF_PORT"); envPort != "" {
			if _, err := fmt.Sscanf(envPort, "%d", port); err != nil {
				log.Fatalf("invalid BFF_PORT %q: %v", envPort, err)
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Initialise Sentry error monitoring.  The DSN is read from SENTRY_DSN
	// (sourced from SSM /vaultmtg/prod/sentry-bff-dsn at deploy time).
	// When empty, Sentry is disabled — all SDK calls become no-ops.
	// The DSN is never logged.
	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.SentryDSN,
			Environment:      cfg.Env,
			TracesSampleRate: 0.1,
		}); err != nil {
			log.Fatalf("sentry.Init: %v", err)
		}
		// Flush buffered events before the process exits.
		defer sentry.Flush(2 * time.Second)
		log.Println("Sentry initialised.")
	} else {
		log.Println("SENTRY_DSN not set — Sentry disabled (development mode only).")
	}

	if cfg.DatabaseURL != "" {
		if err := runMigrationsWithRetry(cfg.DatabaseURL, 30*time.Second); err != nil {
			log.Fatalf("migrations failed: %v", err)
		}
	} else {
		log.Println("DATABASE_URL not set — skipping migrations (development mode only).")
	}

	fmt.Println("MTGA Companion BFF")
	fmt.Println("==================")
	fmt.Printf("port: %d\n\n", *port)

	// Initialise PostHog server-side analytics.  The API key is read from
	// POSTHOG_API_KEY (sourced from SSM /vaultmtg/prod/posthog-api-key at
	// deploy time).  When empty, PostHog is disabled — a no-op client is used
	// so all handler code paths are always exercised.  The key is never logged.
	var postHogClient handlers.PostHogClient
	if cfg.PostHogAPIKey != "" {
		phClient, err := posthoglib.NewWithConfig(cfg.PostHogAPIKey, posthoglib.Config{
			Endpoint: "https://app.posthog.com",
		})
		if err != nil {
			log.Fatalf("posthog.NewWithConfig: %v", err)
		}
		defer phClient.Close()
		postHogClient = phClient
		log.Println("PostHog initialised.")
	} else {
		log.Println("POSTHOG_API_KEY not set — PostHog disabled (development mode only).")
	}

	broker := sse.New()

	sseBroadcaster := &sseBroadcast{broker: broker}
	ingestHandler := handlers.NewIngestHandler(sseBroadcaster)
	if postHogClient != nil {
		ingestHandler = ingestHandler.WithPostHogClient(postHogClient)
	}

	// Wire Clerk auth middleware when CLERK_SECRET_KEY is configured.
	// This middleware protects browser-facing routes by verifying Clerk session
	// JWTs.  When the key is absent (e.g. development without a Clerk account)
	// the middleware is nil and callers fall back to the API-key path or serve
	// a 503.
	var clerkAuthMiddl func(http.Handler) http.Handler
	var clerkAuthSSEMiddl func(http.Handler) http.Handler
	var clerkOAuthMiddl func(http.Handler) http.Handler
	if cfg.ClerkSecretKey != "" {
		clerkAuthMiddl = bffmiddleware.RequireClerkAuth(cfg.ClerkSecretKey)
		// SSE middleware accepts the Clerk session cookie as a fallback token
		// source, because the browser EventSource API cannot set Authorization
		// headers.  See middleware.RequireClerkAuthForSSE for full design notes.
		clerkAuthSSEMiddl = bffmiddleware.RequireClerkAuthForSSE(cfg.ClerkSecretKey)
	} else {
		log.Println("CLERK_SECRET_KEY not set — Clerk JWT auth disabled (development only).")
	}
	// The daemon's PKCE flow returns a Clerk OAuth access token (jti prefix
	// "oat_") which is NOT a session JWT and is rejected by RequireClerkAuth.
	// RequireClerkOAuthToken validates these tokens via /oauth/userinfo on the
	// configured Frontend API host.
	if cfg.ClerkFrontendAPI != "" {
		clerkOAuthMiddl = bffmiddleware.RequireClerkOAuthToken(cfg.ClerkFrontendAPI)
	} else {
		log.Println("CLERK_FRONTEND_API not set — daemon OAuth-token auth disabled (development only).")
	}

	// Wire API key handler and auth middleware when a database is available.
	var (
		apiKeysHandler        *handlers.APIKeysHandler
		apiKeyAuthMiddl       func(http.Handler) http.Handler
		daemonAPIKeyAuthMiddl func(http.Handler) http.Handler
		clerkUserResolver     func(http.Handler) http.Handler
		draftRatingsHandler   *handlers.DraftRatingsHandler
		historyHandler        *handlers.HistoryHandler
		listV2Handler         *handlers.ListV2Handler
		statsHandler          *handlers.StatsHandler
		daemonHealthHandler   *handlers.DaemonHealthHandler
		daemonRegisterHandler *handlers.DaemonRegisterHandler
		matchesHandler        *handlers.MatchesHandler
		collectionHandler     *handlers.CollectionHandler
		questsHandler         *handlers.QuestsHandler
		standardHandler       *handlers.StandardHandler
		gamePlaysHandler      *handlers.GamePlaysHandler
	)

	// projCtx is cancelled on SIGTERM so the projection worker exits cleanly.
	projCtx, projCancel := context.WithCancel(context.Background())

	if cfg.DatabaseURL != "" {
		sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}

		apiKeyRepo := repository.NewAPIKeyRepository(sqlDB)
		apiKeysHandler = handlers.NewAPIKeysHandler(apiKeyRepo)
		apiKeyAuthMiddl = bffmiddleware.APIKeyAuth(apiKeyRepo)

		draftRatingsRepo := repository.NewDraftRatingsRepository(sqlDB)
		draftRatingsHandler = handlers.NewDraftRatingsHandler(draftRatingsRepo, cfg)

		daemonEventsRepo := repository.NewDaemonEventsRepository(sqlDB)
		ingestHandler = ingestHandler.WithRepository(daemonEventsRepo)

		accountRepo := repository.NewAccountRepository(sqlDB)
		matchesRepo := repository.NewMatchesRepository(sqlDB)
		draftSessionsRepo := repository.NewDraftSessionsRepository(sqlDB)
		deckListRepo := repository.NewDeckListRepository(sqlDB)

		historyHandler = handlers.NewHistoryHandler(accountRepo, matchesRepo, draftSessionsRepo)

		// Phase 2 PR #1 — new /api/v1/matches surface (camelCase, full filter
		// support).  Replaces the SPA's daemonClient /matches calls.  See
		// docs/product/milestones/v0.3.1/daemon-local-api-phase2-audit.md.
		matchesHandler = handlers.NewMatchesHandler(matchesRepo, accountRepo)

		// Phase 2 PR #2 — /api/v1/collection surface (cards/stats/sets/value).
		// Replaces the SPA's daemonClient /collection calls.  See
		// .claude/plans/spa-route-migration.md.
		collectionRepo := repository.NewCollectionRepository(sqlDB)
		collectionHandler = handlers.NewCollectionHandler(collectionRepo, accountRepo)

		// Phase 2 PR #3 — /api/v1/quests surface (active/history/wins/stats).
		// QuestRepository was previously write-only (projection worker writes);
		// PR #3 adds the read-side methods used here.
		questsHandler = handlers.NewQuestsHandler(repository.NewQuestRepository(sqlDB), accountRepo)

		// Phase 2 PR #4 — /api/v1/standard surface (sets/rotation/config/
		// validate/legality). Mixed scope: sets+config+legality are global,
		// rotation+affected-decks+validate are account-scoped via accountRepo.
		standardHandler = handlers.NewStandardHandler(repository.NewStandardRepository(sqlDB), accountRepo)

		// Phase 2 PR #5a — game-plays read surface. Routes mount under
		// /api/v1/matches/{matchId}/plays/* and /api/v1/gameplays/* per the
		// SPA contract (gameplays.ts). Backed by game_plays,
		// game_state_snapshots, and opponent_cards_observed.
		gamePlaysHandler = handlers.NewGamePlaysHandler(repository.NewGamePlaysRepository(sqlDB), accountRepo)

		// ListV2Handler provides cursor-paginated v2 endpoints for matches,
		// drafts, decks, and collection (ADR-018).
		cardInventoryRepoV2 := repository.NewCardInventoryRepository(sqlDB)
		listV2Handler = handlers.NewListV2Handler(
			accountRepo, matchesRepo, draftSessionsRepo, deckListRepo, cardInventoryRepoV2,
		)

		daemonHealthHandler = handlers.NewDaemonHealthHandler(daemonEventsRepo)

		// DaemonRegisterHandler mints (or retrieves) a per-account API key for the
		// daemon PKCE registration flow.  Protected by RequireClerkOAuthToken — the
		// daemon calls this with the Clerk OAuth access token obtained via the PKCE
		// browser flow.  See ADR-020 §POST /api/v1/daemon/register Wire Format.
		daemonAPIKeyRepo := repository.NewDaemonAPIKeyRepository(sqlDB)
		userRepo := repository.NewUserRepository(sqlDB)
		daemonRegisterHandler = handlers.NewDaemonRegisterHandler(daemonAPIKeyRepo, userRepo)
		if postHogClient != nil {
			daemonRegisterHandler = daemonRegisterHandler.WithPostHogClient(postHogClient)
		}

		// daemonAPIKeyAuthMiddl protects daemon-facing routes (currently only
		// POST /api/v1/ingest/events).  It validates the api_key minted by
		// daemon_register against daemon_api_keys, resolves account_id (Clerk
		// user_id) → users.id (int64), and sets user_id on the request context
		// so the standard UserIDFromContext continues to work.
		daemonAPIKeyAuthMiddl = bffmiddleware.DaemonAPIKeyAuth(daemonAPIKeyRepo, userRepo)

		// StatsHandler provides deck performance, win-rate trend, and format
		// distribution analytics endpoints (issue #1513).
		statsRepo := repository.NewStatsRepository(sqlDB)
		statsHandler = handlers.NewStatsHandler(accountRepo, statsRepo, statsRepo, statsRepo).
			WithDraftAnalytics(statsRepo).
			WithRankProgression(statsRepo).
			WithResultBreakdown(statsRepo)

		// Wire Clerk→DB user ID bridge when both Clerk and a database are available.
		// userRepo was created above for daemonRegisterHandler/daemonAPIKeyAuthMiddl.
		clerkUserResolver = bffmiddleware.ClerkUserResolver(userRepo)

		// Start projection worker unless disabled by env var.
		if os.Getenv("BFF_PROJECTION_DISABLED") != "true" {
			cardInventoryRepo := repository.NewCardInventoryRepository(sqlDB)
			inventoryRepo := repository.NewInventoryRepository(sqlDB)
			questRepo := repository.NewQuestRepository(sqlDB)
			deckProjectorRepo := repository.NewDeckProjectorRepository(sqlDB)
			gamePlayRepo := repository.NewGamePlayRepository(sqlDB)
			worker := projection.NewWorker(
				daemonEventsRepo,
				accountRepo,
				matchesRepo,
				draftSessionsRepo,
				cardInventoryRepo,
				inventoryRepo,
				questRepo,
				deckProjectorRepo,
				gamePlayRepo,
			)
			go worker.Run(projCtx)
		} else {
			log.Println("BFF_PROJECTION_DISABLED=true — projection worker not started.")
		}
	} else {
		log.Printf("WARN: no DATABASE_URL — API key auth unavailable (env=%s); guarded endpoints return 503", cfg.Env)
	}

	healthzHandler := handlers.NewHealthzHandler(cfg.Env, cfg.DatabaseURL, storage.MigrationStatus)

	// E2EUnguardedSSE is only honoured in development; in any other env the
	// flag is silently ignored so a misconfigured staging/prod box stays safe.
	e2eUnguardedSSE := cfg.Env == "development" && os.Getenv("BFF_E2E_UNGUARDED_SSE") == "true"
	if e2eUnguardedSSE {
		log.Println("WARN: BFF_E2E_UNGUARDED_SSE=true — SSE endpoint is unauthenticated (E2E mode only)")
	}

	r := BuildRouter(cfg, RouterDeps{
		Broker:                broker,
		IngestHandler:         ingestHandler,
		APIKeysHandler:        apiKeysHandler,
		DraftRatingsHandler:   draftRatingsHandler,
		HistoryHandler:        historyHandler,
		ListV2Handler:         listV2Handler,
		StatsHandler:          statsHandler,
		DaemonHealthHandler:   daemonHealthHandler,
		DaemonRegisterHandler: daemonRegisterHandler,
		MatchesHandler:        matchesHandler,
		CollectionHandler:     collectionHandler,
		QuestsHandler:         questsHandler,
		StandardHandler:       standardHandler,
		GamePlaysHandler:      gamePlaysHandler,
		HealthzHandler:        healthzHandler,
		ClerkAuthMiddl:        clerkAuthMiddl,
		ClerkAuthSSEMiddl:     clerkAuthSSEMiddl,
		ClerkOAuthMiddl:       clerkOAuthMiddl,
		ClerkUserResolver:     clerkUserResolver,
		APIKeyAuthMiddl:       apiKeyAuthMiddl,
		DaemonAPIKeyAuthMiddl: daemonAPIKeyAuthMiddl,
		SentryMiddl:           bffmiddleware.NewSentryMiddleware(),
		E2EUnguardedSSE:       e2eUnguardedSSE,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: r,
	}

	go func() {
		log.Printf("BFF listening on :%d", *port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")

	projCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}

	fmt.Println("BFF stopped.")
}

// RouterDeps holds all optional handlers and middleware that BuildRouter needs.
// Nil fields are treated as "not configured" and the corresponding routes are
// either omitted or served with a degraded response.
type RouterDeps struct {
	Broker              *sse.Broker
	IngestHandler       *handlers.IngestHandler
	APIKeysHandler      *handlers.APIKeysHandler
	DraftRatingsHandler *handlers.DraftRatingsHandler
	HistoryHandler      *handlers.HistoryHandler
	// ListV2Handler serves the cursor-paginated v2 list endpoints (ADR-018).
	ListV2Handler *handlers.ListV2Handler
	// StatsHandler serves the analytics stats endpoints (issue #1513).
	StatsHandler        *handlers.StatsHandler
	DaemonHealthHandler *handlers.DaemonHealthHandler
	// DaemonRegisterHandler serves POST /v1/daemon/register — mints or retrieves
	// a per-account API key for the daemon PKCE registration flow (ADR-020).
	// Protected by RequireClerkAuth — the daemon sends its Clerk session JWT.
	DaemonRegisterHandler *handlers.DaemonRegisterHandler
	// MatchesHandler serves the Phase 2 /api/v1/matches/* surface that the
	// SPA's daemonClient previously hit. Protected by DaemonAPIKeyAuth.
	MatchesHandler *handlers.MatchesHandler
	// CollectionHandler serves the Phase 2 /api/v1/collection/* surface
	// (cards/stats/sets/value). Protected by DaemonAPIKeyAuth.
	CollectionHandler *handlers.CollectionHandler
	// QuestsHandler serves the Phase 2 /api/v1/quests/* surface
	// (active/history/wins/stats). Protected by DaemonAPIKeyAuth.
	QuestsHandler *handlers.QuestsHandler
	// StandardHandler serves the Phase 2 /api/v1/standard/* surface
	// (sets/rotation/config/validate/legality). Protected by DaemonAPIKeyAuth.
	StandardHandler *handlers.StandardHandler
	// GamePlaysHandler serves the Phase 2 in-game telemetry routes:
	// /api/v1/matches/{matchId}/plays/*, /opponent-cards, /snapshots
	// and /api/v1/gameplays/game/{gameId}. Protected by DaemonAPIKeyAuth.
	GamePlaysHandler *handlers.GamePlaysHandler
	// HealthzHandler serves GET /healthz — intentionally public (no auth).
	HealthzHandler *handlers.HealthzHandler
	ClerkAuthMiddl func(http.Handler) http.Handler
	// ClerkAuthSSEMiddl is used exclusively for GET /api/v1/events.  It accepts
	// the Clerk session cookie as a fallback token source in addition to the
	// standard Authorization: Bearer header.  This is required because the
	// browser EventSource API cannot set custom request headers.
	// See middleware.RequireClerkAuthForSSE for the full design rationale.
	ClerkAuthSSEMiddl func(http.Handler) http.Handler
	// ClerkOAuthMiddl validates Clerk OAuth access tokens (jti "oat_*") via
	// /oauth/userinfo.  Used on POST /api/v1/daemon/register — the daemon
	// PKCE flow returns OAuth access tokens, not session JWTs.
	ClerkOAuthMiddl   func(http.Handler) http.Handler
	ClerkUserResolver func(http.Handler) http.Handler
	APIKeyAuthMiddl   func(http.Handler) http.Handler
	// DaemonAPIKeyAuthMiddl validates a daemon api_key against daemon_api_keys
	// and resolves the matching Clerk account_id → users.id (int64). Used on
	// POST /api/v1/ingest/events. The legacy APIKeyAuth path checks the
	// api_keys table which is not where daemon_register stores its keys.
	DaemonAPIKeyAuthMiddl func(http.Handler) http.Handler
	// SentryMiddl is the Sentry panic/error capture middleware.  When non-nil
	// it is installed as the outermost middleware so it captures panics from
	// all downstream handlers.  Safe to omit in tests and development.
	SentryMiddl func(http.Handler) http.Handler
	// E2EUnguardedSSE removes auth from GET /api/v1/events when true.
	// Must only be set when MTGA_ENV=development (enforced in main).
	// Used exclusively by the CI pipeline E2E job (BFF_E2E_UNGUARDED_SSE=true).
	E2EUnguardedSSE bool
}

// BuildRouter constructs and returns the chi router for the BFF service.
// It is a standalone function (not a method) so that tests can call it
// directly without spawning a real HTTP server.
func BuildRouter(cfg *config.Config, deps RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	// SentryMiddl is installed before chi's Recoverer so that panics are
	// captured by Sentry before being swallowed.  Repanic=true (set inside
	// NewSentryMiddleware) ensures chi.Recoverer still writes the 500 response.
	if deps.SentryMiddl != nil {
		r.Use(deps.SentryMiddl)
	}
	r.Use(chimiddleware.Recoverer)
	// AllowedOrigins is configured via the ALLOWED_ORIGINS environment variable
	// (comma-separated list).  See ADR-006 for the full connectivity design.
	// Defaults to localhost-only values when the variable is not set.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID"},
	}))

	// ── Public routes ────────────────────────────────────────────────────────
	// These routes require no authentication.

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"bff"}`))
	})

	// GET /healthz — public health check used by staging deploy checks and uptime
	// monitors.  Returns env and migration status.  Intentionally unauthenticated.
	if deps.HealthzHandler != nil {
		r.Get("/healthz", deps.HealthzHandler.ServeHTTP)
	}

	// GET /api/v1/daemon/version — latest daemon version (no auth required).
	daemonVersionHandler := handlers.NewDaemonVersionHandler(cfg)
	r.Get("/api/v1/daemon/version", daemonVersionHandler.GetDaemonVersion)

	// POST /api/v1/daemon/register — daemon PKCE registration (Clerk OAuth token required).
	// The daemon calls this immediately after completing the PKCE browser flow,
	// sending the Clerk OAuth access token as the Bearer token.  The handler
	// validates the token via Clerk's /oauth/userinfo, mints (or retrieves) a
	// per-account API key, and returns it in the response body.  This route
	// uses RequireClerkOAuthToken (NOT RequireClerkAuth) because the PKCE flow
	// returns OAuth access tokens (jti "oat_*") rather than session JWTs.
	// Mounted under /api/v1/ to match the rest of the daemon-facing API (events,
	// daemon/version) — nginx only forwards /api/v1/* to the BFF.
	// See ADR-020 §POST /api/v1/daemon/register Wire Format.
	if deps.DaemonRegisterHandler != nil {
		if deps.ClerkOAuthMiddl != nil {
			r.With(deps.ClerkOAuthMiddl).Post("/api/v1/daemon/register", deps.DaemonRegisterHandler.Register)
		} else {
			log.Println("WARN: POST /api/v1/daemon/register disabled — CLERK_FRONTEND_API not configured")
		}
	}

	// ── Phase 2 — /api/v1/matches/* (camelCase API, full filter support) ─────
	// Replaces the SPA's daemonClient /matches calls. Protected by
	// DaemonAPIKeyAuth so the daemon's keychain-stored api_key authenticates;
	// the same scheme used by /api/v1/ingest/events.  See
	// docs/product/milestones/v0.3.1/daemon-local-api-phase2-audit.md.
	if deps.MatchesHandler != nil {
		if deps.DaemonAPIKeyAuthMiddl != nil {
			m := deps.MatchesHandler
			auth := deps.DaemonAPIKeyAuthMiddl
			// List + lookup
			r.With(auth).Post("/api/v1/matches", m.List)
			r.With(auth).Get("/api/v1/matches/{matchId}", m.Get)
			r.With(auth).Get("/api/v1/matches/{matchId}/games", m.Games)
			// Filter dropdowns
			r.With(auth).Get("/api/v1/matches/formats", m.Formats)
			r.With(auth).Get("/api/v1/matches/archetypes", m.Archetypes)
			// Aggregations
			r.With(auth).Post("/api/v1/matches/stats", m.Stats)
			r.With(auth).Post("/api/v1/matches/trends", m.Trends)
			r.With(auth).Post("/api/v1/matches/format-distribution", m.FormatDistribution)
			r.With(auth).Post("/api/v1/matches/performance-by-hour", m.PerformanceByHour)
			r.With(auth).Post("/api/v1/matches/matchup-matrix", m.MatchupMatrix)
			// Rank views
			r.With(auth).Get("/api/v1/matches/rank-progression/{format}", m.RankProgression)
			r.With(auth).Get("/api/v1/matches/rank-progression-timeline", m.RankProgressionTimeline)
			// Export
			r.With(auth).Get("/api/v1/matches/export", m.Export)
			// Compare
			r.With(auth).Post("/api/v1/matches/compare", m.Compare)
			r.With(auth).Post("/api/v1/matches/compare/formats", m.CompareFormats)
			r.With(auth).Post("/api/v1/matches/compare/decks", m.CompareDecks)
			r.With(auth).Post("/api/v1/matches/compare/time-periods", m.CompareTimePeriods)
		} else {
			log.Println("WARN: /api/v1/matches/* disabled — DaemonAPIKeyAuth middleware not configured")
		}
	}

	// Phase 2 PR #2 — /api/v1/collection surface. Same auth model + envelope
	// contract as /api/v1/matches/*. Replaces the SPA's daemonClient
	// /collection calls (only the live wrappers — dead Wails-era functions
	// were dropped on the SPA side in this PR).
	if deps.CollectionHandler != nil {
		if deps.DaemonAPIKeyAuthMiddl != nil {
			c := deps.CollectionHandler
			auth := deps.DaemonAPIKeyAuthMiddl
			r.With(auth).Post("/api/v1/collection", c.List)
			r.With(auth).Get("/api/v1/collection/stats", c.Stats)
			r.With(auth).Get("/api/v1/collection/sets", c.Sets)
			r.With(auth).Get("/api/v1/collection/value", c.Value)
		} else {
			log.Println("WARN: /api/v1/collection/* disabled — DaemonAPIKeyAuth middleware not configured")
		}
	}

	// Phase 2 PR #3 — /api/v1/quests surface (active/history/wins/stats).
	// Same auth + envelope contract.
	if deps.QuestsHandler != nil {
		if deps.DaemonAPIKeyAuthMiddl != nil {
			q := deps.QuestsHandler
			auth := deps.DaemonAPIKeyAuthMiddl
			r.With(auth).Get("/api/v1/quests/active", q.Active)
			r.With(auth).Get("/api/v1/quests/history", q.History)
			r.With(auth).Get("/api/v1/quests/wins/daily", q.DailyWins)
			r.With(auth).Get("/api/v1/quests/wins/weekly", q.WeeklyWins)
			r.With(auth).Get("/api/v1/quests/stats", q.Stats)
		} else {
			log.Println("WARN: /api/v1/quests/* disabled — DaemonAPIKeyAuth middleware not configured")
		}
	}

	// Phase 2 PR #5a — game-plays / snapshots / opponent-cards routes that
	// extend the matches surface, plus the dedicated /api/v1/gameplays/game
	// path. Same auth + envelope contract.
	if deps.GamePlaysHandler != nil {
		if deps.DaemonAPIKeyAuthMiddl != nil {
			gp := deps.GamePlaysHandler
			auth := deps.DaemonAPIKeyAuthMiddl
			r.With(auth).Get("/api/v1/matches/{matchId}/plays", gp.MatchPlays)
			r.With(auth).Get("/api/v1/matches/{matchId}/plays/timeline", gp.MatchTimeline)
			r.With(auth).Get("/api/v1/matches/{matchId}/plays/summary", gp.MatchPlaySummary)
			r.With(auth).Get("/api/v1/matches/{matchId}/opponent-cards", gp.MatchOpponentCards)
			r.With(auth).Get("/api/v1/matches/{matchId}/snapshots", gp.MatchSnapshots)
			r.With(auth).Get("/api/v1/gameplays/game/{gameId}", gp.PlaysByGame)
		} else {
			log.Println("WARN: gameplays routes disabled — DaemonAPIKeyAuth middleware not configured")
		}
	}

	// Phase 2 PR #4 — /api/v1/standard surface. Mixed scope: sets / config /
	// card-legality are global; rotation / affected-decks / validate are
	// per-account. Same envelope + DaemonAPIKeyAuth contract.
	if deps.StandardHandler != nil {
		if deps.DaemonAPIKeyAuthMiddl != nil {
			s := deps.StandardHandler
			auth := deps.DaemonAPIKeyAuthMiddl
			r.With(auth).Get("/api/v1/standard/sets", s.Sets)
			r.With(auth).Get("/api/v1/standard/config", s.Config)
			r.With(auth).Get("/api/v1/standard/rotation", s.Rotation)
			r.With(auth).Get("/api/v1/standard/rotation/affected-decks", s.AffectedDecks)
			r.With(auth).Post("/api/v1/standard/validate/{deckId}", s.ValidateDeck)
			r.With(auth).Get("/api/v1/standard/cards/{arenaId}/legality", s.CardLegality)
		} else {
			log.Println("WARN: /api/v1/standard/* disabled — DaemonAPIKeyAuth middleware not configured")
		}
	}

	// ── Daemon-facing routes (APIKey auth) ───────────────────────────────────
	// These routes are called by the local daemon binary, not the browser.
	// All daemon M2M routes are protected by APIKeyAuth — the legacy HMAC
	// DAEMON_JWT_SECRET path has been removed (see ADR-009 / issue #1315).

	// POST /api/keys — create a new API key for a user.
	// Protected by APIKeyAuth so user_id is derived from the verified key,
	// never from a caller-supplied header.
	if deps.APIKeysHandler != nil {
		if deps.APIKeyAuthMiddl != nil {
			r.With(deps.APIKeyAuthMiddl).Post("/api/keys", deps.APIKeysHandler.CreateAPIKey)
		} else {
			// No DB available — route omitted rather than serving unauthenticated.
			log.Println("WARN: POST /api/keys disabled — no database for API key auth")
		}
	}

	// POST /api/v1/ingest/events — daemon api_key auth; falls back to unguarded
	// in dev mode. Uses DaemonAPIKeyAuth (not the legacy APIKeyAuth) because the
	// PKCE-minted daemon api_key lives in daemon_api_keys, not api_keys.
	// Mounted under /api/v1/ so nginx (which only forwards /api/v1/*) can reach it.
	if deps.IngestHandler != nil {
		if deps.DaemonAPIKeyAuthMiddl != nil {
			r.With(deps.DaemonAPIKeyAuthMiddl).Post("/api/v1/ingest/events", deps.IngestHandler.IngestEvent)
		} else {
			r.Post("/api/v1/ingest/events", deps.IngestHandler.IngestEvent)
		}
	}

	// ── Browser-facing protected routes (Clerk JWT auth) ─────────────────────
	// All routes below require a valid Clerk session JWT.
	// Auth priority:
	//   1. Clerk JWT (when CLERK_SECRET_KEY is set) — primary auth for browser clients.
	//   2. API-key fallback (when DATABASE_URL is set but CLERK_SECRET_KEY is not).
	//   3. 503 Service Unavailable — neither auth backend is configured.
	//
	// In production both CLERK_SECRET_KEY and DATABASE_URL are required by
	// config.Load(), so only the Clerk path is reachable in production.

	sseHandler := deps.Broker.Handler(bffmiddleware.UserIDFromContext)

	// sseClerkMiddl resolves to the cookie-aware SSE middleware when available,
	// falling back to the standard Bearer-only middleware.  Both verify the same
	// Clerk JWT — only the token transport differs.
	sseClerkMiddl := deps.ClerkAuthSSEMiddl
	if sseClerkMiddl == nil {
		sseClerkMiddl = deps.ClerkAuthMiddl
	}

	switch {
	case deps.ClerkAuthMiddl != nil:
		// GET /api/v1/events — SSE stream for browser clients.
		//
		// Mounted in its own group with the cookie-aware SSE middleware
		// (ClerkAuthSSEMiddl) instead of the standard ClerkAuthMiddl.  The
		// browser EventSource API cannot set custom Authorization headers, so
		// the SSE middleware also accepts the Clerk session cookie ("__session")
		// as a fallback token source.  All other Clerk-protected routes remain
		// in the Bearer-only group below.
		r.Group(func(r chi.Router) {
			r.Use(sseClerkMiddl)
			if deps.ClerkUserResolver != nil {
				r.Use(deps.ClerkUserResolver)
			}
			r.Get("/api/v1/events", sseHandler)
		})

		// Protected group — all non-SSE routes require a valid Clerk JWT via
		// the standard Authorization: Bearer header.
		r.Group(func(r chi.Router) {
			r.Use(deps.ClerkAuthMiddl)

			// ClerkUserResolver bridges the Clerk string user ID to the DB int64
			// user ID required by handlers.  When not configured (e.g. no DB in
			// development) the group still works but UserIDFromContext returns 0.
			if deps.ClerkUserResolver != nil {
				r.Use(deps.ClerkUserResolver)
			}

			// GET /api/v1/draft-ratings/{setCode}/{format} — draft card and color ratings.
			// Protected to prevent unauthenticated scraping and to scope future
			// per-user personalisation features.
			if deps.DraftRatingsHandler != nil {
				r.Get("/api/v1/draft-ratings/{setCode}/{format}", deps.DraftRatingsHandler.GetDraftRatings)
			}

			// ── Cloud history endpoints (Clerk-protected, Postgres-backed) ──────
			// These are NOT the desktop /api/v1/matches and /api/v1/drafts routes
			// (those are SQLite-backed in the desktop BFF and must not be touched).
			// Cloud history lives under /api/v1/history/ to make the split clear.
			if deps.HistoryHandler != nil {
				r.Get("/api/v1/history/matches", deps.HistoryHandler.GetMatches)
				r.Get("/api/v1/history/drafts", deps.HistoryHandler.GetDrafts)
			}

			// ── v2 cursor-paginated list endpoints (ADR-018) ─────────────────
			// These replace the v1 offset-paginated list endpoints.  v1 routes
			// are kept as deprecation shims for one release (v0.4.0), then
			// removed in v0.4.1.
			if deps.ListV2Handler != nil {
				r.Get("/api/v2/history/matches", deps.ListV2Handler.GetMatches)
				r.Get("/api/v2/history/drafts", deps.ListV2Handler.GetDrafts)
				r.Get("/api/v2/decks", deps.ListV2Handler.GetDecks)
				r.Get("/api/v2/collection", deps.ListV2Handler.GetCollection)
				// /api/v1/collection is a v1 alias for the v2 collection endpoint.
				r.Get("/api/v1/collection", deps.ListV2Handler.GetCollection)
			}

			// ── Stats / analytics endpoints (issues #1513, #1514) ───────────
			if deps.StatsHandler != nil {
				r.Get("/api/v1/stats/deck-performance", deps.StatsHandler.GetDeckPerformance)
				r.Get("/api/v1/stats/win-rate-trend", deps.StatsHandler.GetWinRateTrend)
				r.Get("/api/v1/stats/format-distribution", deps.StatsHandler.GetFormatDistribution)
				r.Get("/api/v1/stats/draft-analytics", deps.StatsHandler.GetDraftAnalytics)
				r.Get("/api/v1/stats/rank-progression", deps.StatsHandler.GetRankProgression)
				r.Get("/api/v1/stats/result-breakdown", deps.StatsHandler.GetResultBreakdown)
			}

			// GET /api/v1/health/daemon — reports whether this user's daemon is
			// currently connected (last event received within 60 s).
			// Always 200; the response body carries the status.
			if deps.DaemonHealthHandler != nil {
				r.Get("/api/v1/health/daemon", deps.DaemonHealthHandler.GetDaemonHealth)
			}
		})

	case deps.APIKeyAuthMiddl != nil:
		// Fallback: API-key auth when Clerk is not configured (non-production).
		r.With(deps.APIKeyAuthMiddl).Get("/api/v1/events", sseHandler)

		if deps.DraftRatingsHandler != nil {
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/draft-ratings/{setCode}/{format}", deps.DraftRatingsHandler.GetDraftRatings)
		}

		if deps.HistoryHandler != nil {
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/history/matches", deps.HistoryHandler.GetMatches)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/history/drafts", deps.HistoryHandler.GetDrafts)
		}

		if deps.ListV2Handler != nil {
			r.With(deps.APIKeyAuthMiddl).Get("/api/v2/history/matches", deps.ListV2Handler.GetMatches)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v2/history/drafts", deps.ListV2Handler.GetDrafts)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v2/decks", deps.ListV2Handler.GetDecks)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v2/collection", deps.ListV2Handler.GetCollection)
			// /api/v1/collection is a v1 alias for the v2 collection endpoint.
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/collection", deps.ListV2Handler.GetCollection)
		}

		if deps.StatsHandler != nil {
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/stats/deck-performance", deps.StatsHandler.GetDeckPerformance)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/stats/win-rate-trend", deps.StatsHandler.GetWinRateTrend)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/stats/format-distribution", deps.StatsHandler.GetFormatDistribution)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/stats/draft-analytics", deps.StatsHandler.GetDraftAnalytics)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/stats/rank-progression", deps.StatsHandler.GetRankProgression)
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/stats/result-breakdown", deps.StatsHandler.GetResultBreakdown)
		}

		if deps.DaemonHealthHandler != nil {
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/health/daemon", deps.DaemonHealthHandler.GetDaemonHealth)
		}

	default:
		if deps.E2EUnguardedSSE {
			// E2E pipeline mode: serve SSE without auth so pipeline log-fixture
			// tests can receive events.  Only reachable when MTGA_ENV=development
			// and BFF_E2E_UNGUARDED_SSE=true (enforced in main before BuildRouter).
			// Inject a sentinel user ID (1) so the SSE broker can subscribe the
			// connection — no real auth is performed in this mode.
			e2eSentinelMiddl := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					next.ServeHTTP(w, req.WithContext(bffmiddleware.WithUserID(req.Context(), 1)))
				})
			}
			r.With(e2eSentinelMiddl).Get("/api/v1/events", sseHandler)
		} else {
			// Neither auth backend is configured — serve 503 so the gap is visible.
			r.Get("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "service unavailable — database not configured", http.StatusServiceUnavailable)
			})
		}

		if deps.DraftRatingsHandler != nil {
			r.Get("/api/v1/draft-ratings/{setCode}/{format}", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "service unavailable — auth not configured", http.StatusServiceUnavailable)
			})
		}
	}

	return r
}

// sseBroadcast adapts the SSE Broker to the handlers.EventBroadcaster interface.
type sseBroadcast struct {
	broker *sse.Broker
}

func (b *sseBroadcast) BroadcastDaemonEvent(userID int64, event contract.DaemonEvent) {
	b.broker.Publish(userID, event)
}
