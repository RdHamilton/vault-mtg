package storage

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// QuestRepository handles database operations for quests
type QuestRepository struct {
	db *sql.DB
}

// NewQuestRepository creates a new quest repository
func NewQuestRepository(db *sql.DB) *QuestRepository {
	return &QuestRepository{db: db}
}

// Save saves a quest to the database (insert or update)
func (r *QuestRepository) Save(quest *models.Quest) error {
	// First, check if a quest with this quest_id already exists
	// We also check the completed/rerolled status to handle quest reassignment:
	// If MTGA reuses a quest_id for a new quest after the old one was completed or rerolled,
	// we should create a new record instead of updating the old one.
	existingQuery := `
		SELECT id, ending_progress, assigned_at, completed, rerolled FROM quests
		WHERE quest_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var existingID int
	var existingProgress int
	var existingAssignedAt time.Time
	var existingCompleted bool
	var existingRerolled bool
	err := r.db.QueryRow(existingQuery, quest.QuestID).Scan(&existingID, &existingProgress, &existingAssignedAt, &existingCompleted, &existingRerolled)

	if err == nil {
		// Quest exists - check if this is a quest reassignment
		// If the existing quest was completed or rerolled and the new quest is not completed,
		// this is a NEW quest with a reused ID - insert it as a new record
		if (existingCompleted || existingRerolled) && !quest.Completed {
			// Quest reassignment - insert as new record (fall through to insert logic)
			err = sql.ErrNoRows // Force insert logic below
		}
	}

	if err == nil {
		// Quest exists and is not a reassignment - update it
		// Use the completion status from the parser (which detects completion via quest disappearance)
		// IMPORTANT: Preserve the original assigned_at timestamp for accurate duration calculation

		// Format session_id and completion_source as nullable strings
		var sessionIDStr *string
		if quest.SessionID != "" {
			sessionIDStr = &quest.SessionID
		}
		var completionSourceStr *string
		if quest.CompletionSource != "" {
			completionSourceStr = &quest.CompletionSource
		}

		updateQuery := `
			UPDATE quests
			SET ending_progress = $1,
				completed = $2,
				completed_at = $3,
				last_seen_at = $4,
				can_swap = $5,
				rerolled = 0,
				session_id = COALESCE($6, session_id),
				completion_source = $7
			WHERE id = $8
		`

		_, err = r.db.Exec(updateQuery,
			quest.EndingProgress,
			quest.Completed,
			quest.CompletedAt,
			quest.LastSeenAt,
			quest.CanSwap,
			sessionIDStr,
			completionSourceStr,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update quest: %w", err)
		}

		quest.ID = existingID
		// Preserve the original assigned_at for accurate duration
		quest.AssignedAt = existingAssignedAt
		return nil
	}

	// Quest doesn't exist - insert it
	// Use the completion status from the parser (which detects completion via quest disappearance)

	query := `
		INSERT INTO quests (
			quest_id, quest_type, goal, starting_progress, ending_progress,
			completed, can_swap, rewards, assigned_at, completed_at, last_seen_at, rerolled,
			session_id, completion_source
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`

	// Format nullable string fields
	var sessionIDStr *string
	if quest.SessionID != "" {
		sessionIDStr = &quest.SessionID
	}
	var completionSourceStr *string
	if quest.CompletionSource != "" {
		completionSourceStr = &quest.CompletionSource
	}

	err = r.db.QueryRow(query,
		quest.QuestID, quest.QuestType, quest.Goal,
		quest.StartingProgress, quest.EndingProgress,
		quest.Completed, quest.CanSwap, quest.Rewards,
		quest.AssignedAt.UTC(), quest.CompletedAt, quest.LastSeenAt, quest.Rerolled,
		sessionIDStr, completionSourceStr,
	).Scan(&quest.ID)
	if err != nil {
		return fmt.Errorf("failed to save quest: %w", err)
	}

	return nil
}

// GetActiveQuests returns all incomplete, non-rerolled quests (one per unique quest_id).
// This is used by the API to show currently active quests.
//
// With recovery mode preventing false completions during startup, incomplete quests
// in the DB are genuinely incomplete and should always be shown without time filtering.
func (r *QuestRepository) GetActiveQuests() ([]*models.Quest, error) {
	query := `
		SELECT q.id, q.quest_id, q.quest_type, q.goal, q.starting_progress, q.ending_progress,
		       q.completed, q.can_swap, q.rewards, q.assigned_at, q.completed_at, q.last_seen_at,
		       q.rerolled, q.created_at, q.session_id, q.completion_source
		FROM quests q
		INNER JOIN (
			SELECT quest_id, MAX(created_at) as max_created
			FROM quests
			WHERE completed = 0
			  AND rerolled = 0
			GROUP BY quest_id
		) latest ON q.quest_id = latest.quest_id AND q.created_at = latest.max_created
		WHERE q.completed = 0
		  AND q.rerolled = 0
		ORDER BY q.assigned_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active quests: %w", err)
	}
	defer func() { _ = rows.Close() }() //nolint:errcheck // Ignore error on cleanup

	return r.scanQuests(rows)
}

