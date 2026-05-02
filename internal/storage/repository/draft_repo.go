package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// DraftRepository provides methods for managing draft sessions, picks, and packs.
type DraftRepository interface {
	// Sessions
	CreateSession(ctx context.Context, session *models.DraftSession) error
	GetSession(ctx context.Context, id string) (*models.DraftSession, error)
	GetActiveSessions(ctx context.Context) ([]*models.DraftSession, error)
	GetActiveSessionByIDPrefix(ctx context.Context, prefix string) (*models.DraftSession, error)
	GetCompletedSessions(ctx context.Context, limit int) ([]*models.DraftSession, error)
	GetSessionsByAccount(ctx context.Context, accountID int, limit int) ([]*models.DraftSession, error)
	UpdateSessionStatus(ctx context.Context, id string, status string, endTime *time.Time) error
	UpdateSessionTotalPicks(ctx context.Context, id string, totalPicks int) error
	IncrementSessionPicks(ctx context.Context, id string) error

	// Picks
	SavePick(ctx context.Context, pick *models.DraftPickSession) error
	GetPicksBySession(ctx context.Context, sessionID string) ([]*models.DraftPickSession, error)
	GetPickByNumber(ctx context.Context, sessionID string, packNum, pickNum int) (*models.DraftPickSession, error)
	UpdatePickQuality(ctx context.Context, pickID int, grade string, rank int, packBestGIHWR, pickedCardGIHWR float64, alternativesJSON string) error

	// Packs
	SavePack(ctx context.Context, pack *models.DraftPackSession) error
	GetPacksBySession(ctx context.Context, sessionID string) ([]*models.DraftPackSession, error)
	GetPack(ctx context.Context, sessionID string, packNum, pickNum int) (*models.DraftPackSession, error)

	// Grades
	UpdateSessionGrade(ctx context.Context, sessionID string, overallGrade string, overallScore int, pickQuality, colorDiscipline, deckComposition, strategic float64) error

	// Predictions
	UpdateSessionPrediction(ctx context.Context, sessionID string, winRate, winRateMin, winRateMax float64, factorsJSON string, predictedAt time.Time) error

	// Cleanup
	ClearAllSessions(ctx context.Context) (sessionsDeleted, picksDeleted, packsDeleted int64, err error)
	GetSessionCount(ctx context.Context) (int, error)
	GetPickCount(ctx context.Context) (int, error)
	GetPackCount(ctx context.Context) (int, error)

	// Aggregation
	GetAllPickCardCounts(ctx context.Context) (map[int]int, error)
}

type draftRepository struct {
	db *sql.DB
}

// NewDraftRepository creates a new draft repository.
func NewDraftRepository(db *sql.DB) DraftRepository {
	return &draftRepository{db: db}
}

