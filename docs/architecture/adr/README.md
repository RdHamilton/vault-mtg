# Architecture Decision Records (ADRs)

This directory holds VaultMTG's ADRs. Numbering is zero-padded 4 digits. Status values: **Accepted**, **Superseded**, **Proposed**.

## Index

| ID | Title | Status |
|----|-------|--------|
| 0001 | Service split approaches (daemon / BFF / sync) | Accepted |
| 0002 | CI tiered test strategy | Accepted |
| 0003 | Sync service deployment strategy | Accepted |
| 0004 | SetCache ownership flip (sync owns canonical set data) | Accepted |
| 0005 | Sync delta strategy (incremental updates over full re-fetch) | Accepted |
| 0006 | Vercel BFF connectivity (preview-only) | Accepted |
| 0007 | Frontend serving model — initial proposal | Superseded by 0008 |
| 0008 | Frontend serving model — S3 + CloudFront | Accepted |
| 0009 | User auth provider — Clerk | Accepted |
| 0010 | Draft overlay architecture | Accepted |
| 0011 | Daemon distribution strategy (signed installers, auto-update) | Accepted |
| 0012 | Gameplay event correlation (match_id + game_number composite) | Accepted |
| 0013 | Daemon event ordering (sequence numbers, at-least-once) | Accepted |
| 0014 | Legacy parser extraction into pkg/logparse | Accepted |
| 0015 | Go Lambda batch precomputation pattern | Accepted |
| 0016 | External data — 17Lands bulk CSV ingestion | Accepted (sidelined v0.4.0) |
| 0017 | BFF precomputed read contract (typed envelope) | Accepted |
| 0018 | List endpoint pagination standard (cursor-based) | Accepted |
| 0019 | Staging environment design | Accepted |
| 0025 | AWS audit / observability floor for a solo-operated account | Accepted |
| 0040 | Collection-helper --dump-regions derivation procedure (PII handling, path validation, permissions) | Accepted |

## Conventions

- File naming: `NNNN-kebab-case-title.md`.
- Each ADR file starts with: `# ADR-NNNN: <title>`.
- Required front-matter fields (markdown headers, not YAML): `Status`, `Date`, `Decider`, `Context`, `Decision`, `Consequences`.
- Superseded ADRs link forward to the replacing ADR in their `Status` line.

## Adding a new ADR

1. Take the next 4-digit ID (next available is **0026**).
2. Copy an accepted ADR as a template (0018 is a clean recent one).
3. Fill in front-matter and submit a PR with `Status: Proposed`.
4. After review, flip `Status: Accepted` (or close the PR and write `Status: Rejected`).
5. Update this README index.