// GetIncompleteQuests returns all incomplete, non-rerolled quests (one per unique quest_id)
// WITHOUT any timestamp filtering. This is used internally by the log processor to
// detect rerolled quests - any incomplete quest not in the current MTGA response is rerolled.
//
// This method should NOT be used for the API/UI - use GetActiveQuests() instead.
func (r *QuestRepository) GetIncompleteQuests() ([]*models.Quest, error) {
	query := `
		SELECT q.id, q.quest_id, q.quest_type, q.goal, q.starting_progress, q.ending_progress,
		       q.completed, q.can_swap, q.rewards, q.assigned_at, q.completed_at, q.last_seen_at,
		       q.rerolled, q.created_at, q.session_id, q.completion_source
		FROM quests q
		INNER JOIN (
			SELECT quest_id, MAX(created_at) as max_created
			FROM quests
			WHERE completed = 0
			  AND rerolled = 0
			GROUP BY quest_id
		) latest ON q.quest_id = latest.quest_id AND q.created_at = latest.max_created
		WHERE q.completed = 0
		  AND q.rerolled = 0
		ORDER BY q.assigned_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get incomplete quests: %w", err)
	}
	defer func() { _ = rows.Close() }() //nolint:errcheck // Ignore error on cleanup

	return r.scanQuests(rows)
}

// GetQuestHistory returns quest history with optional filters.
// Returns all quest records (including repeated quest_id assignments) filtered by assigned_at.
func (r *QuestRepository) GetQuestHistory(startDate, endDate *time.Time, limit int) ([]*models.Quest, error) {
	query := `
		SELECT id, quest_id, quest_type, goal, starting_progress, ending_progress,
		       completed, can_swap, rewards, assigned_at, completed_at, last_seen_at,
		       rerolled, created_at, session_id, completion_source
		FROM quests
		WHERE 1=1
	`
	args := []interface{}{}

	if startDate != nil {
		query += " AND DATE(assigned_at) >= ?"
		args = append(args, startDate.Format("2006-01-02"))
	}

	if endDate != nil {
		query += " AND DATE(assigned_at) <= ?"
		args = append(args, endDate.Format("2006-01-02"))
	}

	query += " ORDER BY assigned_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get quest history: %w", err)
	}
	defer func() { _ = rows.Close() }() //nolint:errcheck // Ignore error on cleanup

	return r.scanQuests(rows)
}

// GetQuestStats returns analytics about quest completion.
// Counts all quest instances individually (repeated quest_id assignments are separate).
func (r *QuestRepository) GetQuestStats(startDate, endDate *time.Time) (*models.QuestStats, error) {
	stats := &models.QuestStats{}

	query := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN completed = 1 THEN 1 ELSE 0 END), 0) as completed,
			COALESCE(SUM(CASE WHEN completed = 0 AND rerolled = 0 THEN 1 ELSE 0 END), 0) as active,
			COALESCE(SUM(CASE WHEN rerolled = 1 THEN 1 ELSE 0 END), 0) as rerolled
		FROM quests
		WHERE 1=1
	`
	args := []interface{}{}

	if startDate != nil {
		query += " AND DATE(assigned_at) >= ?"
		args = append(args, startDate.Format("2006-01-02"))
	}

	if endDate != nil {
		query += " AND DATE(assigned_at) <= ?"
		args = append(args, endDate.Format("2006-01-02"))
	}

	err := r.db.QueryRow(query, args...).Scan(
		&stats.TotalQuests, &stats.CompletedQuests, &stats.ActiveQuests, &stats.RerollCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get quest stats: %w", err)
	}

	// Calculate completion rate
	if stats.TotalQuests > 0 {
		stats.CompletionRate = float64(stats.CompletedQuests) / float64(stats.TotalQuests) * 100.0
	}

	// Average completion time (from all completed quest instances)
	query = `
		SELECT AVG(
			CAST((julianday(completed_at) - julianday(assigned_at)) * 86400000 AS INTEGER)
		)
		FROM quests
		WHERE completed = 1 AND completed_at IS NOT NULL
	`
	args = []interface{}{}

	if startDate != nil {
		query += " AND DATE(assigned_at) >= ?"
		args = append(args, startDate.Format("2006-01-02"))
	}

	if endDate != nil {
		query += " AND DATE(assigned_at) <= ?"
		args = append(args, endDate.Format("2006-01-02"))
	}

	var avgMS sql.NullFloat64
	err = r.db.QueryRow(query, args...).Scan(&avgMS)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to calculate average completion time: %w", err)
	}

	if avgMS.Valid {
		stats.AverageCompletionMS = int64(avgMS.Float64)
	}

	// Calculate total gold earned by parsing rewards from completed quests
	stats.TotalGoldEarned = r.calculateTotalGoldEarned(startDate, endDate)

	return stats, nil
}

