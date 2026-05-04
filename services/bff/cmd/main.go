package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
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

	hub := newHub()
	go hub.run()

	ingestBroadcaster := &hubBroadcaster{hub: hub}
	ingestHandler := handlers.NewIngestHandler(ingestBroadcaster)

	// Wire API key handler and auth middleware when a database is available.
	var (
		apiKeysHandler      *handlers.APIKeysHandler
		apiKeyAuthMiddl     func(http.Handler) http.Handler
		draftRatingsHandler *handlers.DraftRatingsHandler
	)

	if *databaseURL != "" {
		sqlDB, err := sql.Open("pgx", *databaseURL)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}

		apiKeyRepo := repository.NewAPIKeyRepository(sqlDB)
		apiKeysHandler = handlers.NewAPIKeysHandler(apiKeyRepo)
		apiKeyAuthMiddl = bffmiddleware.APIKeyAuth(apiKeyRepo)

		draftRatingsRepo := repository.NewDraftRatingsRepository(sqlDB)
		draftRatingsHandler = handlers.NewDraftRatingsHandler(draftRatingsRepo, cfg)
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

	r.Get("/ws", hub.serveWs)

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

	// GET /api/v1/draft-ratings/{setCode}/{format} — draft card and color ratings.
	if draftRatingsHandler != nil {
		r.Get("/api/v1/draft-ratings/{setCode}/{format}", draftRatingsHandler.GetDraftRatings)
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

// ---------------------------------------------------------------------------
// Minimal in-process WebSocket hub
// ---------------------------------------------------------------------------

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type client struct {
	send chan []byte
	conn *websocket.Conn
}

type hub struct {
	mu      sync.RWMutex
	clients map[*client]bool
	reg     chan *client
	unreg   chan *client
	done    chan struct{}
}

func newHub() *hub {
	return &hub{
		clients: make(map[*client]bool),
		reg:     make(chan *client),
		unreg:   make(chan *client),
		done:    make(chan struct{}),
	}
}

func (h *hub) run() {
	for {
		select {
		case <-h.done:
			return
		case c := <-h.reg:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
		case c := <-h.unreg:
			h.mu.Lock()

			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}

			h.mu.Unlock()
		}
	}
}

func (h *hub) broadcast(event wsEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[hub] marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

func (h *hub) serveWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[hub] upgrade: %v", err)
		return
	}

	c := &client{conn: conn, send: make(chan []byte, 256)}

	h.reg <- c

	go func() {
		defer func() { h.unreg <- c; conn.Close() }()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	go func() {
		defer conn.Close()

		for msg := range c.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()
}

// hubBroadcaster adapts the local hub to handlers.EventBroadcaster.
type hubBroadcaster struct {
	hub *hub
}

func (b *hubBroadcaster) BroadcastDaemonEvent(event contract.DaemonEvent) {
	b.hub.broadcast(wsEvent{
		Type: event.Type,
		Data: event,
	})
}
