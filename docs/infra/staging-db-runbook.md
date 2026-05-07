# Staging Database Runbook

**Database**: `vaultmtg_staging` (logical database on the shared RDS instance `mtga-companion-postgres`)
**Application role**: `vaultmtg_staging_app`
**RDS instance**: `mtga-companion-postgres.cujuc62msfbv.us-east-1.rds.amazonaws.com`
**AWS profile**: `personal` / region `us-east-1`

This runbook covers the full lifecycle of the staging database: initial provisioning,
migration, data reset, and debugging access.

---

## 1. Create the staging database (first time only)

Run once to provision `vaultmtg_staging` and the `vaultmtg_staging_app` role.
This is a destructive action on the RDS instance — confirm the target before running.

```bash
AWS_PROFILE=personal bash infra/scripts/create-staging-db.sh
```

What this does:
- Reads the RDS master credentials from SSM `/mtga-companion/production/db-secret-arn`
- Generates a random password for `vaultmtg_staging_app`
- Writes the password to SSM `/mtga-companion/staging/db-password` (SecureString)
- Writes the full `DATABASE_URL` to SSM `/mtga-companion/staging/database-url` (SecureString)
- Sends an SSM Run Command to the EC2 instance to execute the SQL via `psql`
- Creates the database with `LC_COLLATE = 'en_US.UTF-8'` and `TEMPLATE = template0`
- Grants schema-level privileges to `vaultmtg_staging_app`

If the role already exists (re-run), it updates the password in place using `ALTER ROLE`.

After this step the database is empty — no tables exist yet.
**Proceed immediately to step 2.**

---

## 2. Run migrations on staging (apply schema)

Run after initial provisioning, and after every schema migration that ships to `main`.

```bash
# Requires golang-migrate CLI:
#   go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

AWS_PROFILE=personal bash infra/scripts/run-staging-migrations.sh
```

What this does:
- Reads `DATABASE_URL` from SSM `/mtga-companion/staging/database-url`
- Runs `migrate up` against `vaultmtg_staging` using migrations from
  `services/bff/internal/storage/migrations/postgres/`
- Applies table-level `GRANT` statements from `infra/db/grant-staging-tables.sql`

Idempotent: re-running when already at migration HEAD is a no-op.

To check the current migration version:
```bash
DATABASE_URL=$(aws ssm get-parameter --profile personal \
    --name "/mtga-companion/staging/database-url" \
    --with-decryption --query "Parameter.Value" --output text)

migrate \
  -path services/bff/internal/storage/migrations/postgres \
  -database "$DATABASE_URL" \
  version
```

CI deploy (`.github/workflows/staging-deploy.yml`) runs this automatically on
every push to `main` that touches `services/bff/**`.

---

## 3. Reset / truncate staging data

Wipes all user-data rows from `vaultmtg_staging` without dropping the schema.
Useful before a testing sprint or to clear stale test accounts.

```bash
# Truncate user-data tables only (preserves card/ratings reference data):
AWS_PROFILE=personal bash infra/scripts/truncate-staging-db.sh

# Truncate everything including reference tables (cards, sets, ratings):
AWS_PROFILE=personal bash infra/scripts/truncate-staging-db.sh --all
```

The script prompts for `YES` confirmation before executing.

User-data tables truncated (with `RESTART IDENTITY CASCADE`):
- `users`, `accounts`
- `matches`, `games`, `game_plays`, `game_state_snapshots`
- `draft_sessions`, `draft_picks`, `draft_packs`, `draft_match_results`
- `daemon_events`
- `collection`, `collection_history`, `collection_new`
- `inventory`, `inventory_history`
- `currency_history`
- `rank_history`
- `quests`
- `decks`, `deck_cards`, `deck_notes`, `deck_tags`, `deck_performance_history`, `deck_permutations`
- `player_stats`, `matchup_statistics`
- `opponent_cards_observed`, `opponent_deck_profiles`
- `user_play_patterns`
- `processed_log_files`
- `api_keys`, `settings`, `sync_hashes`

Reference tables (preserved by default, truncated only with `--all`):
- `cards`, `sets`, `set_cards`
- `draft_card_ratings`, `draft_color_ratings`
- `deck_archetypes`, `archetype_card_weights`, `archetype_expected_cards`
- All `edhrec_*`, `cfb_ratings`, `mtgzone_*`, `ml_*`, `card_*` tables

After truncation, run migrations again to ensure the schema is at HEAD:
```bash
AWS_PROFILE=personal bash infra/scripts/run-staging-migrations.sh
```

