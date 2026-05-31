// Command mtga-sync-lambda is the AWS Lambda entrypoint for the mtga-sync service.
// AWS EventBridge Scheduler invokes this function on a configurable cron schedule.
//
// # Credentials (SSM SecureString, fetched at runtime)
//
// Unlike the former design (DB_PASSWORD injected as a plaintext env var via
// CFN {{resolve:ssm:...}} at stack-update time per ADR-035), this Lambda NEVER
// places the DB password in its environment. It fetches the password from an SSM
// SecureString parameter at runtime via ssm:GetParameter with WithDecryption=true
// (which requires kms:Decrypt on the param's CMK). The password therefore never
// appears in the Lambda configuration, CloudFormation state, or
// `aws lambda get-function-configuration` output. See #341.
//
// Required environment variables:
//
//	DB_HOST              RDS endpoint hostname
//	DB_NAME              PostgreSQL database name
//	DB_USER              PostgreSQL role name (mtga_sync)
//	DB_PASSWORD_SSM_PATH SSM SecureString param path holding the DB password
//
// Optional:
//
//	DB_PORT          PostgreSQL port (default: 5432)
//	LAMBDA_LOCAL_DSN Full PostgreSQL DSN for local dev — bypasses SSM and the
//	                 env-var assembly entirely. Never set in production.
//	SYNC_ACTIVE_SETS Comma-separated set codes to refresh, e.g. "FDN,BLB,DSK"
//	                 When unset, active sets are queried from the database.
//
// (PR #2650 is the forensic record of the prior IAM-auth attempt.)
package main

import (
	"context"
	"log"
	"os"
	"strings"

	awslambda "github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/datasets"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/dbconn"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/handler"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
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
	scryfallClient := scryfall.NewClient()
	h := handler.New(client, scryfallClient, store, activeSets())

	awslambda.Start(h.Handle)
}

// resolveDSN returns the PostgreSQL DSN for this invocation.
//
//   - If LAMBDA_LOCAL_DSN is set, it is returned as-is (local dev only).
//   - Otherwise the DSN is assembled from DB_HOST / DB_NAME / DB_USER / DB_PORT
//     plus a password fetched from SSM SecureString at DB_PASSWORD_SSM_PATH.
func resolveDSN(ctx context.Context) (string, error) {
	if localDSN := os.Getenv("LAMBDA_LOCAL_DSN"); localDSN != "" {
		log.Println("[sync] LAMBDA_LOCAL_DSN set — using local DSN (local dev mode)")
		return localDSN, nil
	}

	password, err := fetchDBPassword(ctx, os.Getenv("DB_PASSWORD_SSM_PATH"))
	if err != nil {
		return "", err
	}

	return dbconn.BuildPasswordDSN(dbconn.Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		DBName:   os.Getenv("DB_NAME"),
		User:     os.Getenv("DB_USER"),
		Password: password,
	})
}

// fetchDBPassword reads the DB password from an SSM SecureString parameter,
// decrypting it at runtime (WithDecryption=true → requires kms:Decrypt). The
// decrypted value is never written to the environment or logged.
func fetchDBPassword(ctx context.Context, ssmPath string) (string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}
	return dbconn.FetchSecureString(ctx, ssm.NewFromConfig(cfg), ssmPath)
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
