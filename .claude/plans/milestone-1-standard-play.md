# Milestone 1: Enhanced Standard Play

## EPIC: #776 - Intelligent Deck Building & Analysis

### Vision
Transform MTGA-Companion into an intelligent Standard play assistant that helps users build competitive decks, track their evolution, analyze performance, and continuously improve their gameplay.

---

## Phase 0: Infrastructure (NEW)

### Issue #801 - Database migration system for v1.4.1 schema changes
**Status**: Todo
**Complexity**: Medium
**Dependencies**: None
**Priority**: HIGH - Blocks all Phase 1 work

**Scope**:
- Create migration framework with Up/Down support
- Add `schema_migrations` table for version tracking
- Create migration for `deck_permutations` table
- Run migrations on app startup
- Handle existing decks (create initial permutation)

**Files to create**:
- `internal/storage/migrations/migrations.go`
- `internal/storage/migrations/001_deck_permutations.go`
- `internal/storage/migrations/migrations_test.go`

---

### Issue #802 - Add test coverage for Standard Play features
**Status**: Todo (Ongoing)
**Complexity**: Ongoing
**Dependencies**: None
**Priority**: HIGH - Tests written alongside features

**Scope**:
- Unit tests for all new repositories
- Component tests for all new UI
- Integration tests for key flows
- Coverage targets: 80-95% depending on component

---

## Phase 1: Foundation

### Issue #773 - Standard Legality Validation and Set Management
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #801

**Scope**:
- Track which sets are Standard-legal
- Validate deck legality in real-time
- Handle set rotation (quarterly updates)
- API integration with Scryfall for set data

**Files to modify/create**:
- `internal/mtga/cards/legality.go`
- `internal/storage/repository/set_repo.go`
- `frontend/src/services/api/cards.ts`

---

### Issue #766 - Deck Permutation Tracking System
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #801

**Scope**:
- Track every deck modification as a new permutation
- Store parent-child relationship between versions
- Calculate diff between permutations
- Track win/loss per permutation

