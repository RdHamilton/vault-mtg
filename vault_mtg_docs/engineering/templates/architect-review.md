# Architect Review Template

> **How to use:** Copy this file to `vault_mtg_docs/engineering/architecture/adr/YYYY-MM-ADR-NNN-title.md`.
> Fill in every section. Do not delete any section heading — leave it blank or write "N/A" with a
> one-line reason if it truly does not apply.
>
> **Status values:** `Proposed` | `Accepted` | `Superseded` | `Rejected`
>
> File path: `vault_mtg_docs/engineering/templates/architect-review.md`

---

# ADR-NNNN: \<Title\>

**Status**: Proposed
**Date**: YYYY-MM-DD
**Decider**: Ray Hamilton, Architect Agent
**Supersedes / Amends**: *(link to prior ADR, or "N/A")*
**Related**: *(links to related ADRs, issues, PRs)*

---

## Context

*Describe the problem, the forces at play, and why a decision is needed now. Include any relevant
incident history, existing constraints, or prior decisions that make this decision necessary.*

---

## Decision

*State the decision clearly in one or two sentences. Then elaborate: what exactly will be done,
what configuration will be applied, what the target state looks like.*

---

## Consequences

### What becomes easier

*List concrete improvements this decision enables.*

### What becomes harder / costs

*List concrete costs, new failure modes, or operational burden this decision introduces.*

### Risk assessment

*Describe the risk of the decision going wrong and the mitigations in place.*

---

## Alternatives Considered

*For each alternative: name it, describe it, explain why it was rejected.*

### A. \<Alternative name\>

*Description and rejection rationale.*

---

## Implementation Tickets

*Enumerate the tickets that will implement this decision. Include ticket number, title, effort
estimate, and owning agent. File these with Pam before this ADR is marked `Accepted`.*

| # | Title | Effort | Agent |
|---|---|---|---|
| TBD | | | |

---

## Live Destination Audit (REQUIRED — blocking gate)

**This section is mandatory for every architecture review that names a cloud identifier
as a rename target, a new resource name, or a migration destination.**

A repository grep cannot detect AWS resources created outside the repo's scope
(by other services, manual console operations, or other CloudFormation stacks).
Before this review may be `APPROVED`, you must enumerate the live account and
paste the output below.

**Canonical failure case**: Phase 5 `/vaultmtg/*` SSM namespace — assumed
clean via repo grep; was a live shared namespace containing 7 staging integration
secrets. The BFF EC2 role was granted a blanket `/vaultmtg/*` SSM read policy
during Phase 5 migration, producing 147 unauthorized reads across all 7 secrets.
Confirmed by CloudTrail 2026-05-18 and escalated to P1 incident #2310.

### Identifiers claimed in this review

*List every cloud identifier named in this ADR as a rename target, new resource,
or migration destination. For each one, run the corresponding CLI command below
and paste the output.*

| Identifier | Type | Status |
|---|---|---|
| `<identifier>` | `<SSM prefix \| S3 bucket \| IAM path \| RDS instance \| other>` | Paste enumeration output below |

### Enumeration commands (run each that applies; all must use `--profile personal`)

```bash
# SSM path prefix — confirms the namespace is unused before claiming it
aws ssm get-parameters-by-path --path "<prefix>/" --recursive --profile personal

# S3 bucket — confirms the bucket does not already exist
aws s3 ls s3://<bucket-name> --profile personal

# IAM roles at a path — confirms no role already claims the path/name
aws iam list-roles --query "Roles[?Path=='<path>']" --output table --profile personal

# RDS instance identifier — confirms the identifier is not already in use
aws rds describe-db-instances --db-instance-identifier <id> --profile personal

# CloudFormation stack name — confirms no stack already holds the name
aws cloudformation describe-stacks --stack-name <name> --profile personal

# Route 53 hosted zone — confirms the zone exists and is owned by this account
aws route53 list-hosted-zones --query "HostedZones[?Name=='<domain>.']" --profile personal
```

### Enumeration output

*Paste the raw CLI output here. Do not paraphrase. An architecture review that names a
destination identifier without a pasted live-account enumeration confirming it is unused
**cannot be `APPROVED`**.*

```
<paste CLI output here>
```

> **Reviewer gate**: If this section is absent, empty, or contains prose instead of a pasted
> CLI transcript, return the review as `CHANGES REQUIRED` — do not approve.

---

## Sign-off

- [ ] Architect (Ray): reviewed and approved
- [ ] Lead Engineer (Lee): aware of dependency graph and implementation sequencing
- [ ] PM (Najah / Pam): tickets filed before `Accepted` is set
