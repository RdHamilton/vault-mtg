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
	"net/url"
	"os"
	"strings"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/datasets"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/dbconn"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/handler"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
	awslambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// DIAGNOSTIC (vault-mtg-tickets#37 follow-up): surface the exact inputs
	// used to construct the IAM auth token so a stubborn PAM-auth failure can
	// be root-caused from CloudWatch logs without round-tripping a redeploy
	// per hypothesis. Remove after the live root cause is identified.
	//
	// Round 2: label order matches value order — DB_USER then DB_NAME.
	// Earlier rev had labels DB_USER/DB_NAME but passed cfg.DBName/cfg.User,
	// which is what produced the swapped-looking CloudWatch line. The
	// underlying cfg.User (= os.Getenv("DB_USER") = "mtga_sync") was always
	// correct — see BuildAuthToken inputs line for the load-bearing value.
	log.Printf("[sync:diag] env DB_HOST=%q DB_PORT=%q DB_USER=%q DB_NAME=%q AWS_REGION=%q",
		cfg.Host, cfg.Port, cfg.User, cfg.DBName, cfg.Region)
	log.Printf("[sync:diag] env AWS_DEFAULT_REGION=%q AWS_LAMBDA_FUNCTION_NAME=%q AWS_EXECUTION_ENV=%q",
		os.Getenv("AWS_DEFAULT_REGION"),
		os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
		os.Getenv("AWS_EXECUTION_ENV"))

	// Round 2 hypothesis 3 — AWS_SESSION_TOKEN truncation/staleness in transit.
	// Log only the PREFIX of the env-injected session token to confirm whether
	// the SDK is using a healthy STS-issued token. Never log the full token.
	sessionTokenEnv := os.Getenv("AWS_SESSION_TOKEN")
	sessionTokenPrefix := sessionTokenEnv
	if len(sessionTokenPrefix) > 32 {
		sessionTokenPrefix = sessionTokenPrefix[:32] + "..."
	}
	log.Printf("[sync:diag] env AWS_SESSION_TOKEN prefix=%q len=%d",
		sessionTokenPrefix, len(sessionTokenEnv))

	accessKeyEnv := os.Getenv("AWS_ACCESS_KEY_ID")
	accessKeyEnvPrefix := accessKeyEnv
	if len(accessKeyEnvPrefix) > 4 {
		accessKeyEnvPrefix = accessKeyEnvPrefix[:4] + "..."
	}
	log.Printf("[sync:diag] env AWS_ACCESS_KEY_ID prefix=%q len=%d",
		accessKeyEnvPrefix, len(accessKeyEnv))

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}

	log.Printf("[sync:diag] aws.Config Region=%q (LoadDefaultConfig)", awsCfg.Region)

	// Retrieve credentials once just for diagnostics so we can log the issuer
	// info without leaking secrets. The credential cache will satisfy the
	// subsequent Retrieve() call from inside BuildAuthToken.
	creds, credsErr := awsCfg.Credentials.Retrieve(ctx)
	if credsErr != nil {
		log.Printf("[sync:diag] creds.Retrieve error: %v", credsErr)
	} else {
		accessKeyPrefix := creds.AccessKeyID
		if len(accessKeyPrefix) > 4 {
			accessKeyPrefix = accessKeyPrefix[:4] + "..."
		}
		log.Printf("[sync:diag] creds Source=%q AccessKeyIDPrefix=%q HasSessionToken=%t CanExpire=%t Expires=%s",
			creds.Source, accessKeyPrefix, creds.SessionToken != "", creds.CanExpire, creds.Expires.UTC().Format("2006-01-02T15:04:05Z"))

		// Round 2 hypothesis 3 — confirm SDK creds match env-injected creds.
		// If the SDK is using stale or alternate credentials (e.g. from a
		// cached chain provider), this comparison will flag it.
		credsMatch := creds.AccessKeyID == accessKeyEnv
		log.Printf("[sync:diag] creds SDK_AccessKeyID matches env AWS_ACCESS_KEY_ID: %t", credsMatch)
	}

	// Round 2 hypothesis 2 — STS GetCallerIdentity from within the Lambda.
	// Confirms:
	//   (a) STS is reachable from this Lambda VPC (no DNS / endpoint surprise)
	//   (b) The principal the SDK actually presents matches the role we
	//       simulated against (mtga-companion-production-sync-lambda-role)
	//   (c) The session has not been wrapped by an unexpected role chain
	stsClient := sts.NewFromConfig(awsCfg)
	if ident, idErr := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); idErr != nil {
		log.Printf("[sync:diag] sts.GetCallerIdentity error: %v", idErr)
	} else {
		account := ""
		if ident.Account != nil {
			account = *ident.Account
		}
		arn := ""
		if ident.Arn != nil {
			arn = *ident.Arn
		}
		userID := ""
		if ident.UserId != nil {
			userID = *ident.UserId
		}
		log.Printf("[sync:diag] sts.GetCallerIdentity Account=%q Arn=%q UserId=%q",
			account, arn, userID)
	}

	dsn, err := dbconn.BuildDSN(ctx, cfg, awsCfg.Credentials, instrumentedTokenProvider)
	if err != nil {
		log.Printf("[sync:diag] BuildDSN error: %v", err)
		return "", err
	}

	return dsn, nil
}