// CreateSession creates a new draft session.
// Uses INSERT ... ON CONFLICT DO UPDATE to handle replays where the same draft session may be processed multiple times.
func (r *draftRepository) CreateSession(ctx context.Context, session *models.DraftSession) error {
	query := `
		INSERT INTO draft_sessions (id, account_id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT(id) DO UPDATE SET
			account_id = excluded.account_id,
			event_name = excluded.event_name,
			set_code = excluded.set_code,
			draft_type = excluded.draft_type,
			start_time = excluded.start_time,
			status = excluded.status,
			total_picks = excluded.total_picks,
			updated_at = excluded.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		session.ID,
		session.AccountID,
		session.EventName,
		session.SetCode,
		session.DraftType,
		session.StartTime,
		session.Status,
		session.TotalPicks,
		session.CreatedAt,
		session.UpdatedAt,
	)
	return err
}

// GetSession retrieves a draft session by ID.
func (r *draftRepository) GetSession(ctx context.Context, id string) (*models.DraftSession, error) {
	query := `
		SELECT id, account_id, event_name, set_code, draft_type, start_time, end_time, status, total_picks,
			overall_grade, overall_score, pick_quality_score, color_discipline_score,
			deck_composition_score, strategic_score,
			predicted_win_rate, predicted_win_rate_min, predicted_win_rate_max,
			prediction_factors, predicted_at,
			created_at, updated_at
		FROM draft_sessions
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	session := &models.DraftSession{}
	var accountID sql.NullInt64
	var endTime sql.NullTime
	var overallGrade sql.NullString
	var overallScore sql.NullInt64
	var pickQuality sql.NullFloat64
	var colorDiscipline sql.NullFloat64
	var deckComposition sql.NullFloat64
	var strategic sql.NullFloat64
	var predictedWinRate sql.NullFloat64
	var predictedWinRateMin sql.NullFloat64
	var predictedWinRateMax sql.NullFloat64
	var predictionFactors sql.NullString
	var predictedAt sql.NullTime

	err := row.Scan(
		&session.ID,
		&accountID,
		&session.EventName,
		&session.SetCode,
		&session.DraftType,
		&session.StartTime,
		&endTime,
		&session.Status,
		&session.TotalPicks,
		&overallGrade,
		&overallScore,
		&pickQuality,
		&colorDiscipline,
		&deckComposition,
		&strategic,
		&predictedWinRate,
		&predictedWinRateMin,
		&predictedWinRateMax,
		&predictionFactors,
		&predictedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if accountID.Valid {
		session.AccountID = int(accountID.Int64)
	}
	if endTime.Valid {
		session.EndTime = &endTime.Time
	}
	if overallGrade.Valid {
		session.OverallGrade = &overallGrade.String
	}
	if overallScore.Valid {
		score := int(overallScore.Int64)
		session.OverallScore = &score
	}
	if pickQuality.Valid {
		session.PickQualityScore = &pickQuality.Float64
	}
	if colorDiscipline.Valid {
		session.ColorDisciplineScore = &colorDiscipline.Float64
	}
	if deckComposition.Valid {
		session.DeckCompositionScore = &deckComposition.Float64
	}
	if strategic.Valid {
		session.StrategicScore = &strategic.Float64
	}
	if predictedWinRate.Valid {
		session.PredictedWinRate = &predictedWinRate.Float64
	}
	if predictedWinRateMin.Valid {
		session.PredictedWinRateMin = &predictedWinRateMin.Float64
	}
	if predictedWinRateMax.Valid {
		session.PredictedWinRateMax = &predictedWinRateMax.Float64
	}
	if predictionFactors.Valid {
		session.PredictionFactors = &predictionFactors.String
	}
	if predictedAt.Valid {
		session.PredictedAt = &predictedAt.Time
	}

	return session, nil
}

// GetActiveSessions retrieves all active draft sessions.
func (r *draftRepository) GetActiveSessions(ctx context.Context) ([]*models.DraftSession, error) {
	query := `
		SELECT id, account_id, event_name, set_code, draft_type, start_time, end_time, status, total_picks, created_at, updated_at
		FROM draft_sessions
		WHERE status = 'in_progress'
		ORDER BY start_time DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	sessions := []*models.DraftSession{}
	for rows.Next() {
		session := &models.DraftSession{}
		var accountID sql.NullInt64
		var endTime sql.NullTime

		err := rows.Scan(
			&session.ID,
			&accountID,
			&session.EventName,
			&session.SetCode,
			&session.DraftType,
			&session.StartTime,
			&endTime,
			&session.Status,
			&session.TotalPicks,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if accountID.Valid {
			session.AccountID = int(accountID.Int64)
		}
		if endTime.Valid {
			session.EndTime = &endTime.Time
		}

		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// GetActiveSessionByIDPrefix finds an active (in_progress) session whose ID starts with the given prefix.
// This is used to find existing sessions created with timestamp suffixes (e.g., "QuickDraft_TLA_20251127_*").
// Returns the most recently created session if multiple exist.
func (r *draftRepository) GetActiveSessionByIDPrefix(ctx context.Context, prefix string) (*models.DraftSession, error) {
	query := `
		SELECT id, account_id, event_name, set_code, draft_type, start_time, end_time, status, total_picks, created_at, updated_at
		FROM draft_sessions
		WHERE status = 'in_progress' AND id LIKE $1 || '%'
		ORDER BY created_at DESC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, prefix)

	session := &models.DraftSession{}
	var accountID sql.NullInt64
	var endTime sql.NullTime

	err := row.Scan(
		&session.ID,
		&accountID,
		&session.EventName,
		&session.SetCode,
		&session.DraftType,
		&session.StartTime,
		&endTime,
		&session.Status,
		&session.TotalPicks,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if accountID.Valid {
		session.AccountID = int(accountID.Int64)
	}
	if endTime.Valid {
		session.EndTime = &endTime.Time
	}

	return session, nil
}

// GetCompletedSessions retrieves completed draft sessions ordered by completion date.
func (r *draftRepository) GetCompletedSessions(ctx context.Context, limit int) ([]*models.DraftSession, error) {
	query := `
		SELECT id, account_id, event_name, set_code, draft_type, start_time, end_time, status, total_picks, created_at, updated_at
		FROM draft_sessions
		WHERE status = 'completed'
		ORDER BY start_time DESC
		LIMIT $1
	`
	return r.scanSessions(r.db.QueryContext(ctx, query, limit))
}

// GetSessionsByAccount retrieves draft sessions for a specific account, ordered by start time.
func (r *draftRepository) GetSessionsByAccount(ctx context.Context, accountID int, limit int) ([]*models.DraftSession, error) {
	query := `
		SELECT id, account_id, event_name, set_code, draft_type, start_time, end_time, status, total_picks, created_at, updated_at
		FROM draft_sessions
		WHERE account_id = $1
		ORDER BY start_time DESC
		LIMIT $2
	`
	return r.scanSessions(r.db.QueryContext(ctx, query, accountID, limit))
}

func (r *draftRepository) scanSessions(rows *sql.Rows, err error) ([]*models.DraftSession, error) {
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	sessions := []*models.DraftSession{}
	for rows.Next() {
		session := &models.DraftSession{}
		var accountID sql.NullInt64
		var endTime sql.NullTime

		if err := rows.Scan(
			&session.ID,
			&accountID,
			&session.EventName,
			&session.SetCode,
			&session.DraftType,
			&session.StartTime,
			&endTime,
			&session.Status,
			&session.TotalPicks,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if accountID.Valid {
			session.AccountID = int(accountID.Int64)
		}
		if endTime.Valid {
			session.EndTime = &endTime.Time
		}

		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// UpdateSessionStatus updates the status and optionally the end time of a draft session.
func (r *draftRepository) UpdateSessionStatus(ctx context.Context, id string, status string, endTime *time.Time) error {
	query := `
		UPDATE draft_sessions
		SET status = $1, end_time = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, status, endTime, time.Now(), id)
	return err
}

// UpdateSessionTotalPicks updates the total_picks value for a session.
func (r *draftRepository) UpdateSessionTotalPicks(ctx context.Context, id string, totalPicks int) error {
	query := `
		UPDATE draft_sessions
		SET total_picks = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, totalPicks, time.Now(), id)
	return err
}

// IncrementSessionPicks increments the total_picks counter for a session.
func (r *draftRepository) IncrementSessionPicks(ctx context.Context, id string) error {
	query := `
		UPDATE draft_sessions
		SET total_picks = total_picks + 1, updated_at = $1
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

// SavePick saves a draft pick.
// Uses ON CONFLICT DO UPDATE to handle replays where the same pick may be processed multiple times.
func (r *draftRepository) SavePick(ctx context.Context, pick *models.DraftPickSession) error {
	query := `
		INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(session_id, pack_number, pick_number) DO UPDATE SET
			card_id = excluded.card_id,
			timestamp = excluded.timestamp
		RETURNING id
	`
	err := r.db.QueryRowContext(ctx, query,
		pick.SessionID,
		pick.PackNumber,
		pick.PickNumber,
		pick.CardID,
		pick.Timestamp,
	).Scan(&pick.ID)
	if err != nil {
		return err
	}

	return nil
}

// GetPicksBySession retrieves all picks for a draft session.
func (r *draftRepository) GetPicksBySession(ctx context.Context, sessionID string) ([]*models.DraftPickSession, error) {
	query := `
		SELECT id, session_id, pack_number, pick_number, card_id, timestamp,
			pick_quality_grade, pick_quality_rank, pack_best_gihwr, picked_card_gihwr, alternatives_json
		FROM draft_picks
		WHERE session_id = $1
		ORDER BY pack_number, pick_number
	`
	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	picks := []*models.DraftPickSession{}
	for rows.Next() {
		pick := &models.DraftPickSession{}
		var grade sql.NullString
		var rank sql.NullInt64
		var packBestGIHWR sql.NullFloat64
		var pickedCardGIHWR sql.NullFloat64
		var alternativesJSON sql.NullString

		err := rows.Scan(
			&pick.ID,
			&pick.SessionID,
			&pick.PackNumber,
			&pick.PickNumber,
			&pick.CardID,
			&pick.Timestamp,
			&grade,
			&rank,
			&packBestGIHWR,
			&pickedCardGIHWR,
			&alternativesJSON,
		)
		if err != nil {
			return nil, err
		}

		if grade.Valid {
			pick.PickQualityGrade = &grade.String
		}
		if rank.Valid {
			rankInt := int(rank.Int64)
			pick.PickQualityRank = &rankInt
		}
		if packBestGIHWR.Valid {
			pick.PackBestGIHWR = &packBestGIHWR.Float64
		}
		if pickedCardGIHWR.Valid {
			pick.PickedCardGIHWR = &pickedCardGIHWR.Float64
		}
		if alternativesJSON.Valid {
			pick.AlternativesJSON = &alternativesJSON.String
		}

		picks = append(picks, pick)
	}

	return picks, rows.Err()
}

// GetPickByNumber retrieves a specific pick by pack and pick number.
func (r *draftRepository) GetPickByNumber(ctx context.Context, sessionID string, packNum, pickNum int) (*models.DraftPickSession, error) {
	query := `
		SELECT id, session_id, pack_number, pick_number, card_id, timestamp
		FROM draft_picks
		WHERE session_id = $1 AND pack_number = $2 AND pick_number = $3
	`
	row := r.db.QueryRowContext(ctx, query, sessionID, packNum, pickNum)

	pick := &models.DraftPickSession{}
	err := row.Scan(
		&pick.ID,
		&pick.SessionID,
		&pick.PackNumber,
		&pick.PickNumber,
		&pick.CardID,
		&pick.Timestamp,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return pick, nil
}

// SavePack saves a draft pack.
// Uses INSERT OR REPLACE to handle replays where the same pack may be processed multiple times.
func (r *draftRepository) SavePack(ctx context.Context, pack *models.DraftPackSession) error {
	log.Printf("[SavePack] Saving pack: session=%s, pack=%d, pick=%d, cards=%d",
		pack.SessionID, pack.PackNumber, pack.PickNumber, len(pack.CardIDs))

	// Convert []string to JSON for storage
	cardIDsJSON, err := json.Marshal(pack.CardIDs)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO draft_packs (session_id, pack_number, pick_number, card_ids, timestamp)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(session_id, pack_number, pick_number) DO UPDATE SET
			card_ids = excluded.card_ids,
			timestamp = excluded.timestamp
		RETURNING id
	`
	err = r.db.QueryRowContext(ctx, query,
		pack.SessionID,
		pack.PackNumber,
		pack.PickNumber,
		string(cardIDsJSON),
		pack.Timestamp,
	).Scan(&pack.ID)
	if err != nil {
		log.Printf("[SavePack] ERROR saving pack: %v", err)
		return err
	}

	log.Printf("[SavePack] Successfully saved pack with ID=%d", pack.ID)
	return nil
}

// GetPacksBySession retrieves all packs for a draft session.
func (r *draftRepository) GetPacksBySession(ctx context.Context, sessionID string) ([]*models.DraftPackSession, error) {
	query := `
		SELECT id, session_id, pack_number, pick_number, card_ids, timestamp
		FROM draft_packs
		WHERE session_id = $1
		ORDER BY pack_number, pick_number
	`
	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	packs := []*models.DraftPackSession{}
	for rows.Next() {
		pack := &models.DraftPackSession{}
		var cardIDsJSON string

		err := rows.Scan(
			&pack.ID,
			&pack.SessionID,
			&pack.PackNumber,
			&pack.PickNumber,
			&cardIDsJSON,
			&pack.Timestamp,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON back to []string
		if err := json.Unmarshal([]byte(cardIDsJSON), &pack.CardIDs); err != nil {
			return nil, err
		}

		packs = append(packs, pack)
	}

	return packs, rows.Err()
}

// GetPack retrieves a specific pack by pack and pick number.
func (r *draftRepository) GetPack(ctx context.Context, sessionID string, packNum, pickNum int) (*models.DraftPackSession, error) {
	log.Printf("[GetPack] Looking for pack: session=%s, pack=%d, pick=%d", sessionID, packNum, pickNum)

	query := `
		SELECT id, session_id, pack_number, pick_number, card_ids, timestamp
		FROM draft_packs
		WHERE session_id = $1 AND pack_number = $2 AND pick_number = $3
	`
	row := r.db.QueryRowContext(ctx, query, sessionID, packNum, pickNum)

	pack := &models.DraftPackSession{}
	var cardIDsJSON string

	err := row.Scan(
		&pack.ID,
		&pack.SessionID,
		&pack.PackNumber,
		&pack.PickNumber,
		&cardIDsJSON,
		&pack.Timestamp,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[GetPack] No pack found for session=%s, pack=%d, pick=%d", sessionID, packNum, pickNum)
			return nil, nil
		}
		log.Printf("[GetPack] ERROR querying pack: %v", err)
		return nil, err
	}

	// Parse JSON back to []string
	if err := json.Unmarshal([]byte(cardIDsJSON), &pack.CardIDs); err != nil {
		log.Printf("[GetPack] ERROR unmarshaling card IDs: %v", err)
		return nil, err
	}

	log.Printf("[GetPack] Found pack with %d cards", len(pack.CardIDs))
	return pack, nil
}

// UpdatePickQuality updates the pick quality analysis fields for a pick.
func (r *draftRepository) UpdatePickQuality(ctx context.Context, pickID int, grade string, rank int, packBestGIHWR, pickedCardGIHWR float64, alternativesJSON string) error {
	query := `
		UPDATE draft_picks
		SET pick_quality_grade = $1,
			pick_quality_rank = $2,
			pack_best_gihwr = $3,
			picked_card_gihwr = $4,
			alternatives_json = $5
		WHERE id = $6
	`
	_, err := r.db.ExecContext(ctx, query, grade, rank, packBestGIHWR, pickedCardGIHWR, alternativesJSON, pickID)
	return err
}

// UpdateSessionGrade updates the grade fields for a draft session.
func (r *draftRepository) UpdateSessionGrade(ctx context.Context, sessionID string, overallGrade string, overallScore int, pickQuality, colorDiscipline, deckComposition, strategic float64) error {
	query := `
		UPDATE draft_sessions
		SET overall_grade = $1,
			overall_score = $2,
			pick_quality_score = $3,
			color_discipline_score = $4,
			deck_composition_score = $5,
			strategic_score = $6,
			updated_at = $7
		WHERE id = $8
	`
	_, err := r.db.ExecContext(ctx, query, overallGrade, overallScore, pickQuality, colorDiscipline, deckComposition, strategic, time.Now(), sessionID)
	return err
}

// UpdateSessionPrediction updates the win rate prediction fields for a draft session.
func (r *draftRepository) UpdateSessionPrediction(ctx context.Context, sessionID string, winRate, winRateMin, winRateMax float64, factorsJSON string, predictedAt time.Time) error {
	query := `
		UPDATE draft_sessions
		SET predicted_win_rate = $1,
		    predicted_win_rate_min = $2,
		    predicted_win_rate_max = $3,
		    prediction_factors = $4,
		    predicted_at = $5,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $6
	`
	_, err := r.db.ExecContext(ctx, query, winRate, winRateMin, winRateMax, factorsJSON, predictedAt, sessionID)
	return err
}

// ClearAllSessions deletes all draft sessions, picks, and packs from the database.
// Returns the count of each type deleted.
func (r *draftRepository) ClearAllSessions(ctx context.Context) (sessionsDeleted, picksDeleted, packsDeleted int64, err error) {
	// Delete picks first (foreign key relationship)
	picksResult, err := r.db.ExecContext(ctx, `DELETE FROM draft_picks`)
	if err != nil {
		return 0, 0, 0, err
	}
	picksDeleted, _ = picksResult.RowsAffected()

	// Delete packs (foreign key relationship)
	packsResult, err := r.db.ExecContext(ctx, `DELETE FROM draft_packs`)
	if err != nil {
		return 0, picksDeleted, 0, err
	}
	packsDeleted, _ = packsResult.RowsAffected()

	// Delete sessions
	sessionsResult, err := r.db.ExecContext(ctx, `DELETE FROM draft_sessions`)
	if err != nil {
		return 0, picksDeleted, packsDeleted, err
	}
	sessionsDeleted, _ = sessionsResult.RowsAffected()

	return sessionsDeleted, picksDeleted, packsDeleted, nil
}

// GetSessionCount returns the total number of draft sessions.
func (r *draftRepository) GetSessionCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM draft_sessions`).Scan(&count)
	return count, err
}

// GetPickCount returns the total number of draft picks.
func (r *draftRepository) GetPickCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM draft_picks`).Scan(&count)
	return count, err
}

// GetPackCount returns the total number of draft packs.
func (r *draftRepository) GetPackCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM draft_packs`).Scan(&count)
	return count, err
}

// GetAllPickCardCounts returns aggregated card counts across all draft picks.
// Returns a map of card ID (as int) to pick count.
func (r *draftRepository) GetAllPickCardCounts(ctx context.Context) (map[int]int, error) {
	query := `
		SELECT dp.card_id, COUNT(*) as pick_count
		FROM draft_picks dp
		JOIN draft_sessions ds ON dp.session_id = ds.id
		GROUP BY dp.card_id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query draft picks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cardCounts := make(map[int]int)
	for rows.Next() {
		var cardIDStr string
		var pickCount int
		if err := rows.Scan(&cardIDStr, &pickCount); err != nil {
			return nil, fmt.Errorf("failed to scan draft pick: %w", err)
		}

		// Convert card ID string to int
		var cardID int
		if _, err := fmt.Sscanf(cardIDStr, "%d", &cardID); err != nil {
			// Skip non-numeric card IDs
			continue
		}

		cardCounts[cardID] = pickCount
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating draft picks: %w", err)
	}

	return cardCounts, nil
}
