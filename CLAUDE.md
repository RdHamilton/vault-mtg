- Always write Playwright e2e tests for new UI and UI changes
- Use the REST API adapter for new components (enables E2E testing)
- Run `npx tsc --noEmit` for TypeScript type checking (separate from vitest)
- Run `npm run test:run` to run vitest component tests

## Context Management - Use Subagents

To keep the main conversation focused and avoid context bloat:

- **Investigation/Exploration**: Use the `Explore` subagent to find code, trace implementations, or answer "where is X?" questions. Return only the summary to main context.
- **Planning**: Use the `Plan` subagent for designing implementations before writing code. Get user approval on the plan before implementing.
- **Parallel research**: When multiple things need investigation, spawn parallel agents to research simultaneously.
- **Self-contained tasks**: Use `general-purpose` subagent for tasks like "run tests and summarize failures" or "check all files importing X".

The main conversation should focus on:
- High-level goals and decisions
- Implementation based on agent summaries
- User communication and approval

Avoid in main context:
- Reading many files directly (use Explore agent)
- Long investigation chains
- Raw test output dumps

## Test Coverage Guidelines
- Always update UI/component tests when making UI changes
- Add integration tests for backend changes (repository, handlers, services)
- Add missing test coverage to files that are touched but lacking coverage
- Test types required for code changes:
  - Unit tests: For utility functions and business logic
  - Component tests: For React components (MatchHistory.test.tsx pattern)
  - Integration tests: For backend repository/handler changes (match_repo_test.go pattern)
  - E2E tests: For critical user flows when applicable
