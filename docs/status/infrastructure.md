# Infrastructure Agent -- Current Task Status
**Updated**: 2026-05-10T08:00 ET
**Task**: Triage restart-bff-staging.sh and run-staging-migrations.sh failures
**Status**: Fixes applied — PR pending

---

## Context

`restart-bff-staging.sh` and `run-staging-migrations.sh` have been failing on every staging deploy for 24h. Staging has not completed end-to-end.

---

## Root Cause Analysis

### Failure 1 — `run-staging-migrations.sh`: Wrong script path in workflow (P0)

**Workflow line 167:**
```
commands=["AWS_PROFILE=default bash /opt/mtga-companion/scripts/run-staging-migrations.sh"]
```

- `/opt/mtga-companion/scripts/run-staging-migrations.sh` does not exist on EC2.  
  The script lives at `infra/scripts/run-staging-migrations.sh` in the repo and was **never synced to S3** (only `scripts/deploy/` was synced).
- `AWS_PROFILE=default` is wrong. On EC2, the instance IAM role provides credentials — no named profile. The script also defaulted to `personal` which also breaks.

**Fix applied:**
- Workflow now syncs `infra/scripts/` to `s3://.../infra-scripts/` alongside `scripts/deploy/`.
- Workflow migration step now copies `run-staging-migrations.sh` from S3 to `/tmp/` and executes it (same pattern as all other scripts).
- `--profile` flag removed from all AWS calls in the script; replaced with a conditional `_PROFILE_ARG` that is only set when `AWS_PROFILE` env var is present (for local use), absent for EC2.

### Failure 2 — `run-staging-migrations.sh`: Wrong `REPO_ROOT` when run from `/tmp` (P0)

The script computed `REPO_ROOT` from `${BASH_SOURCE[0]}`. When copied to `/tmp/run-staging-migrations.sh` and executed via SSM, `BASH_SOURCE[0]` resolves to `/tmp/run-staging-migrations.sh`, so `../../` traversal produces `/` — then `MIGRATIONS_DIR=/services/bff/...` doesn't exist and the script exits immediately.

**Fix applied:** Script now detects whether the relative root contains `services/bff/` and falls back to `/opt/mtga-companion` when it doesn't (the canonical EC2 repo location).

### Failure 3 — `run-staging-migrations.sh`: Reads production SSM paths for staging grants (P0)

Post-migration grants section read from:
- `/mtga-companion/production/db-secret-arn`
- `/mtga-companion/production/db-endpoint`

These are production parameters. The staging master credentials live at:
- `/mtga-companion/staging/db-secret-arn`
- `/mtga-companion/staging/db-endpoint`

**Fix applied:** Changed both SSM parameter names to `/staging/` paths.

### Failure 4 — `restart-bff-staging.sh`: No guard for missing systemd unit (P1)

The script ran `systemctl restart mtga-bff-staging` with no check whether `mtga-bff-staging.service` exists. If the unit file is absent the exit code is 5 ("unit not found") — a cryptic failure. The service unit may not have been installed after a fresh EC2 bootstrap.

**Fix applied:** Script now checks for `UNIT_FILE` existence and prints a clear error message with remediation steps. Also added `daemon-reload` + `enable` before restart for robustness.

---

## Files Changed

| File | Change |
|------|--------|
| `.github/workflows/staging-deploy.yml` | Added `infra/scripts/` S3 sync; fixed migration step command |
| `infra/scripts/run-staging-migrations.sh` | Fixed `REPO_ROOT` resolution; removed `--profile`; fixed SSM paths to staging |
| `scripts/deploy/restart-bff-staging.sh` | Added unit file existence guard; added `daemon-reload`+`enable` |

---

## Ray Action Required

**The following SSM parameters must exist in AWS Parameter Store (us-east-1) before the next staging deploy will fully succeed:**

| Parameter | Type | Purpose |
|-----------|------|---------|
| `/mtga-companion/staging/db-secret-arn` | String | ARN of the Secrets Manager secret holding staging RDS master credentials |
| `/mtga-companion/staging/db-endpoint` | String | Hostname of the staging RDS instance |

**How to check:**
```bash
aws ssm get-parameter --profile personal --region us-east-1 \
  --name /mtga-companion/staging/db-secret-arn --query Parameter.Value --output text

aws ssm get-parameter --profile personal --region us-east-1 \
  --name /mtga-companion/staging/db-endpoint --query Parameter.Value --output text
```