// instrumentedTokenProvider wraps auth.BuildAuthToken with logging that
// surfaces the exact endpoint, region, dbUser passed to the signer and the
// generated token shape (host + query keys, never the signature). Used only
// while diagnosing vault-mtg-tickets#37 PAM auth failure.
func instrumentedTokenProvider(
	ctx context.Context, endpoint, region, dbUser string,
	creds aws.CredentialsProvider, optFns ...func(*auth.BuildAuthTokenOptions),
) (string, error) {
	log.Printf("[sync:diag] BuildAuthToken inputs endpoint=%q region=%q dbUser=%q",
		endpoint, region, dbUser)

	token, err := auth.BuildAuthToken(ctx, endpoint, region, dbUser, creds, optFns...)
	if err != nil {
		log.Printf("[sync:diag] BuildAuthToken error: %v", err)
		return token, err
	}

	// Token format: <host>:<port>/?Action=connect&DBUser=...&X-Amz-Algorithm=...
	// Log the host:port + query parameter NAMES so we can verify shape without
	// leaking the signature.
	//
	// Round 2: also log the X-Amz-Credential VALUE (access key prefix + scope:
	// <KEY>/<DATE>/<REGION>/<SERVICE>/aws4_request). This is the load-bearing
	// value for hypothesis 2/3 — it reveals which principal the signer
	// actually used, the date stamp the request was signed with, and the
	// service scope. None of these are secret. X-Amz-Signature is the only
	// secret query parameter and we never log it.
	if i := strings.IndexByte(token, '?'); i >= 0 {
		hostPart := token[:i]
		queryPart := token[i+1:]

		var (
			paramNames    []string
			amzCredential string
			amzDate       string
			amzExpires    string
			dbUserParam   string
			actionParam   string
		)
		for _, kv := range strings.Split(queryPart, "&") {
			eq := strings.IndexByte(kv, '=')
			if eq < 0 {
				paramNames = append(paramNames, kv)
				continue
			}
			name := kv[:eq]
			paramNames = append(paramNames, name)
			value := kv[eq+1:]
			// URL-decode the credential value so we can read the scope cleanly.
			// Errors here are not load-bearing — just log the raw value.
			switch name {
			case "X-Amz-Credential":
				if decoded, decodeErr := url.QueryUnescape(value); decodeErr == nil {
					amzCredential = decoded
				} else {
					amzCredential = value
				}
				// Redact the access key portion to avoid leaking the AKID;
				// keep the scope (date/region/service/aws4_request) intact.
				if slash := strings.IndexByte(amzCredential, '/'); slash > 0 {
					key := amzCredential[:slash]
					scope := amzCredential[slash:]
					if len(key) > 4 {
						key = key[:4] + "..."
					}
					amzCredential = key + scope
				}
			case "X-Amz-Date":
				amzDate = value
			case "X-Amz-Expires":
				amzExpires = value
			case "DBUser":
				dbUserParam = value
			case "Action":
				actionParam = value
			}
		}
		log.Printf("[sync:diag] BuildAuthToken result host=%q len=%d params=%v",
			hostPart, len(token), paramNames)
		log.Printf("[sync:diag] BuildAuthToken token Action=%q DBUser=%q X-Amz-Date=%q X-Amz-Expires=%q X-Amz-Credential=%q",
			actionParam, dbUserParam, amzDate, amzExpires, amzCredential)
	} else {
		log.Printf("[sync:diag] BuildAuthToken result UNEXPECTED_SHAPE len=%d", len(token))
	}

	return token, nil
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
