# Specialized Agent Prompts

Use these prompts to spawn specialized agents for each milestone area.

---

## Agent: Standard Play Foundation

```
You are working on MTGA-Companion v1.4.1, specifically the Enhanced Standard Play feature (EPIC #776).

Your focus is Phase 1: Foundation, which includes:
- #773: Standard Legality Validation and Set Management
- #766: Deck Permutation Tracking System
- #769: In-Game Play Tracking and Recording

Context:
- This is a Go backend with React/TypeScript frontend
- Database is SQLite
- Read `.claude/plans/milestone-1-standard-play.md` for detailed requirements
- All changes need tests (Go tests for backend, Jest/Vitest for frontend)

Start by reading the milestone plan, then implement the specified issue.
```

---

## Agent: Deck Builder

```
You are working on MTGA-Companion v1.4.1, specifically the Enhanced Standard Play feature (EPIC #776).

Your focus is Phase 2: Basic Building, which includes:
- #767: Card-Based Deck Builder with Collection Integration
- #768: Deck Theme and Archetype System

Prerequisites: Phase 1 must be complete (#773, #766, #769)

Context:
- This is a Go backend with React/TypeScript frontend
- UI uses custom components (no component library)
- Read `.claude/plans/milestone-1-standard-play.md` for detailed requirements
- Focus on user experience and collection awareness

Start by reading the milestone plan and existing deck components, then implement.
```

---

## Agent: ML Intelligence

```
You are working on MTGA-Companion v1.4.1, specifically the Enhanced Standard Play feature (EPIC #776).

Your focus is Phase 3: Intelligence, which includes:
- #770: ML-Powered Suggestion Engine with Historical Learning
- #771: Performance-Based Card Recommendations
- #774: Complete Deck Generation from Seed Card

Prerequisites: Phase 1 and 2 must be complete

Context:
- Existing ML engine in `internal/ml/` can be extended
- Ollama LLM integration available for explanations
- Focus on learning from user's historical data
- Read `.claude/plans/milestone-1-standard-play.md` for detailed requirements

Start by reading the milestone plan and existing ML code, then implement.
```

---

## Agent: Analytics

```
You are working on MTGA-Companion v1.4.1, specifically the Enhanced Standard Play feature (EPIC #776).

Your focus is Phase 4: Analysis, which includes:
- #772: Deck Notes and Play Improvement Suggestions
- #775: Opponent Deck Analysis and Meta Matching

Prerequisites: Phase 1-3 should be substantially complete

Context:
- Build on play tracking from #769
- Integrate with ML engine from Phase 3
- Focus on actionable insights
- Read `.claude/plans/milestone-1-standard-play.md` for detailed requirements

Start by reading the milestone plan and existing analytics code, then implement.
```

---

## Agent: Test Writer

```
You are working on MTGA-Companion v1.4.1, specifically Technical Debt (Milestone 2).

Your focus is improving test coverage:
- #799: Add comprehensive tests for replay_engine.go
- #800: Add Stop method to API WebSocket Hub (and its test)

Context:
- Go tests use standard testing package
- WebSocket tests use httptest and gorilla/websocket
- Read `.claude/plans/milestone-2-technical-debt.md` for test cases
- Run `golangci-lint run --timeout=5m` after changes
- Run `gofumpt -w .` to format code

Start by reading the target files, then implement comprehensive tests.
```

---

## Agent: Draft Features

```
You are working on MTGA-Companion, specifically Advanced Draft Features (Milestone 3).

Issues in scope:
- #115: Analyze drafting patterns
- #117: Add visual draft pick timeline
- #120: Add archetype performance analysis
- #121: Add temporal trend analysis
- #122: Compare draft stats with community averages

Context:
- Draft data already tracked in `draft_picks` and `draft_sessions` tables
- 17Lands ratings available in `card_ratings` table
- Focus on analytics and visualization
- This is lower priority than Standard Play features

Start by understanding existing draft infrastructure, then implement.
```

---

## How to Use

1. Copy the appropriate prompt above
2. Use the Task tool with `subagent_type: "general-purpose"`
3. Include the prompt and specific issue number
4. Agent will read context files and implement

Example:
```
Task tool call:
- subagent_type: "general-purpose"
- prompt: "[Paste agent prompt] Now implement issue #773."
```
