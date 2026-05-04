// Command mtga-sync fetches 17Lands card ratings and other external data on a
// configurable schedule and persists it to Postgres.
//
// Environment variables:
//
//	DATABASE_URL       PostgreSQL connection string (required)
//	SYNC_REFRESH_HOUR  Hour of day (0-23) to run the daily refresh (default: 2)
//	SYNC_ACTIVE_SETS   Comma-separated set codes to refresh, e.g. "FDN,BLB,DSK"
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/refresh"
	"github.com/ramonehamilton/mtga-sync/internal/scryfall"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("open db pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	store := datasets.NewPostgresStore(pool)
	ratingsClient := seventeenlands.NewClient()
	scryfallClient := scryfall.NewClient()
	sched := refresh.New(scryfallClient, ratingsClient, store)

	log.Println("[mtga-sync] starting scheduler")
	sched.Start(ctx)
	log.Println("[mtga-sync] stopped")
}