// calculateTotalGoldEarned sums the gold rewards from all completed quests.
// Falls back to estimate of 500 gold per quest if parsing fails.
func (r *QuestRepository) calculateTotalGoldEarned(startDate, endDate *time.Time) int {
	query := `
		SELECT COALESCE(rewards, '') as rewards
		FROM quests
		WHERE completed = 1
	`
	args := []interface{}{}

	if startDate != nil {
		query += " AND DATE(assigned_at) >= $1"
		args = append(args, startDate.Format("2006-01-02"))
	}

	if endDate != nil {
		query += " AND DATE(assigned_at) <= $2"
		args = append(args, endDate.Format("2006-01-02"))
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		// Fall back to estimate on error
		return 0
	}
	defer func() { _ = rows.Close() }() //nolint:errcheck // Ignore error on cleanup

	totalGold := 0

	for rows.Next() {
		var rewards string
		if err := rows.Scan(&rewards); err != nil {
			continue
		}

		gold := parseGoldFromRewards(rewards)
		totalGold += gold
	}

	return totalGold
}

// parseGoldFromRewards extracts the gold amount from a rewards string.
// The rewards field can be:
// - A numeric string like "500" or "750"
// - Empty string (defaults to 500)
// - Invalid data (defaults to 500)
func parseGoldFromRewards(rewards string) int {
	rewards = strings.TrimSpace(rewards)

	if rewards == "" {
		return 500 // Default estimate for missing data
	}

	// Try to parse as integer
	if gold, err := strconv.Atoi(rewards); err == nil && gold > 0 {
		return gold
	}

	// Fall back to conservative estimate
	return 500
}

