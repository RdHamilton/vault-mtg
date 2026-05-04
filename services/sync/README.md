# mtga-sync

Scheduled service that fetches 17Lands card ratings and Scryfall set data on a
daily schedule and persists them to the shared PostgreSQL database.

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string. On EC2, sourced from AWS SSM Parameter Store (`/mtga/prod/database_url`) via the systemd `ExecStartPre` step — never hardcoded in the binary or unit file. |
| `SYNC_REFRESH_HOUR` | No | `2` | Hour of day (0–23, UTC) at which the daily sync runs. |
| `SYNC_ACTIVE_SETS` | No | — | Optional comma-separated set codes to override which sets are refreshed (e.g. `FDN,BLB,DSK`). **Not needed in production.** The Scryfall set sync keeps `sets.is_standard_legal` current automatically; the DB-driven path is the expected production path. |

## Postgres Role

The `mtga_sync` role is created by BFF migration `000057_create_sync_user_grants`.
Its grants are:

| Table | Permissions |
|---|---|
| `sets` | `SELECT`, `INSERT`, `UPDATE` (write added by migration `000059`) |
| `set_cards` | `SELECT` |
| `draft_card_ratings` | `SELECT`, `INSERT`, `UPDATE`, `DELETE` |
| `draft_color_ratings` | `SELECT`, `INSERT`, `UPDATE` |
| `dataset_metadata` | `SELECT`, `INSERT`, `UPDATE` |

Write access to user-facing tables (`matches`, `draft_sessions`, `collection`) is
explicitly revoked.

The role password is set post-migration via the deployment pipeline / Secrets
Manager. It is never stored in source control.

## Startup

On a successful database connection the service logs:

```
[mtga-sync] database connection verified
```

Infrastructure can grep this line to confirm connectivity before marking a
deploy successful.

## Systemd Unit

The unit file lives at `.github/deploy/mtga-sync.service`. Key points:

- `ExecStartPre` fetches `DATABASE_URL` from SSM and writes it to
  `/etc/mtga-sync.env` so no credentials appear in the process environment
  visible to other users.
- `SYNC_REFRESH_HOUR` is set inline with `Environment=SYNC_REFRESH_HOUR=2`.
- Restart policy is `on-failure` with a 10 s back-off.

## Running Tests

```bash
cd services/sync
go test ./...
```

Integration tests require a `TEST_DATABASE_URL` environment variable pointing
to a real PostgreSQL instance. They are skipped automatically when the variable
is not set.
