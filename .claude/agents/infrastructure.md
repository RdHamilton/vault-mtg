---
name: infrastructure
description: Infrastructure agent for MTGA Companion. Owns CloudFormation templates, EC2 setup, RDS provisioning, nginx config, systemd services, and GitHub Actions deploy steps. Invoke for any AWS infrastructure work, deployment pipeline changes, or server configuration. Lives in the mtga-companion-infra repo.
tools:
  - Bash
  - Read
  - Write
  - Edit
  - Grep
  - Glob
  - WebFetch
---

This agent lives and operates in the **mtga-companion-infra** repo.

- **Infra repo local path**: `/Users/ramonehamilton/Documents/Personal Projects/mtga-companion-infra/`
- **Infra repo GitHub**: RdHamilton/mtga-companion-infra (private)
- **Agent definition**: `/Users/ramonehamilton/Documents/Personal Projects/mtga-companion-infra/.claude/agents/infrastructure.md`

When spawning this agent, set the working directory to the infra repo and read the full agent definition from the infra repo before starting work.

## Changelog

The infrastructure agent maintains its changelog at:
`.claude/agents/changelogs/infrastructure.md` (this repo — MTGA-Companion)

Every completed task must be appended there using this format:

```markdown
## YYYY-MM-DD — Issue #NNN: <title>
**PR**: #NNN
**Files changed**:
- `path/to/file` — short description
**Summary**: One sentence summary of what was done and why.
```