// GetQuestByID retrieves a quest by its database ID
func (r *QuestRepository) GetQuestByID(id int) (*models.Quest, error) {
	query := `
		SELECT id, quest_id, quest_type, goal, starting_progress, ending_progress,
		       completed, can_swap, rewards, assigned_at, completed_at, last_seen_at,
		       rerolled, created_at, session_id, completion_source
		FROM quests
		WHERE id = $1
	`

	quest := &models.Quest{}
	var sessionID sql.NullString
	var completionSource sql.NullString

	var completedAt sql.NullTime
	var lastSeenAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&quest.ID, &quest.QuestID, &quest.QuestType, &quest.Goal,
		&quest.StartingProgress, &quest.EndingProgress, &quest.Completed,
		&quest.CanSwap, &quest.Rewards, &quest.AssignedAt,
		&completedAt, &lastSeenAt, &quest.Rerolled, &quest.CreatedAt,
		&sessionID, &completionSource,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quest not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get quest: %w", err)
	}

	if completedAt.Valid {
		quest.CompletedAt = &completedAt.Time
	}
	if lastSeenAt.Valid {
		quest.LastSeenAt = &lastSeenAt.Time
	}

	if sessionID.Valid {
		quest.SessionID = sessionID.String
	}
	if completionSource.Valid {
		quest.CompletionSource = completionSource.String
	}

	return quest, nil
}

// MarkCompleted marks a quest as completed
func (r *QuestRepository) MarkCompleted(questID string, assignedAt time.Time, completedAt time.Time) error {
	query := `
		UPDATE quests
		SET completed = 1, completed_at = $1, ending_progress = goal
		WHERE quest_id = $2 AND assigned_at = $3
	`

	_, err := r.db.Exec(query, completedAt.UTC(), questID, assignedAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to mark quest as completed: %w", err)
	}

	return nil
}

// MarkRerolled marks a quest as rerolled
func (r *QuestRepository) MarkRerolled(questID string, assignedAt time.Time) error {
	query := `
		UPDATE quests
		SET rerolled = 1
		WHERE quest_id = $1 AND assigned_at = $2
	`

	_, err := r.db.Exec(query, questID, assignedAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to mark quest as rerolled: %w", err)
	}

	return nil
}

