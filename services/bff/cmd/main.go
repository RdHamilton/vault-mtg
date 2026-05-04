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
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/api/sse"
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

	if *databaseURL != "" {
		if err := runMigrationsWithRetry(*databaseURL, 30*time.Second); err != nil {
			log.Fatalf("migrations failed: %v", err)
		}
	} else {
		log.Println("DATABASE_URL not set — skipping migrations.")
	}

	fmt.Println("MTGA Companion BFF")
	fmt.Println("==================")
	fmt.Printf("port: %d\n\n", *port)

	broker := sse.New()

	sseBroadcaster := &sseBroadcast{broker: broker}
	ingestHandler := handlers.NewIngestHandler(sseBroadcaster)

	// Wire API key handler and auth middleware when a database is available.
	var (
		apiKeysHandler  *handlers.APIKeysHandler
		apiKeyAuthMiddl func(http.Handler) http.Handler
	)

	if *databaseURL != "" {
		sqlDB, err := sql.Open("pgx", *databaseURL)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}

		apiKeyRepo := repository.NewAPIKeyRepository(sqlDB)
		apiKeysHandler = handlers.NewAPIKeysHandler(apiKeyRepo)
		apiKeyAuthMiddl = bffmiddleware.APIKeyAuth(apiKeyRepo)
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "X-User-ID"},
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"bff"}`))
	})

	// GET /api/v1/events — SSE stream for browser clients.
	r.Get("/api/v1/events", broker.ServeHTTP)

	// POST /api/keys — create a new API key for a user (placeholder auth via X-User-ID).
	if apiKeysHandler != nil {
		r.Post("/api/keys", apiKeysHandler.CreateAPIKey)
	}

	// POST /v1/ingest/events — guarded by API key auth when DB is available.
	if apiKeyAuthMiddl != nil {
		r.With(apiKeyAuthMiddl).Post("/v1/ingest/events", ingestHandler.IngestEvent)
	} else {
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

func (b *sseBroadcast) BroadcastDaemonEvent(event contract.DaemonEvent) {
	b.broker.Publish(event)
}
