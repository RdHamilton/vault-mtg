// Command mtga-sync-lambda is the AWS Lambda entrypoint for the mtga-sync service.
// AWS EventBridge Scheduler invokes this function on a configurable cron schedule.
//
// # Production (password auth)
//
// The Lambda connects to RDS PostgreSQL using a static password supplied via
// the DB_PASSWORD environment variable. CloudFormation resolves the password
// from SSM Parameter Store at stack-update time (see sync-lambda.yml's
// DbPasswordSsmPath parameter) and injects it into the Lambda's environment.
// The mtga_sync DB role is M2M-only — there is no human path through this
// credential and no PII flows through it. Required environment variables:
//
//	DB_HOST     RDS endpoint hostname
//	DB_NAME     PostgreSQL database name
//	DB_USER     PostgreSQL role name (mtga_sync)
//	DB_PORT     PostgreSQL port (default: 5432)
//	DB_PASSWORD PostgreSQL password for the DB_USER role
//
// (PR #2650 is the forensic record of the IAM-auth attempt that preceded
// this design; left open for any future re-attempt.)
//
// # Local development (direct DSN — bypasses env-var assembly)
//
// Set LAMBDA_LOCAL_DSN to a full PostgreSQL connection string. When this
// variable is present the env-var path is skipped entirely. Never set
// this in production Lambda environment variables.
//
//	LAMBDA_LOCAL_DSN  PostgreSQL DSN for local dev (e.g. postgres://user:pass@localhost/mtga)
//
// # Optional
//
//	SYNC_ACTIVE_SETS  Comma-separated set codes to refresh, e.g. "FDN,BLB,DSK"
//	                  When unset, active sets are queried from the database.
package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/datasets"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/dbconn"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/handler"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
	awslambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	dsn, err := resolveDSN()
	if err != nil {
		log.Fatalf("resolve DB connection: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
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

// resolveDSN returns the PostgreSQL DSN to use for this invocation.
//
//   - If LAMBDA_LOCAL_DSN is set, it is returned as-is (local dev only).
//   - Otherwise, a DSN is assembled from DB_HOST / DB_NAME / DB_USER / DB_PORT /
//     DB_PASSWORD. DB_PASSWORD is supplied by CFN at stack-update time from
//     SSM Parameter Store.
func resolveDSN() (string, error) {
	if localDSN := os.Getenv("LAMBDA_LOCAL_DSN"); localDSN != "" {
		log.Println("[sync] LAMBDA_LOCAL_DSN set — using local DSN (local dev mode)")
		return localDSN, nil
	}

	cfg := dbconn.Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		DBName:   os.Getenv("DB_NAME"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
	}

	return dbconn.BuildPasswordDSN(cfg)
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
