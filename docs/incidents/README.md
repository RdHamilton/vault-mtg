# Incidents

Incident post-mortems and runbooks owned by `infrastructure`.

## Scope

- Production incident post-mortems (timeline, root cause, action items)
- Runbooks for known failure modes (BFF down, daemon registrar broken, projection lag spike)
- On-call rotation references (post-GA)

## Conventions

- File naming: `YYYY-MM-DD-short-title.md`.
- Required sections: Summary, Timeline (UTC), Root cause, Resolution, Action items.
- Severity tags: SEV1 (user-facing data loss), SEV2 (degraded UX, not blocking), SEV3 (internal-only).

## Owners

- `infrastructure` (primary)
- `lead-engineer` (review)
- `architect` (consulted on architectural action items)

## Notes

This directory is currently empty. The first formal incident post-mortem will land here when v0.4.0 closed beta opens.