**Recommended cadence**: truncate weekly (Sunday 03:00 UTC) to prevent staging
from accumulating months of test data. To automate, add a cron entry on the EC2 instance:
```
0 3 * * 0 AWS_PROFILE=personal bash /opt/vaultmtg/infra/scripts/truncate-staging-db.sh >> /var/log/vaultmtg/truncate-staging.log 2>&1
```
Note: automated cron runs skip the interactive confirmation prompt — modify the
script to accept a `--yes` flag if fully unattended operation is needed.

---

## 4. Connect to staging DB for debugging

The RDS instance is not publicly accessible. All psql sessions must go through
the EC2 instance via SSM Session Manager.

### Option A: SSM port-forward (recommended for interactive sessions)

```bash
# On your local machine — opens a local tunnel to port 5433 -> RDS :5432
aws ssm start-session \
    --profile personal \
    --target <EC2_INSTANCE_ID> \
    --document-name AWS-StartPortForwardingSessionToRemoteHost \
    --parameters '{
        "host": ["mtga-companion-postgres.cujuc62msfbv.us-east-1.rds.amazonaws.com"],
        "portNumber": ["5432"],
        "localPortNumber": ["5433"]
    }'

# In a second terminal, connect via psql:
PGPASSWORD=$(aws ssm get-parameter --profile personal \
    --name "/mtga-companion/staging/db-password" \
    --with-decryption --query "Parameter.Value" --output text)

psql -h 127.0.0.1 -p 5433 -U vaultmtg_staging_app -d vaultmtg_staging
```

Find the EC2 instance ID:
```bash
aws ec2 describe-instances \
    --profile personal \
    --filters "Name=tag:Name,Values=mtga-companion" "Name=instance-state-name,Values=running" \
    --query "Reservations[0].Instances[0].InstanceId" \
    --output text
```

### Option B: psql via SSM Run Command (non-interactive)

For one-off queries that do not require an interactive session:
```bash
EC2_INSTANCE_ID=<instance-id>

aws ssm send-command \
    --profile personal \
    --instance-ids "$EC2_INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=["PGPASSWORD=$(aws ssm get-parameter --name /mtga-companion/staging/db-password --with-decryption --query Parameter.Value --output text) psql -h mtga-companion-postgres.cujuc62msfbv.us-east-1.rds.amazonaws.com -U vaultmtg_staging_app -d vaultmtg_staging -c \"SELECT count(*) FROM users;\""]' \
    --output text
```

---

## 5. SSM parameter reference

| Parameter path | Type | Description |
|---|---|---|
| `/mtga-companion/production/db-endpoint` | String | RDS hostname |
| `/mtga-companion/production/db-name` | String | Production database name (`mtga_companion`) |
| `/mtga-companion/production/db-secret-arn` | String | ARN of the Secrets Manager secret holding master credentials |
| `/mtga-companion/staging/db-password` | SecureString | Password for `vaultmtg_staging_app` role |
| `/mtga-companion/staging/database-url` | SecureString | Full `postgres://` URL for the staging database |

Parameters under `/mtga-companion/staging/` are written by `create-staging-db.sh`
and read by `run-staging-migrations.sh`, `truncate-staging-db.sh`, and the
`bff-staging` systemd service.

---

## 6. Isolation guarantees

- `vaultmtg_staging_app` has no privileges on any other database on the RDS instance.
- The role cannot `CONNECT` to `mtga_companion` (the production database).
- If a staging BFF bug issues a malformed query, it can only affect `vaultmtg_staging`.
- The RDS instance itself is shared — see ADR staging-environment-design.md §Negative
  consequences for the accepted blast-radius trade-off.

---

## 7. Disaster recovery

Staging has no backup requirement. If `vaultmtg_staging` is corrupted or needs
to be rebuilt from scratch:

```bash
# 1. Drop the staging database (master user only)
aws ssm send-command --profile personal \
    --instance-ids <EC2_INSTANCE_ID> \
    --document-name AWS-RunShellScript \
    --parameters 'commands=["PGPASSWORD=<master-pw> psql -h <rds-endpoint> -U postgres -c \"DROP DATABASE IF EXISTS vaultmtg_staging;\""]'

# 2. Re-provision
AWS_PROFILE=personal bash infra/scripts/create-staging-db.sh

# 3. Re-apply migrations
AWS_PROFILE=personal bash infra/scripts/run-staging-migrations.sh
```

Full rebuild takes under 5 minutes.
