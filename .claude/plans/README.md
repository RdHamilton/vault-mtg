# MTGA-Companion Implementation Plans

This directory contains implementation plans for MTGA-Companion development, designed to persist context across Claude Code sessions.

## Files

| File | Purpose |
|------|---------|
| `v1.4.1-implementation-plan.md` | Master plan with all milestones and progress |
| `milestone-1-standard-play.md` | Detailed plan for Enhanced Standard Play (EPIC #776) |
| `milestone-2-technical-debt.md` | Technical debt and quality improvements |
| `agent-prompts.md` | Pre-written prompts for specialized agents |

## How to Resume Work

### Starting a New Session

1. **Read the master plan**:
   ```
   Read .claude/plans/v1.4.1-implementation-plan.md
   ```

2. **Check current project status**:
   ```bash
   gh project item-list 26 --owner RdHamilton --format json | jq '[.items[] | {title, status: .status, number: .content.number}]'
   ```

3. **Update session tracking** in the master plan

4. **Pick next task** based on dependencies and priorities

### Using Specialized Agents

1. Read `agent-prompts.md` for pre-configured prompts
2. Spawn agent with Task tool using appropriate prompt
3. Agent reads context files and implements

### After Completing Work

1. Update milestone file with progress
2. Update master plan session tracking
3. Mark issues as Done in GitHub project:
   ```bash
   gh project item-edit --project 26 --id <ITEM_ID> --field-id <STATUS_FIELD_ID> --value "Done"
   ```

## Priority Order

1. **Milestone 1**: Enhanced Standard Play (v1.4.1 primary goal)
2. **Milestone 2**: Technical Debt (maintains code quality)
3. **Milestone 3**: Advanced Draft Features (v2.0 scope)
4. **Milestone 4**: GUI & Integration (low priority)
5. **Milestone 5**: Future Features (backlog)

## Quick Commands

```bash
# View project status
gh project view 26 --owner RdHamilton

# List all project items
gh project item-list 26 --owner RdHamilton

# View specific issue
gh issue view <number>

# Run tests
cd frontend && npm run test:run
go test ./...

# Run linters
golangci-lint run --timeout=5m
gofumpt -w .
cd frontend && npm run lint
```
