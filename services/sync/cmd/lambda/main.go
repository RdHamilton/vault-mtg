// Command mtga-sync-lambda is the AWS Lambda entrypoint for the mtga-sync service.
// AWS EventBridge Scheduler invokes this function on a configurable cron schedule.
//
// # Production (IAM auth — Lambda execution role)
//
// The Lambda connects to RDS using AWS IAM authentication as mandated by ADR-003.
// No static password is stored.  The execution role must have rds-db:connect
// permission for the mtga_sync DB user.  Required environment variables:
//
//	DB_HOST    RDS endpoint hostname
//	DB_NAME    PostgreSQL database name
//	DB_USER    PostgreSQL role name (mtga_sync)
//	DB_PORT    PostgreSQL port (default: 5432)
//	AWS_REGION AWS region of the RDS instance (e.g. us-east-1)
//
// # Local development (direct DSN — bypasses IAM)
//
// Set LAMBDA_LOCAL_DSN to a full PostgreSQL connection string.  When this
// variable is present the IAM token flow is skipped entirely.  Never set
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

	awslambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/dbconn"
	"github.com/ramonehamilton/mtga-sync/internal/handler"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

func main() {
	ctx := context.Background()

	dsn, err := resolveDSN(ctx)
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
//   - Otherwise, a DSN is assembled from DB_HOST / DB_NAME / DB_USER / DB_PORT
//     plus a fresh RDS IAM auth token generated from the Lambda execution role.
//
// IAM tokens expire after 15 minutes.  Fetching once per invocation is safe:
// Lambda invocations are short-lived and the pool is not reused across invocations.
func resolveDSN(ctx context.Context) (string, error) {
	if localDSN := os.Getenv("LAMBDA_LOCAL_DSN"); localDSN != "" {
		log.Println("[sync] LAMBDA_LOCAL_DSN set — skipping IAM auth (local dev mode)")
		return localDSN, nil
	}

	cfg := dbconn.Config{
		Host:   os.Getenv("DB_HOST"),
		Port:   os.Getenv("DB_PORT"),
		DBName: os.Getenv("DB_NAME"),
		User:   os.Getenv("DB_USER"),
		Region: os.Getenv("AWS_REGION"),
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}

	return dbconn.BuildDSN(ctx, cfg, awsCfg.Credentials, auth.BuildAuthToken)
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