If either returns an error, create it:
```bash
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /mtga-companion/staging/db-secret-arn \
  --type String --value "<staging-secret-arn>"

aws ssm put-parameter --profile personal --region us-east-1 \
  --name /mtga-companion/staging/db-endpoint \
  --type String --value "<rds-staging-hostname>"
```

**Also verify the staging systemd unit is installed on EC2:**
```bash
aws ssm send-command --profile personal \
  --instance-ids <EC2_INSTANCE_ID> \
  --document-name AWS-RunShellScript \
  --parameters 'commands=["ls -la /etc/systemd/system/mtga-bff-staging.service"]' \
  --region us-east-1
```
If absent, run `infra/scripts/install-staging-service.sh` on the instance.

---

## Checkpoint Log

| Time (ET) | Checkpoint |
|-----------|-----------|
| 2026-05-10 08:00 | Root cause diagnosed for all 3 script bugs + workflow misconfiguration |
| 2026-05-10 08:00 | Fixes applied to staging-deploy.yml, run-staging-migrations.sh, restart-bff-staging.sh |
| 2026-05-10 08:00 | PR opened — pending Ray review |

---

## Previous Investigation (Run #25620349421)

Prior agent session (01:20 ET) diagnosed the Stage Binary SSM poll-loop timeout as a transient GHA API race, not an EC2 issue. That run ultimately succeeded through the staging step. Failures at restart and migrations steps are the new issues diagnosed and fixed here.

---

# Task: Install mtga-bff-staging.service on EC2

**Updated**: 2026-05-10T05:40 UTC
**Status**: Unit installed and enabled — blocked on bad SENTRY_DSN SSM parameter (Ray action required)

## What Was Done

1. `infra/systemd/mtga-bff-staging.service` already existed in repo with correct config:
   - Binary: `/usr/local/bin/mtga-bff-staging`
   - EnvironmentFile: `/etc/mtga-companion-staging/env`
   - `MTGA_ENV=staging`, `Restart=on-failure`, `RestartSec=5`, `User=ec2-user`
2. Unit file written to `/etc/systemd/system/mtga-bff-staging.service` on `i-065351fbb99da2d22` via SSM.
3. `systemctl daemon-reload && systemctl enable mtga-bff-staging` — succeeded.
4. `systemctl start mtga-bff-staging` — service crashes immediately on every start.

## Root Cause of Crash

```
sentry.Init: [Sentry] DsnParseError: invalid scheme
```

The SSM parameter `/vaultmtg/staging/sentry-bff-dsn` (type: `String`) contains a **raw KMS/Secrets Manager ciphertext blob** instead of a plaintext Sentry DSN URL. The `provision-staging-env.sh` script correctly fetches it without `--with-decryption` (it's a `String` type), but the stored value was never the actual DSN — it's encrypted ciphertext. The BFF binary treats it as the DSN and Sentry rejects the scheme.

**Parameter value (corrupted):**
```
AQICAHhL4WPgmGSIc3GopW9... (base64 KMS ciphertext)
```

**Expected value format:**
```
https://<key>@<org>.ingest.sentry.io/<project-id>
```

## Checkpoint Log

| Time (UTC) | Checkpoint |
|-----------|-----------|
| 2026-05-10 05:30 | Unit file confirmed correct in repo |
| 2026-05-10 05:31 | Unit written to EC2 via SSM, enabled successfully |
| 2026-05-10 05:32 | Start attempted — crash loop due to bad SENTRY_DSN |
| 2026-05-10 05:38 | Service stopped and disabled to halt crash loop |
| 2026-05-10 05:40 | PR opened for `infra/systemd/mtga-bff-staging.service` |

## Ray Action Required — BLOCKER

**Fix the corrupted `/vaultmtg/staging/sentry-bff-dsn` SSM parameter:**

1. Get the real Sentry DSN from the Sentry dashboard for the staging project.
2. Re-store it:
```bash
aws ssm put-parameter --profile personal --region us-east-1 \
  --name /vaultmtg/staging/sentry-bff-dsn \
  --type String \
  --value "https://<key>@<org>.ingest.sentry.io/<project-id>" \
  --overwrite
```
3. Re-run `provision-staging-env.sh` on the instance to regenerate `/etc/mtga-companion-staging/env`.
4. Re-enable and start the service:
```bash
aws ssm send-command --profile personal \
  --instance-ids i-065351fbb99da2d22 \
  --document-name AWS-RunShellScript \
  --parameters 'commands=["systemctl enable mtga-bff-staging","systemctl start mtga-bff-staging","systemctl status mtga-bff-staging --no-pager"]' \
  --region us-east-1
```