**Database Schema** (created by migration #801):
```sql
CREATE TABLE deck_permutations (
    id INTEGER PRIMARY KEY,
    deck_id TEXT NOT NULL,
    parent_permutation_id INTEGER,
    cards TEXT NOT NULL,
    created_at TIMESTAMP,
    wins INTEGER DEFAULT 0,
    losses INTEGER DEFAULT 0,
    FOREIGN KEY (deck_id) REFERENCES decks(id),
    FOREIGN KEY (parent_permutation_id) REFERENCES deck_permutations(id)
);
```

**Files to modify/create**:
- `internal/storage/repository/deck_permutation_repo.go`
- `internal/gui/deck_facade.go` (extend)
- `frontend/src/components/DeckHistory.tsx`

---

### Issue #769 - In-Game Play Tracking and Recording
**Status**: Todo
**Complexity**: High
**Dependencies**: #801

**Scope**:
- Track each game action (play, attack, block)
- Record mulligan decisions
- Track mana curve execution
- Store play sequences per match

**Implementation Notes**:
- Extend log parser to capture game actions
- Create new `game_plays` table
- Add WebSocket events for real-time tracking

---

## Phase 2: Basic Building

### Issue #767 - Card-Based Deck Builder with Collection Integration
**Status**: Todo
**Complexity**: High
**Dependencies**: #773

**Scope**:
- Search and filter cards
- Drag-and-drop deck building
- Show owned vs needed cards
- Real-time legality validation
- Mana curve visualization

---

### Issue #768 - Deck Theme and Archetype System
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #767

**Scope**:
- Define archetypes (Control, Aggro, Midrange, Combo)
- Define themes (Mill, Lifegain, Tokens, Reanimator)
- Auto-detect deck archetype
- Tag cards with archetype/theme relevance

---

### Issue #807 - Undo/redo functionality for deck builder (NEW)
**Status**: Todo
**Complexity**: Low
**Dependencies**: #767

**Scope**:
- Undo/redo stack for deck modifications
- Keyboard shortcuts (Ctrl+Z, Ctrl+Y)
- UI buttons with tooltips
- Clear stack on save

---

## Phase 3: Intelligence

### Issue #770 - ML-Powered Suggestion Engine
**Status**: Todo
**Complexity**: High
**Dependencies**: #766, #769

**Scope**:
- Train on card combination success rates
- Personalize based on user's play patterns
- Suggest card additions/removals
- Improve accuracy over time

---

### Issue #771 - Performance-Based Card Recommendations
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #770

**Scope**:
- Track card performance metrics
- Identify underperforming cards
- Suggest replacements based on data

---

### Issue #774 - Complete Deck Generation from Seed Card
**Status**: Todo
**Complexity**: High
**Dependencies**: #768, #770

**Scope**:
- Start with single card, generate full deck
- Consider archetype preferences
- Factor in collection ownership
- Generate strategy write-up

---

### Issue #804 - User settings for ML suggestion engine (NEW)
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #770

**Scope**:
- Enable/disable suggestions
- Suggestion frequency control
- Minimum confidence threshold
- Learning preferences
- Data retention settings
- "Clear Learned Data" button

---

## Phase 4: Analysis

### Issue #772 - Deck Notes and Play Improvement Suggestions
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #769, #770

**Scope**:
- Auto-generate deck notes
- Mulligan guidance
- Sequencing recommendations
- Matchup-specific advice

---

### Issue #775 - Opponent Deck Analysis and Meta Matching
**Status**: Todo
**Complexity**: High
**Dependencies**: #769

**Scope**:
- Reconstruct opponent deck from observed cards
- Match to known meta archetypes
- Track matchup performance
- Strategic insights per matchup

---

### Issue #803 - Set rotation notifications and deck legality alerts (NEW)
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #773

**Scope**:
- Rotation calendar display
- Deck legality warnings
- Proactive rotation notifications
- Card replacement suggestions

---

## Phase 5: Polish (NEW)

### Issue #805 - Loading states and progress indicators (NEW)
**Status**: Todo
**Complexity**: Medium
**Dependencies**: #770, #774

**Scope**:
- ProgressBar component
- ProgressModal component
- BackgroundTaskBar component
- WebSocket progress events
- Cancel functionality

---

### Issue #806 - Deck export to external platforms (NEW)
**Status**: Todo
**Complexity**: Low
**Dependencies**: #767

**Scope**:
- Export to Arena format
- Export to Moxfield URL
- Export to Archidekt URL
- JSON export with history
- Copy to clipboard

---

### Issue #808 - Documentation for Enhanced Standard Play (NEW)
**Status**: Todo
**Complexity**: Low
**Dependencies**: All above

**Scope**:
- In-app tooltips and help
- First-run tutorial
- User guides
- FAQ

---

## Progress Tracking

### Completed
- (none yet)

### In Progress
- (none yet)

### Blocked
- #773, #766, #769 blocked by #801 (migrations)
- #767 blocked by #773
- #768, #807 blocked by #767
- #770 blocked by #766, #769
- #771, #774, #804 blocked by #770
- #772 blocked by #769, #770
- #775 blocked by #769
- #803 blocked by #773
- #805 blocked by #770, #774
- #806 blocked by #767
- #808 blocked by all

---

## Implementation Order

```
1. #801 (Migrations)      ← START HERE
2. #802 (Tests)           ← Ongoing
3. #773 (Legality)        ← After #801
4. #766 (Permutations)    ← After #801
5. #769 (Play Tracking)   ← After #801
6. #767 (Deck Builder)    ← After #773
7. #768 (Themes)          ← After #767
8. #807 (Undo/Redo)       ← After #767
9. #770 (ML Engine)       ← After #766, #769
10. #771 (Recommendations) ← After #770
11. #774 (Deck Gen)       ← After #768, #770
12. #804 (ML Settings)    ← After #770
13. #772 (Deck Notes)     ← After #769, #770
14. #775 (Opponent)       ← After #769
15. #803 (Rotation)       ← After #773
16. #805 (Progress)       ← After #770, #774
17. #806 (Export)         ← After #767
18. #808 (Docs)           ← Last
```
