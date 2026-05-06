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
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/api/sse"
	"github.com/ramonehamilton/mtga-bff/internal/config"
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

	broker := sse.New()

	sseBroadcaster := &sseBroadcast{broker: broker}
	ingestHandler := handlers.NewIngestHandler(sseBroadcaster)

	// Wire Clerk auth middleware when CLERK_SECRET_KEY is configured.
	// This middleware protects browser-facing routes by verifying Clerk session
	// JWTs.  When the key is absent (e.g. development without a Clerk account)
	// the middleware is nil and callers fall back to the API-key path or serve
	// a 503.
	var clerkAuthMiddl func(http.Handler) http.Handler
	if cfg.ClerkSecretKey != "" {
		clerkAuthMiddl = bffmiddleware.RequireClerkAuth(cfg.ClerkSecretKey)
	} else {
		log.Println("CLERK_SECRET_KEY not set — Clerk JWT auth disabled (development only).")
	}

	// Wire API key handler and auth middleware when a database is available.
	var (
		apiKeysHandler      *handlers.APIKeysHandler
		apiKeyAuthMiddl     func(http.Handler) http.Handler
		clerkUserResolver   func(http.Handler) http.Handler
		draftRatingsHandler *handlers.DraftRatingsHandler
	)

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

		// Wire Clerk→DB user ID bridge when both Clerk and a database are available.
		userRepo := repository.NewUserRepository(sqlDB)
		clerkUserResolver = bffmiddleware.ClerkUserResolver(userRepo)
	} else {
		log.Printf("WARN: no DATABASE_URL — API key auth unavailable (env=%s); guarded endpoints return 503", cfg.Env)
	}

	r := BuildRouter(cfg, RouterDeps{
		Broker:              broker,
		IngestHandler:       ingestHandler,
		APIKeysHandler:      apiKeysHandler,
		DraftRatingsHandler: draftRatingsHandler,
		ClerkAuthMiddl:      clerkAuthMiddl,
		ClerkUserResolver:   clerkUserResolver,
		APIKeyAuthMiddl:     apiKeyAuthMiddl,
		SentryMiddl:         bffmiddleware.NewSentryMiddleware(),
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
	ClerkAuthMiddl      func(http.Handler) http.Handler
	ClerkUserResolver   func(http.Handler) http.Handler
	APIKeyAuthMiddl     func(http.Handler) http.Handler
	// SentryMiddl is the Sentry panic/error capture middleware.  When non-nil
	// it is installed as the outermost middleware so it captures panics from
	// all downstream handlers.  Safe to omit in tests and development.
	SentryMiddl func(http.Handler) http.Handler
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

	// GET /api/v1/daemon/version — latest daemon version (no auth required).
	daemonVersionHandler := handlers.NewDaemonVersionHandler(cfg)
	r.Get("/api/v1/daemon/version", daemonVersionHandler.GetDaemonVersion)

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

	// POST /v1/ingest/events — API-key auth; falls back to unguarded in dev mode.
	if deps.IngestHandler != nil {
		if deps.APIKeyAuthMiddl != nil {
			r.With(deps.APIKeyAuthMiddl).Post("/v1/ingest/events", deps.IngestHandler.IngestEvent)
		} else {
			r.Post("/v1/ingest/events", deps.IngestHandler.IngestEvent)
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

	switch {
	case deps.ClerkAuthMiddl != nil:
		// Protected group — all routes inside require a valid Clerk JWT.
		r.Group(func(r chi.Router) {
			r.Use(deps.ClerkAuthMiddl)

			// ClerkUserResolver bridges the Clerk string user ID to the DB int64
			// user ID required by handlers.  When not configured (e.g. no DB in
			// development) the group still works but UserIDFromContext returns 0.
			if deps.ClerkUserResolver != nil {
				r.Use(deps.ClerkUserResolver)
			}

			// GET /api/v1/events — SSE stream for browser clients.
			r.Get("/api/v1/events", sseHandler)

			// GET /api/v1/draft-ratings/{setCode}/{format} — draft card and color ratings.
			// Protected to prevent unauthenticated scraping and to scope future
			// per-user personalisation features.
			if deps.DraftRatingsHandler != nil {
				r.Get("/api/v1/draft-ratings/{setCode}/{format}", deps.DraftRatingsHandler.GetDraftRatings)
			}
		})

	case deps.APIKeyAuthMiddl != nil:
		// Fallback: API-key auth when Clerk is not configured (non-production).
		r.With(deps.APIKeyAuthMiddl).Get("/api/v1/events", sseHandler)

		if deps.DraftRatingsHandler != nil {
			r.With(deps.APIKeyAuthMiddl).Get("/api/v1/draft-ratings/{setCode}/{format}", deps.DraftRatingsHandler.GetDraftRatings)
		}

	default:
		// Neither auth backend is configured — serve 503 so the gap is visible.
		r.Get("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable — database not configured", http.StatusServiceUnavailable)
		})

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
