# mtga-sync

Scheduled Lambda function that fetches 17Lands card ratings and Scryfall set metadata
on a daily schedule and persists them to the shared PostgreSQL database.

## Architecture

The sync service runs as an **AWS Lambda function** triggered by **EventBridge Scheduler**
on a nightly cron. Each invocation performs a single full sync run across all active
draft sets and formats, then exits. There is no long-running process.

```
EventBridge Scheduler (cron)
        │
        ▼
  Lambda: mtga-sync          ← cmd/lambda/main.go
        │
        ├── FetchSets (Scryfall) → UpsertSets → sets table
        ├── FetchCardRatings (17Lands) → delta check → UpsertRatings → draft_card_ratings
        └── FetchColorRatings (17Lands) → UpsertColorRatings → draft_color_ratings
```

## Lambda Entrypoint

`cmd/lambda/main.go` — connects to RDS, wires `SyncHandler`, and calls `lambda.Start`.

The binary is compiled as `bootstrap` (the Lambda custom runtime convention):

```bash
GOOS=linux GOARCH=amd64 go build -o bootstrap ./cmd/lambda/main.go
zip sync-lambda.zip bootstrap
```

CI builds and deploys this automatically on every push to `main` via
`.github/workflows/sync.yml`.

## Environment Variables

### Production (IAM auth — Lambda execution role)

The Lambda connects to RDS using AWS IAM authentication. No static password is stored.
The execution role must have `rds-db:connect` permission for the `mtga_sync` DB user.

| Variable | Required | Description |
|---|---|---|
| `DB_HOST` | Yes | RDS endpoint hostname |
| `DB_NAME` | Yes | PostgreSQL database name |
| `DB_USER` | Yes | PostgreSQL role name (`mtga_sync`) |
| `DB_PORT` | No | PostgreSQL port (default: `5432`) |
| `AWS_REGION` | Yes | AWS region of the RDS instance (e.g. `us-east-1`) |
| `SYNC_ACTIVE_SETS` | No | Comma-separated set codes to refresh (e.g. `FDN,BLB,DSK`). When unset, active sets are queried from `sets.is_draft_active`. Not needed in production. |
| `SYNC_FORMATS` | No | Comma-separated 17Lands format names (e.g. `PremierDraft,QuickDraft,Sealed`). Defaults to `PremierDraft,QuickDraft`. |

### Local Development

Set `LAMBDA_LOCAL_DSN` to a full PostgreSQL connection string to skip IAM auth entirely.
**Never set this in a production Lambda environment.**

| Variable | Description |
|---|---|
| `LAMBDA_LOCAL_DSN` | Full PostgreSQL DSN for local dev (e.g. `postgres://user:pass@localhost/mtga`) |

## EventBridge Trigger

The Lambda is triggered by an **EventBridge Scheduler** rule. The schedule is configured
in the AWS console or via Terraform — the binary does not manage its own schedule.

Recommended schedule: `cron(0 2 * * ? *)` (02:00 UTC daily).

The event payload is ignored. Any invocation triggers a full sync across all active sets.

## Deploy Process

Deployment is automated via `.github/workflows/sync.yml`:

1. On push to `main` (path-filtered to `services/sync/**`), CI builds the `bootstrap` binary
   for `linux/amd64` and zips it.
2. CI authenticates to AWS using OIDC (no stored access keys) via the role in
   `secrets.AWS_DEPLOY_ROLE_ARN`.
3. `aws lambda update-function-code` uploads the zip to the `mtga-sync` function.
4. CI waits for `function-updated` before reporting success.

Manual deploy (emergency use only):

```bash
cd services/sync
GOOS=linux GOARCH=amd64 go build -o bootstrap ./cmd/lambda/main.go
zip sync-lambda.zip bootstrap
aws lambda update-function-code \
  --function-name mtga-sync \
  --zip-file fileb://sync-lambda.zip \
  --region us-east-1
```

## Local Dev Invocation

To invoke the Lambda handler locally against a dev database:

```bash
export LAMBDA_LOCAL_DSN="postgres://mtga_sync:yourpassword@localhost:5432/mtga?sslmode=disable"
export SYNC_ACTIVE_SETS="FDN"      # optional: restrict to one set
export SYNC_FORMATS="PremierDraft" # optional: restrict to one format
go run ./cmd/lambda/main.go
```

The `LAMBDA_LOCAL_DSN` path bypasses IAM token generation. The handler runs once and exits.

## Delta Sync Behavior

Each invocation checks whether the fetched payload has changed before writing to the
database. This avoids unnecessary writes on days when 17Lands has not updated ratings.

### Hash-check flow (per set/format pair)

1. **Fetch** card ratings from 17Lands for the set/format.
2. **Sort** cards by `MtgaID` to produce a stable byte sequence.
3. **Hash** the sorted payload (SHA-256).
4. **Compare** the computed hash against the stored hash via `GetHash(ctx, key)`.
   - `key` format: `<setCode>/<draftFormat>` (e.g. `FDN/PremierDraft`)
   - `GetHash` queries the `sync_hashes` table (migration `000065_add_sync_hashes`).
   - Returns `("", nil)` when no hash has been stored yet (first run).
5. **Skip** if the hashes match — the payload is unchanged. `UpsertRatings` is **not**
   called, so `draft_card_ratings.cached_at` is **not** updated on a skip run.
6. **Upsert** if the hashes differ (or no stored hash exists): call `UpsertRatings` to
   replace all rows for the set/format, then call `SetHash(ctx, key, newHash)` to
   persist the new hash.

### Hash store

Hashes are stored in the `sync_hashes` table, created by BFF migration
`000065_add_sync_hashes`:

```sql
CREATE TABLE IF NOT EXISTS sync_hashes (
    key        TEXT PRIMARY KEY,
    hash       TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

The `Store` interface exposes two methods:

```go
// GetHash returns the stored hash for the given key, or ("", nil) if none exists.
GetHash(ctx context.Context, key string) (string, error)

// SetHash upserts the hash for the given key.
SetHash(ctx context.Context, key string, hash string) error
```

`dataset_metadata.dataset_version` is **not** used for delta tracking. `sync_hashes` is
the canonical hash store.

### Effect on cached_at

`draft_card_ratings.cached_at` is set to `time.Now().UTC()` only when `UpsertRatings`
runs. A skip run (hash match) leaves `cached_at` unchanged. The BFF staleness check
(`X-Cache-Degraded`) is based on `cached_at`, so a skip run does not reset the
staleness clock.

## Postgres Role

The `mtga_sync` role is created by BFF migration `000057_create_sync_user_grants`.
Its grants are:

| Table | Permissions |
|---|---|
| `sets` | `SELECT`, `INSERT`, `UPDATE` |
| `set_cards` | `SELECT` |
| `draft_card_ratings` | `SELECT`, `INSERT`, `UPDATE`, `DELETE` |
| `draft_color_ratings` | `SELECT`, `INSERT`, `UPDATE` |
| `sync_hashes` | `SELECT`, `INSERT`, `UPDATE`, `DELETE` |

Write access to user-facing tables (`matches`, `draft_sessions`, `collection`) is
explicitly revoked. The role password is never stored in source control.

## Running Tests

```bash
cd services/sync
go test ./...
```

Integration tests require a `TEST_DATABASE_URL` environment variable pointing to a real
PostgreSQL instance. They are skipped automatically when the variable is not set.
