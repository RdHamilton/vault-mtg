# Pattern: Batch Lambda Precomputation

**Author**: `architect`
**Status**: TODO — fill in after Wave 2 implementation
**Referenced by**: ADR-0015 (`docs/architecture/adr/0015-go-lambda-batch-precomputation.md`)

---

## Intent

Document the canonical pattern for adding a new batch precomputation job in VaultMTG. This is the third or fourth Lambda we'll add (after sync, ML feature builder, possibly leaderboards) and we want a single source of truth for: trigger, idempotency, observability, deploy, rollback.

## Status

This page is a placeholder. ADR-0015 captures the decision; the operational pattern (file layout, EventBridge config, IAM scoping, dashboard wiring) will be filled in once Wave 2 ships its first batch Lambda implementation. After that, this page becomes the recipe.

## Scope when filled in

1. File layout under `services/<job-name>/`
2. EventBridge schedule configuration
3. Idempotency strategy (deterministic keys, run-token table)
4. CloudWatch metric/alarm conventions
5. Deploy via GitHub Actions
6. Rollback recipe
7. Local testing harness

## Related

- ADR-0015 — go-lambda-batch-precomputation
- `services/sync/` — reference implementation (smaller scope, same pattern)
