// Command mtga-sync-lambda is the AWS Lambda entrypoint for the mtga-sync service.
// AWS EventBridge Scheduler invokes this function on a configurable cron schedule.
//
// Environment variables:
//
//	DATABASE_URL      PostgreSQL connection string (required)
//	SYNC_ACTIVE_SETS  Comma-separated set codes to refresh, e.g. "FDN,BLB,DSK"
//	                  When unset, active sets are queried from the database.
package main

import (
	"context"
	"log"
	"os"
	"strings"

	awslambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/handler"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("open db pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	store := datasets.NewPostgresStore(pool)
	client := seventeenlands.NewClient()
	h := handler.New(client, store, activeSets())

	awslambda.Start(h.Handle)
}

// activeSets parses SYNC_ACTIVE_SETS and returns a non-nil slice when the env
// var is set, or nil to fall through to DB-driven active set resolution.
func activeSets() []string {
	v := os.Getenv("SYNC_ACTIVE_SETS")
	if v == "" {
		return nil
	}

	var sets []string

	for _, s := range strings.Split(v, ",") {
		if t := strings.TrimSpace(s); t != "" {
			sets = append(sets, t)
		}
	}

	return sets
}
