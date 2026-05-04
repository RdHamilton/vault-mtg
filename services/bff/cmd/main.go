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
	"strings"
	"syscall"
	"time"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
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
	port            = flag.Int("port", 8080, "HTTP server port")
	databaseURL     = flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	daemonJWTSecret = strings.TrimSpace(os.Getenv("DAEMON_JWT_SECRET"))
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

	// Wire daemon register handler when DAEMON_JWT_SECRET is set.
	var daemonRegisterHandler *handlers.DaemonRegisterHandler
	if daemonJWTSecret != "" {
		daemonRegisterHandler = handlers.NewDaemonRegisterHandler(daemonJWTSecret)
	} else {
		log.Println("DAEMON_JWT_SECRET not set — daemon registration endpoint disabled.")
	}

	// Wire API key handler and auth middleware when a database is available.
	var (
		apiKeysHandler      *handlers.APIKeysHandler
		apiKeyAuthMiddl     func(http.Handler) http.Handler
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
	} else {
		log.Printf("WARN: no DATABASE_URL — API key auth unavailable (env=%s); guarded endpoints return 503", cfg.Env)
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	// AllowedOrigins is configured via the ALLOWED_ORIGINS environment variable
	// (comma-separated list).  See ADR-006 for the full connectivity design.
	// Defaults to localhost-only values when the variable is not set.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID"},
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"bff"}`))
	})

	// GET /api/v1/events — SSE stream for browser clients.
	//
	// Auth middleware is ALWAYS required.  Without a database the endpoint
	// returns 503 Service Unavailable rather than falling back to an
	// unauthenticated stream.  This closes the security gap reported in
	// issue #1141 where the endpoint was unguarded when DATABASE_URL was
	// unset.
	sseHandler := broker.Handler(bffmiddleware.UserIDFromContext)
	if apiKeyAuthMiddl != nil {
		r.With(apiKeyAuthMiddl).Get("/api/v1/events", sseHandler)
	} else {
		r.Get("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable — database not configured", http.StatusServiceUnavailable)
		})
	}

	// POST /api/keys — create a new API key for a user.
	// Requires a valid daemon JWT so user_id is derived from the verified token,
	// never from a caller-supplied header.
	if apiKeysHandler != nil {
		if daemonJWTSecret != "" {
			r.With(bffmiddleware.DaemonJWTAuth(daemonJWTSecret)).Post("/api/keys", apiKeysHandler.CreateAPIKey)
		} else {
			// No JWT secret configured — omit the route entirely rather than
			// serving it without authentication.
			log.Println("WARN: POST /api/keys disabled — DAEMON_JWT_SECRET not set")
		}
	}

	// POST /api/daemon/register — issue a daemon JWT; requires DAEMON_JWT_SECRET and
	// a valid API key so the user_id is derived from context, never from the body.
	if daemonRegisterHandler != nil {
		if apiKeyAuthMiddl != nil {
			r.With(apiKeyAuthMiddl).Post("/api/daemon/register", daemonRegisterHandler.Register)
		} else {
			// No DB available — registration is impossible without user identity.
			// Log a warning; the route is omitted rather than serving unauthenticated.
			log.Println("WARN: daemon register endpoint disabled — no database for API key auth")
		}
	}

	// GET /api/v1/draft-ratings/{setCode}/{format} — draft card and color ratings.
	if draftRatingsHandler != nil {
		r.Get("/api/v1/draft-ratings/{setCode}/{format}", draftRatingsHandler.GetDraftRatings)
	}

	// POST /v1/ingest/events — JWT auth takes priority when secret is configured;
	// falls back to API-key auth, then unguarded (dev mode).
	switch {
	case daemonJWTSecret != "":
		r.With(bffmiddleware.DaemonJWTAuth(daemonJWTSecret)).Post("/v1/ingest/events", ingestHandler.IngestEvent)
	case apiKeyAuthMiddl != nil:
		r.With(apiKeyAuthMiddl).Post("/v1/ingest/events", ingestHandler.IngestEvent)
	default:
		r.Post("/v1/ingest/events", ingestHandler.IngestEvent)
	}

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

// sseBroadcast adapts the SSE Broker to the handlers.EventBroadcaster interface.
type sseBroadcast struct {
	broker *sse.Broker
}

func (b *sseBroadcast) BroadcastDaemonEvent(userID int64, event contract.DaemonEvent) {
	b.broker.Publish(userID, event)
}