// MarkQuestsCompletedByGraphState marks quests as completed based on GraphGetGraphState data.
// It maps Quest1-7 from the graph state to actual quests by their assigned order.
func (r *QuestRepository) MarkQuestsCompletedByGraphState(completedQuestNumbers map[int]bool, timestamp time.Time) error {
	// Get active quests ordered by assigned_at (oldest first)
	// This maps to Quest1-7 in the graph state
	query := `
		SELECT q.id, q.quest_id, q.quest_type, q.goal, q.starting_progress, q.ending_progress,
		       q.completed, q.can_swap, q.rewards, q.assigned_at, q.completed_at, q.last_seen_at,
		       q.rerolled, q.created_at, q.session_id, q.completion_source
		FROM quests q
		INNER JOIN (
			SELECT quest_id, MAX(created_at) as max_created
			FROM quests
			WHERE completed = 0
			GROUP BY quest_id
		) latest ON q.quest_id = latest.quest_id AND q.created_at = latest.max_created
		WHERE q.completed = 0
		ORDER BY q.assigned_at ASC
		LIMIT 7
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to get active quests for completion: %w", err)
	}
	defer func() { _ = rows.Close() }() //nolint:errcheck // Ignore error on cleanup

	quests, err := r.scanQuests(rows)
	if err != nil {
		return fmt.Errorf("failed to scan quests for completion: %w", err)
	}

	// Mark quests as completed based on their position (Quest1 = oldest, Quest2 = 2nd oldest, etc.)
	for i, quest := range quests {
		questNumber := i + 1 // Quest1 = 1, Quest2 = 2, etc.

		// Check if this quest number is marked as completed in graph state
		if completedQuestNumbers[questNumber] {
			// Only mark as completed if it's not already completed
			if !quest.Completed {
				if err := r.MarkCompleted(quest.QuestID, quest.AssignedAt, timestamp); err != nil {
					return fmt.Errorf("failed to mark quest %d (%s) as completed: %w", questNumber, quest.QuestID, err)
				}
			}
		}
	}

	return nil
}

// MarkActiveQuestsCompleted marks all active quests that have reached their goal as completed.
// This is useful when we know quests should be completed but don't have specific quest IDs.
func (r *QuestRepository) MarkActiveQuestsCompleted(timestamp time.Time) error {
	query := `
		UPDATE quests
		SET completed = 1, completed_at = $1
		WHERE completed = 0
		  AND ending_progress >= goal
		  AND id IN (
			SELECT q.id
			FROM quests q
			INNER JOIN (
				SELECT quest_id, MAX(created_at) as max_created
				FROM quests
				WHERE completed = 0
				GROUP BY quest_id
			) latest ON q.quest_id = latest.quest_id AND q.created_at = latest.max_created
		  )
	`

	result, err := r.db.Exec(query, timestamp.UTC())
	if err != nil {
		return fmt.Errorf("failed to mark active quests as completed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// Log how many quests were marked completed
		_ = rowsAffected // Prevent unused variable warning
	}

	return nil
}

// parseTimestamp attempts to parse a timestamp string in multiple formats
func parseTimestamp(s string) (time.Time, error) {
	// Trim any leading/trailing whitespace
	s = strings.TrimSpace(s)

	// Try RFC3339 with microseconds (e.g., "2025-11-23T06:01:35.548252Z")
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}

	// Try RFC3339 without microseconds (e.g., "2025-11-23T06:01:35Z")
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try SQLite format with fractional seconds (variable length 1-9 digits)
	// Go's time.Parse requires exact digit count, so try common lengths
	sqliteFormats := []string{
		"2006-01-02 15:04:05.999999999", // nanoseconds (9 digits)
		"2006-01-02 15:04:05.999999",    // microseconds (6 digits)
		"2006-01-02 15:04:05.99999",     // 5 digits
		"2006-01-02 15:04:05.9999",      // 4 digits
		"2006-01-02 15:04:05.999",       // milliseconds (3 digits)
		"2006-01-02 15:04:05.99",        // 2 digits
		"2006-01-02 15:04:05.9",         // 1 digit
	}

	for _, format := range sqliteFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	// Try SQLite format without fractional seconds (e.g., "2006-01-02 15:04:05")
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}

// scanQuests is a helper to scan multiple quest rows
func (r *QuestRepository) scanQuests(rows *sql.Rows) ([]*models.Quest, error) {
	quests := []*models.Quest{}

	for rows.Next() {
		quest := &models.Quest{}
		var completedAt sql.NullTime
		var lastSeenAt sql.NullTime
		var sessionID sql.NullString
		var completionSource sql.NullString

		err := rows.Scan(
			&quest.ID, &quest.QuestID, &quest.QuestType, &quest.Goal,
			&quest.StartingProgress, &quest.EndingProgress, &quest.Completed,
			&quest.CanSwap, &quest.Rewards, &quest.AssignedAt,
			&completedAt, &lastSeenAt, &quest.Rerolled, &quest.CreatedAt,
			&sessionID, &completionSource,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan quest: %w", err)
		}

		if completedAt.Valid {
			quest.CompletedAt = &completedAt.Time
		}
		if lastSeenAt.Valid {
			quest.LastSeenAt = &lastSeenAt.Time
		}

		if sessionID.Valid {
			quest.SessionID = sessionID.String
		}
		if completionSource.Valid {
			quest.CompletionSource = completionSource.String
		}

		quests = append(quests, quest)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating quest rows: %w", err)
	}

	return quests, nil
}

// HasAnyQuestData returns true if any quest rows exist with a non-NULL last_seen_at.
// This distinguishes "no quest data yet" from "all quests completed."
func (r *QuestRepository) HasAnyQuestData() bool {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM quests WHERE last_seen_at IS NOT NULL").Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}
