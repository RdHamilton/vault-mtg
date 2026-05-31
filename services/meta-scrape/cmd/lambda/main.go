// Command meta-scrape-lambda is the AWS Lambda entrypoint for the meta-scrape
// service (#177, ADR-044). AWS EventBridge Scheduler invokes this function daily
// at 03:00 UTC (offset from the 02:00 sync Lambda to avoid NAT/RDS contention).
// Each invocation refreshes the MTGGoldfish + MTGTop8 metagame for every
// supported format, upserts the archetype rows into mtgzone_archetypes, and
// wires the per-archetype card lists into mtgzone_archetype_cards. This is what
// repopulates the prod Meta page.
//
// # Credentials (SSM SecureString, fetched at runtime)
//
// Unlike the sync Lambda (whose DB_PASSWORD is resolved from an SSM Type=String
// param into a plaintext env var at stack-update time per the ADR-035 trade-off),
// this Lambda NEVER places the DB password in its environment. It fetches the
// password from an SSM SecureString parameter at runtime via ssm:GetParameter
// with WithDecryption=true (which requires kms:Decrypt on the param's CMK). The
// password therefore never appears in the Lambda configuration, CloudFormation
// state, or `aws lambda get-function-configuration` output. See #341.
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
//	DB_PORT              PostgreSQL port (default: 5432)
//	LAMBDA_LOCAL_DSN     Full PostgreSQL DSN for local dev — bypasses SSM and the
//	                     env-var assembly entirely. Never set in production.
package main

import (
	"context"
	"log"
	"os"
	"time"

	awslambda "github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/dbconn"
	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/handler"
	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/scraper"
	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/store"
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

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	meta := store.NewMetaStore(pool)
	svc := scraper.NewService(nil, meta)
	h := handler.New(svc, meta)

	awslambda.Start(h.Handle)
}

// resolveDSN returns the PostgreSQL DSN for this invocation.
//
//   - If LAMBDA_LOCAL_DSN is set, it is returned as-is (local dev only).
//   - Otherwise the DSN is assembled from DB_HOST / DB_NAME / DB_USER / DB_PORT
//     plus a password fetched from SSM SecureString at DB_PASSWORD_SSM_PATH.
func resolveDSN(ctx context.Context) (string, error) {
	if localDSN := os.Getenv("LAMBDA_LOCAL_DSN"); localDSN != "" {
		log.Println("[meta-scrape] LAMBDA_LOCAL_DSN set — using local DSN (local dev mode)")
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
